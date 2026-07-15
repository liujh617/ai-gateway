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
	"sort"
	"strings"
	"time"

	"open-ai-gateway/internal/audit"
	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/middleware"
	"open-ai-gateway/internal/responsestore"
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
	middleware.SetLogPreviousResponse(r.Context(), req.PreviousResponseID != "")
	chatReq, validationErr := req.ChatRequest()
	if validationErr != nil {
		s.writeAuditedError(w, r, routes.ResponsesPath, req.Model, validationErr)
		return
	}
	history, stateErr := s.responseHistory(r, req.PreviousResponseID, req.Model)
	if stateErr != nil {
		s.writeAuditedError(w, r, routes.ResponsesPath, req.Model, stateErr)
		return
	}
	currentMessages := chatReq.Messages
	if req.Instructions != "" {
		currentMessages = currentMessages[1:]
		chatReq.Messages = append([]compat.ChatMessage{chatReq.Messages[0]}, history...)
		chatReq.Messages = append(chatReq.Messages, currentMessages...)
	} else {
		chatReq.Messages = append(append([]compat.ChatMessage(nil), history...), currentMessages...)
	}
	if !validResponseToolOutputs(history, currentMessages) {
		s.writeAuditedError(w, r, routes.ResponsesPath, req.Model, compat.InvalidRequest("function_call_output references an unknown call", "input"))
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
	requestEvent.PreviousResponseID = req.PreviousResponseID
	requestEvent.Body = rawBody(req)
	s.audit.Record(r.Context(), requestEvent)
	if req.Stream {
		s.streamResponse(w, r, route, externalModel, chatReq, history, currentMessages, req)
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
	response.PreviousResponseID = nil
	if req.PreviousResponseID != "" {
		response.PreviousResponseID = req.PreviousResponseID
	}
	shouldStore := req.Store == nil || *req.Store
	willStore := shouldStore && s.responseStore != nil && s.responseStore.Enabled()
	response.Store = willStore
	if willStore {
		payload, err := json.Marshal(response)
		if err != nil {
			s.writeAuditedError(w, r, routes.ResponsesPath, externalModel, compat.ServerError(http.StatusInternalServerError, "failed to store response state"))
			return
		}
		transcript := append(append(append([]compat.ChatMessage(nil), history...), currentMessages...), chatResp.Choices[0].Message)
		err = s.responseStore.Put(responsestore.Record{ID: response.ID, Client: clientFromContext(r.Context()), Model: externalModel, Transcript: transcript, Response: payload})
		if err != nil {
			if errors.Is(err, responsestore.ErrContextTooLarge) {
				s.writeAuditedError(w, r, routes.ResponsesPath, externalModel, compat.InvalidRequest("response context is too large", "previous_response_id"))
				return
			}
			s.writeAuditedError(w, r, routes.ResponsesPath, externalModel, compat.ServerError(http.StatusInternalServerError, "failed to store response state"))
			return
		}
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

func (s *Server) handleResponse(w http.ResponseWriter, r *http.Request) {
	responseID := r.PathValue("response_id")
	if r.Method == http.MethodDelete {
		s.deleteResponse(w, r, responseID)
		return
	}
	if s.responseStore == nil || !s.responseStore.Enabled() {
		s.writeError(w, r, responseNotFound())
		return
	}
	record, _, ok := s.responseStore.GetByID(responseID, clientFromContext(r.Context()))
	if !ok || len(record.Response) == 0 {
		s.writeError(w, r, responseNotFound())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(record.Response)
}

func (s *Server) deleteResponse(w http.ResponseWriter, r *http.Request, responseID string) {
	if s.responseStore == nil || !s.responseStore.Enabled() {
		s.writeError(w, r, responseNotFound())
		return
	}
	if _, ok := s.responseStore.DeleteByID(responseID, clientFromContext(r.Context())); !ok {
		s.writeError(w, r, responseNotFound())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(compat.DeletedResponse{ID: responseID, Object: "response", Deleted: true})
}

func responseNotFound() *compat.Error {
	param := "response_id"
	return compat.NewError(http.StatusNotFound, "invalid_request_error", "response not found", &param)
}

func validResponseToolOutputs(history, current []compat.ChatMessage) bool {
	known := make(map[string]bool)
	for _, message := range append(append([]compat.ChatMessage(nil), history...), current...) {
		if raw := message.Extra["tool_calls"]; len(raw) > 0 {
			var calls []struct {
				ID string `json:"id"`
			}
			if json.Unmarshal(raw, &calls) != nil {
				return false
			}
			for _, call := range calls {
				known[call.ID] = true
			}
		}
		if raw := message.Extra["tool_call_id"]; len(raw) > 0 {
			var callID string
			if json.Unmarshal(raw, &callID) != nil || !known[callID] {
				return false
			}
		}
	}
	return true
}

func (s *Server) responseHistory(r *http.Request, previousResponseID, model string) ([]compat.ChatMessage, *compat.Error) {
	if previousResponseID == "" {
		return nil, nil
	}
	if s.responseStore == nil || !s.responseStore.Enabled() {
		return nil, compat.InvalidRequest("response store is disabled", "previous_response_id")
	}
	record, reason, ok := s.responseStore.Get(previousResponseID, clientFromContext(r.Context()), model)
	if ok {
		return record.Transcript, nil
	}
	if reason == responsestore.MissModel {
		return nil, compat.InvalidRequest("previous response model does not match request model", "previous_response_id")
	}
	param := "previous_response_id"
	return nil, compat.NewError(http.StatusNotFound, "invalid_request_error", "previous response not found", &param)
}

func (s *Server) streamResponse(w http.ResponseWriter, r *http.Request, route router.ModelRoute, externalModel string, req compat.ChatCompletionRequest, history, currentMessages []compat.ChatMessage, responseReq compat.ResponseRequest) {
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
	textStarted := false
	textOutputIndex := -1
	nextOutputIndex := 0
	functionStates := map[int]*responseFunctionStreamState{}
	functionOrder := make([]*responseFunctionStreamState, 0)
	var usage *compat.Usage
	emit := func(eventType string, fields map[string]any) bool {
		fields["type"] = eventType
		fields["sequence_number"] = sequence
		sequence++
		if err := writeTypedSSE(w, eventType, fields); err != nil {
			return false
		}
		chunkEvent := s.auditBaseEvent(r, audit.EventStreamChunk, routes.ResponsesPath, externalModel)
		chunkEvent.Provider = providerName
		chunkEvent.UpstreamModel = upstreamModel
		chunkEvent.Status = http.StatusOK
		chunkEvent.Body = rawBody(fields)
		s.audit.Record(r.Context(), chunkEvent)
		flusher.Flush()
		return true
	}
	shouldStore := responseReq.Store == nil || *responseReq.Store
	willStore := shouldStore && s.responseStore != nil && s.responseStore.Enabled()
	var previousResponseID any
	if responseReq.PreviousResponseID != "" {
		previousResponseID = responseReq.PreviousResponseID
	}
	base := &compat.Response{ID: responseID, Object: "response", CreatedAt: created.Unix(), Status: "in_progress", Model: externalModel, Output: []compat.ResponseOutputMessage{}, ParallelToolCalls: true, PreviousResponseID: previousResponseID, Store: willStore, Tools: []any{}}
	if !emit("response.created", map[string]any{"response": base}) || !emit("response.in_progress", map[string]any{"response": base}) {
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
		if len(chunk.Choices) > 1 || (len(chunk.Choices) == 1 && chunk.Choices[0].Index != 0) {
			emit("error", map[string]any{"error": compat.ErrorResponseFor(compat.ServerError(http.StatusBadGateway, "provider returned unsupported stream content")).Error})
			return
		}
		if len(chunk.Choices) == 1 && chunk.Choices[0].Delta.Content != "" {
			if !textStarted {
				textStarted = true
				textOutputIndex = nextOutputIndex
				if !emit("response.output_item.added", map[string]any{"output_index": textOutputIndex, "item": compat.ResponseOutputMessage{ID: messageID, Type: "message", Status: "in_progress", Role: "assistant"}}) || !emit("response.content_part.added", map[string]any{"item_id": messageID, "output_index": textOutputIndex, "content_index": 0, "part": compat.ResponseOutputText{Type: "output_text", Text: "", Annotations: []any{}}}) {
					return
				}
				nextOutputIndex++
			}
			delta := chunk.Choices[0].Delta.Content
			text += delta
			if !emit("response.output_text.delta", map[string]any{"item_id": messageID, "output_index": textOutputIndex, "content_index": 0, "delta": delta}) {
				return
			}
		}
		if len(chunk.Choices) == 1 {
			extra := chunk.Choices[0].Delta.Extra
			for key := range extra {
				if key != "tool_calls" {
					emit("error", map[string]any{"error": compat.ErrorResponseFor(compat.ServerError(http.StatusBadGateway, "provider returned unsupported stream content")).Error})
					return
				}
			}
			if raw := extra["tool_calls"]; len(raw) > 0 {
				var deltas []chatToolCallDelta
				if json.Unmarshal(raw, &deltas) != nil {
					emit("error", map[string]any{"error": compat.ErrorResponseFor(compat.ServerError(http.StatusBadGateway, "provider returned invalid function call stream")).Error})
					return
				}
				sort.SliceStable(deltas, func(i, j int) bool { return deltas[i].Index < deltas[j].Index })
				for _, delta := range deltas {
					state := functionStates[delta.Index]
					if state == nil {
						if delta.Index < 0 || strings.TrimSpace(delta.ID) == "" || delta.Type != "function" || strings.TrimSpace(delta.Function.Name) == "" {
							emit("error", map[string]any{"error": compat.ErrorResponseFor(compat.ServerError(http.StatusBadGateway, "provider returned invalid function call stream")).Error})
							return
						}
						state = &responseFunctionStreamState{ChatIndex: delta.Index, OutputIndex: nextOutputIndex, ItemID: responseIdentifier("fc"), CallID: delta.ID, Name: delta.Function.Name}
						nextOutputIndex++
						functionStates[delta.Index] = state
						functionOrder = append(functionOrder, state)
						item := compat.ResponseOutputMessage{ID: state.ItemID, Type: "function_call", Status: "in_progress", CallID: state.CallID, Name: state.Name, Arguments: ""}
						if !emit("response.output_item.added", map[string]any{"output_index": state.OutputIndex, "item": item}) {
							return
						}
					} else if (delta.ID != "" && delta.ID != state.CallID) || (delta.Function.Name != "" && delta.Function.Name != state.Name) {
						emit("error", map[string]any{"error": compat.ErrorResponseFor(compat.ServerError(http.StatusBadGateway, "provider returned conflicting function call stream")).Error})
						return
					}
					state.Arguments += delta.Function.Arguments
					if delta.Function.Arguments != "" && !emit("response.function_call_arguments.delta", map[string]any{"item_id": state.ItemID, "output_index": state.OutputIndex, "delta": delta.Function.Arguments}) {
						return
					}
				}
			}
		}
		if chunk.Usage != nil {
			usage = chunk.Usage
			s.observeUsage(routes.ResponsesPath, externalModel, providerName, clientFromContext(r.Context()), chunk.Usage, pricing)
		}
	}
	completedOutput := make([]compat.ResponseOutputMessage, nextOutputIndex)
	if textStarted {
		donePart := compat.ResponseOutputText{Type: "output_text", Text: text, Annotations: []any{}}
		doneItem := compat.ResponseOutputMessage{ID: messageID, Type: "message", Status: "completed", Role: "assistant", Content: []compat.ResponseOutputText{donePart}}
		if !emit("response.output_text.done", map[string]any{"item_id": messageID, "output_index": textOutputIndex, "content_index": 0, "text": text}) || !emit("response.content_part.done", map[string]any{"item_id": messageID, "output_index": textOutputIndex, "content_index": 0, "part": donePart}) || !emit("response.output_item.done", map[string]any{"output_index": textOutputIndex, "item": doneItem}) {
			return
		}
		completedOutput[textOutputIndex] = doneItem
	}
	for _, state := range functionOrder {
		if !validJSONValue(state.Arguments) {
			emit("error", map[string]any{"error": compat.ErrorResponseFor(compat.ServerError(http.StatusBadGateway, "provider returned invalid function arguments")).Error})
			return
		}
		item := compat.ResponseOutputMessage{ID: state.ItemID, Type: "function_call", Status: "completed", CallID: state.CallID, Name: state.Name, Arguments: state.Arguments}
		if !emit("response.function_call_arguments.done", map[string]any{"item_id": state.ItemID, "output_index": state.OutputIndex, "arguments": state.Arguments}) || !emit("response.output_item.done", map[string]any{"output_index": state.OutputIndex, "item": item}) {
			return
		}
		completedOutput[state.OutputIndex] = item
	}
	if len(completedOutput) == 0 {
		emit("error", map[string]any{"error": compat.ErrorResponseFor(compat.ServerError(http.StatusBadGateway, "provider returned empty response")).Error})
		return
	}
	completed := *base
	completed.Status = "completed"
	completed.Output = completedOutput
	if usage != nil {
		completed.Usage = &compat.ResponseUsage{InputTokens: usage.PromptTokens, OutputTokens: usage.CompletionTokens, TotalTokens: usage.TotalTokens}
	}
	if !emit("response.completed", map[string]any{"response": &completed}) {
		return
	}
	if willStore {
		payload, err := json.Marshal(&completed)
		if err != nil {
			s.logger.Warn("failed to encode completed response stream", "error", err)
		} else {
			assistant := streamAssistantMessage(text, textStarted, functionOrder)
			transcript := append(append(append([]compat.ChatMessage(nil), history...), currentMessages...), assistant)
			if err := s.responseStore.Put(responsestore.Record{ID: responseID, Client: clientFromContext(r.Context()), Model: externalModel, Transcript: transcript, Response: payload}); err != nil {
				s.logger.Warn("failed to store completed response stream", "error", err)
			}
		}
	}
	doneEvent := s.auditBaseEvent(r, audit.EventStreamDone, routes.ResponsesPath, externalModel)
	doneEvent.Provider, doneEvent.UpstreamModel, doneEvent.Status = providerName, upstreamModel, http.StatusOK
	s.audit.Record(r.Context(), doneEvent)
}

func streamAssistantMessage(text string, textStarted bool, functions []*responseFunctionStreamState) compat.ChatMessage {
	content := json.RawMessage("null")
	if textStarted {
		content, _ = json.Marshal(text)
	}
	message := compat.ChatMessage{Role: "assistant", Content: content}
	if len(functions) == 0 {
		return message
	}
	calls := make([]map[string]any, 0, len(functions))
	for _, state := range functions {
		calls = append(calls, map[string]any{
			"id":       state.CallID,
			"type":     "function",
			"function": map[string]string{"name": state.Name, "arguments": state.Arguments},
		})
	}
	raw, _ := json.Marshal(calls)
	message.Extra = map[string]json.RawMessage{"tool_calls": raw}
	return message
}

type responseFunctionStreamState struct {
	ChatIndex, OutputIndex          int
	ItemID, CallID, Name, Arguments string
}
type chatToolCallDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func validJSONValue(value string) bool {
	var raw any
	return json.Unmarshal([]byte(value), &raw) == nil
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
