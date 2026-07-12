package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"open-ai-gateway/internal/audit"
	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/middleware"
	"open-ai-gateway/internal/router"
	"open-ai-gateway/internal/routes"
)

func (s *Server) handleResponses(w http.ResponseWriter, r *http.Request) {
	if err := requireJSONContentType(r); err != nil {
		s.writeError(w, r, err)
		return
	}
	var req compat.ResponseRequest
	if err := decodeJSONBody(s.requestBody(w, r), &req); err != nil {
		s.writeError(w, r, decodeError(err))
		return
	}
	middleware.SetLogStream(r.Context(), req.Stream)
	chatReq, validationErr := req.ChatRequest()
	if validationErr != nil {
		s.writeAuditedError(w, r, routes.ResponsesPath, req.Model, validationErr)
		return
	}
	if !s.modelAllowedForRequest(r, req.Model) {
		middleware.SetLogRoute(r.Context(), req.Model, "", "")
		s.writeAuditedError(w, r, routes.ResponsesPath, req.Model, compat.ModelNotFound(req.Model))
		return
	}
	route, resolveErr := s.router.ResolveFor(req.Model, "chat")
	if resolveErr != nil {
		middleware.SetLogRoute(r.Context(), req.Model, "", "")
		s.writeAuditedError(w, r, routes.ResponsesPath, req.Model, resolveErr)
		return
	}
	externalModel := req.Model
	requestEvent := s.auditBaseEvent(r, audit.EventRequest, routes.ResponsesPath, externalModel)
	requestEvent.Body = rawBody(req)
	s.audit.Record(r.Context(), requestEvent)
	if req.Stream {
		s.streamResponse(w, r, route, externalModel, chatReq)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.requestTimeout)
	defer cancel()
	chatResp, providerName, upstreamModel, err := s.createChatCompletionWithFallback(ctx, r, routes.ResponsesPath, route, externalModel, chatReq)
	if err != nil {
		s.writeAuditedError(w, r, routes.ResponsesPath, externalModel, providerError(err))
		return
	}
	response, conversionErr := compat.NewResponseEnvelope(externalModel, chatResp, time.Now(), responseIdentifier("resp"), responseIdentifier("msg"))
	if conversionErr != nil {
		s.writeAuditedError(w, r, routes.ResponsesPath, externalModel, conversionErr)
		return
	}
	responseEvent := s.auditBaseEvent(r, audit.EventResponse, routes.ResponsesPath, externalModel)
	responseEvent.Provider = providerName
	responseEvent.UpstreamModel = upstreamModel
	responseEvent.Status = http.StatusOK
	responseEvent.Body = rawBody(response)
	s.audit.Record(r.Context(), responseEvent)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func (s *Server) streamResponse(w http.ResponseWriter, r *http.Request, route router.ModelRoute, externalModel string, req compat.ChatCompletionRequest) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, r, compat.ServerError(http.StatusInternalServerError, "streaming unsupported"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.streamTimeout)
	defer cancel()
	stream, providerName, upstreamModel, pricing, err := s.openChatCompletionStreamWithFallback(ctx, r, routes.ResponsesPath, route, externalModel, req)
	if err != nil {
		s.writeAuditedError(w, r, routes.ResponsesPath, externalModel, providerError(err))
		return
	}
	defer stream.Close()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	responseID, messageID := responseIdentifier("resp"), responseIdentifier("msg")
	created := time.Now()
	sequence := 0
	text := ""
	var usage *compat.Usage
	emit := func(eventType string, fields map[string]any) bool {
		fields["type"] = eventType
		fields["sequence_number"] = sequence
		sequence++
		if err := writeTypedSSE(w, eventType, fields); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	base := &compat.Response{ID: responseID, Object: "response", CreatedAt: created.Unix(), Status: "in_progress", Model: externalModel, Output: []compat.ResponseOutputMessage{}, ParallelToolCalls: true, Store: false, Tools: []any{}}
	if !emit("response.created", map[string]any{"response": base}) || !emit("response.in_progress", map[string]any{"response": base}) {
		return
	}
	item := compat.ResponseOutputMessage{ID: messageID, Type: "message", Status: "in_progress", Role: "assistant", Content: []compat.ResponseOutputText{}}
	if !emit("response.output_item.added", map[string]any{"output_index": 0, "item": item}) ||
		!emit("response.content_part.added", map[string]any{"item_id": messageID, "output_index": 0, "content_index": 0, "part": compat.ResponseOutputText{Type: "output_text", Text: "", Annotations: []any{}}}) {
		return
	}
	for {
		chunk, nextErr := stream.Next(ctx)
		if nextErr != nil {
			if errors.Is(nextErr, io.EOF) {
				break
			}
			if errors.Is(nextErr, context.DeadlineExceeded) || canFallbackProviderError(nextErr) {
				s.providerHealth.MarkFailure(providerName)
				s.observeProviderHealth(providerName)
			}
			if !errors.Is(nextErr, context.Canceled) {
				emit("error", map[string]any{"error": compat.ErrorResponseFor(providerError(nextErr)).Error})
			}
			return
		}
		if len(chunk.Choices) > 1 || (len(chunk.Choices) == 1 && (chunk.Choices[0].Index != 0 || len(chunk.Choices[0].Delta.Extra) != 0)) {
			emit("error", map[string]any{"error": compat.ErrorResponseFor(compat.ServerError(http.StatusBadGateway, "provider returned unsupported stream content")).Error})
			return
		}
		if len(chunk.Choices) == 1 && chunk.Choices[0].Delta.Content != "" {
			delta := chunk.Choices[0].Delta.Content
			text += delta
			if !emit("response.output_text.delta", map[string]any{"item_id": messageID, "output_index": 0, "content_index": 0, "delta": delta}) {
				return
			}
		}
		if chunk.Usage != nil {
			usage = chunk.Usage
			s.observeUsage(routes.ResponsesPath, externalModel, providerName, clientFromContext(r.Context()), chunk.Usage, pricing)
		}
	}
	donePart := compat.ResponseOutputText{Type: "output_text", Text: text, Annotations: []any{}}
	doneItem := compat.ResponseOutputMessage{ID: messageID, Type: "message", Status: "completed", Role: "assistant", Content: []compat.ResponseOutputText{donePart}}
	if !emit("response.output_text.done", map[string]any{"item_id": messageID, "output_index": 0, "content_index": 0, "text": text}) ||
		!emit("response.content_part.done", map[string]any{"item_id": messageID, "output_index": 0, "content_index": 0, "part": donePart}) ||
		!emit("response.output_item.done", map[string]any{"output_index": 0, "item": doneItem}) {
		return
	}
	completed := *base
	completed.Status = "completed"
	completed.Output = []compat.ResponseOutputMessage{doneItem}
	if usage != nil {
		completed.Usage = &compat.ResponseUsage{InputTokens: usage.PromptTokens, OutputTokens: usage.CompletionTokens, TotalTokens: usage.TotalTokens}
	}
	emit("response.completed", map[string]any{"response": &completed})
	doneEvent := s.auditBaseEvent(r, audit.EventStreamDone, routes.ResponsesPath, externalModel)
	doneEvent.Provider, doneEvent.UpstreamModel, doneEvent.Status = providerName, upstreamModel, http.StatusOK
	s.audit.Record(r.Context(), doneEvent)
}

func writeTypedSSE(w io.Writer, eventType string, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, payload)
	return err
}

func responseIdentifier(prefix string) string {
	var value [12]byte
	if _, err := rand.Read(value[:]); err != nil {
		return prefix + "_" + strings.Repeat("0", 24)
	}
	return prefix + "_" + hex.EncodeToString(value[:])
}
