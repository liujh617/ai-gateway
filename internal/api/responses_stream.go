package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"open-ai-gateway/internal/audit"
	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/conversation"
	"open-ai-gateway/internal/middleware"
	"open-ai-gateway/internal/provider"
	providerdialect "open-ai-gateway/internal/provider/dialect"
	"open-ai-gateway/internal/reasoningenvelope"
	"open-ai-gateway/internal/responsestore"
	"open-ai-gateway/internal/router"
	"open-ai-gateway/internal/routes"
)

func (s *Server) handleDialectStreamingResponse(w http.ResponseWriter, r *http.Request, req compat.ResponseRequest) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, r, compat.ServerError(http.StatusInternalServerError, "streaming unsupported"))
		return
	}
	binding := reasoningenvelope.Binding{Audience: s.reasoningAudience, Client: clientFromContext(r.Context()), ExternalModel: req.Model}
	current, validationErr := req.ConversationRequest(func(token string) (conversation.Reasoning, error) {
		if s.reasoningEnvelope == nil {
			return conversation.Reasoning{}, reasoningenvelope.ErrInvalid
		}
		payload, err := s.reasoningEnvelope.Open(token, binding)
		if err != nil {
			return conversation.Reasoning{}, err
		}
		return reasoningFromPayload(payload), nil
	})
	if validationErr != nil {
		s.writeAuditedError(w, r, routes.ResponsesPath, req.Model, validationErr)
		return
	}
	history, stateErr := s.responseConversationHistory(r, req.PreviousResponseID, req.ConversationID(), req.Model)
	if stateErr != nil {
		s.writeAuditedError(w, r, routes.ResponsesPath, req.Model, stateErr)
		return
	}
	currentItems := current.Turn.Items
	storedCurrent := currentItems
	combined := conversation.CloneRequest(current)
	if req.Instructions != "" {
		storedCurrent = currentItems[1:]
		combined.Turn.Items = append([]conversation.Item{currentItems[0]}, history...)
		combined.Turn.Items = append(combined.Turn.Items, storedCurrent...)
	} else {
		combined.Turn.Items = append(append([]conversation.Item(nil), history...), currentItems...)
	}
	if err := conversation.ValidateRequest(combined, s.conversationLimits); err != nil {
		s.writeAuditedError(w, r, routes.ResponsesPath, req.Model, compat.InvalidRequest(err.Error(), "input"))
		return
	}
	if !s.modelAllowedForRequest(r, req.Model) {
		middleware.SetLogRoute(r.Context(), req.Model, "", "")
		s.writeAuditedError(w, r, routes.ResponsesPath, req.Model, compat.ModelNotFound(req.Model))
		return
	}
	modelRoute, resolveErr := s.router.ResolveFor(req.Model, "chat")
	if resolveErr != nil {
		s.writeAuditedError(w, r, routes.ResponsesPath, req.Model, resolveErr)
		return
	}
	attempts, routeErr := responseAttempts(modelRoute, combined)
	if routeErr != nil {
		s.writeAuditedError(w, r, routes.ResponsesPath, req.Model, compat.InvalidRequest("invalid reasoning item", "input"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.streamTimeout)
	defer cancel()
	stream, decoder, attempt, openErr := s.openResponseDialectStream(ctx, r, req.Model, combined, attempts)
	if openErr != nil {
		s.writeAuditedError(w, r, routes.ResponsesPath, req.Model, responseDialectError(openErr))
		return
	}
	defer stream.Close()
	requestEvent := s.auditBaseEvent(r, audit.EventRequest, routes.ResponsesPath, req.Model)
	requestEvent.PreviousResponseID, requestEvent.Body = req.PreviousResponseID, rawBody(req)
	s.audit.Record(r.Context(), requestEvent)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	responseID, messageID := responseIdentifier("resp"), responseIdentifier("msg")
	sequence := 0
	emit := func(eventType string, fields map[string]any) bool {
		fields["type"], fields["sequence_number"] = eventType, sequence
		sequence++
		if err := writeTypedSSE(w, eventType, fields); err != nil {
			return false
		}
		event := s.auditBaseEvent(r, audit.EventStreamChunk, routes.ResponsesPath, req.Model)
		event.Provider, event.UpstreamModel, event.Status, event.Body = attempt.ProviderName, attempt.UpstreamModel, http.StatusOK, rawBody(fields)
		s.audit.Record(r.Context(), event)
		flusher.Flush()
		return true
	}
	shouldStore := req.Store == nil || *req.Store
	willStore := shouldStore && s.responseStore != nil && s.responseStore.Enabled()
	var previousResponseID any
	if req.PreviousResponseID != "" {
		previousResponseID = req.PreviousResponseID
	}
	base := &compat.Response{ID: responseID, Object: "response", CreatedAt: time.Now().Unix(), Status: "in_progress", Model: req.Model, Output: []compat.ResponseOutputMessage{}, ParallelToolCalls: true, PreviousResponseID: previousResponseID, ConversationID: req.ConversationID(), Store: willStore, Tools: []any{}}
	if !emit("response.created", map[string]any{"response": base}) || !emit("response.in_progress", map[string]any{"response": base}) {
		return
	}

	dialectName := responseDialectName(attempt.Dialect)
	textOutputIndex := 0
	if dialectName == "deepseek" {
		textOutputIndex = 1
	}
	textStarted := false
	text := strings.Builder{}
	for {
		chunk, err := stream.Next(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			if errors.Is(err, context.DeadlineExceeded) || canFallbackProviderError(err) {
				s.providerHealth.MarkFailure(attempt.ProviderName)
				s.observeProviderHealth(attempt.ProviderName)
			}
			if !errors.Is(err, context.Canceled) {
				emit("error", map[string]any{"error": compat.ErrorResponseFor(providerError(err)).Error})
			}
			return
		}
		events, err := decoder.Push(*chunk)
		if err != nil {
			emit("error", map[string]any{"error": compat.ErrorResponseFor(responseDialectError(err)).Error})
			return
		}
		for _, event := range events {
			if event.TextDelta == "" {
				continue
			}
			if !textStarted {
				textStarted = true
				if !emit("response.output_item.added", map[string]any{"output_index": textOutputIndex, "item": compat.ResponseOutputMessage{ID: messageID, Type: "message", Status: "in_progress", Role: "assistant"}}) || !emit("response.content_part.added", map[string]any{"item_id": messageID, "output_index": textOutputIndex, "content_index": 0, "part": compat.ResponseOutputText{Type: "output_text", Text: "", Annotations: []any{}}}) {
					return
				}
			}
			text.WriteString(event.TextDelta)
			if !emit("response.output_text.delta", map[string]any{"item_id": messageID, "output_index": textOutputIndex, "content_index": 0, "delta": event.TextDelta}) {
				return
			}
		}
	}
	result, finishErr := decoder.Finish()
	if finishErr != nil {
		emit("error", map[string]any{"error": compat.ErrorResponseFor(responseDialectError(finishErr)).Error})
		return
	}
	actualRoute := conversation.RouteBinding{Dialect: dialectName, Provider: attempt.ProviderName, UpstreamModel: attempt.UpstreamModel}
	attachReasoningRoute(&result.Turn, actualRoute)
	completed, conversionErr := compat.NewResponseEnvelopeFromTurn(req.Model, result.Turn, result.Usage, time.Unix(base.CreatedAt, 0), responseID, messageID, func(item conversation.Reasoning) (string, error) {
		if s.reasoningEnvelope == nil {
			return "", errors.New("reasoning envelope codec is unavailable")
		}
		return s.reasoningEnvelope.Seal(payloadFromReasoning(item), binding)
	})
	if conversionErr != nil {
		emit("error", map[string]any{"error": compat.ErrorResponseFor(conversionErr).Error})
		return
	}
	completed.PreviousResponseID, completed.ConversationID, completed.Store = previousResponseID, req.ConversationID(), willStore
	for outputIndex, item := range completed.Output {
		switch item.Type {
		case "reasoning":
			if !emit("response.output_item.added", map[string]any{"output_index": outputIndex, "item": item}) || !emit("response.output_item.done", map[string]any{"output_index": outputIndex, "item": item}) {
				return
			}
		case "message":
			if !textStarted {
				if !emit("response.output_item.added", map[string]any{"output_index": outputIndex, "item": compat.ResponseOutputMessage{ID: item.ID, Type: "message", Status: "in_progress", Role: "assistant"}}) || !emit("response.content_part.added", map[string]any{"item_id": item.ID, "output_index": outputIndex, "content_index": 0, "part": compat.ResponseOutputText{Type: "output_text", Text: "", Annotations: []any{}}}) {
					return
				}
			}
			visible := item.Content[0]
			if !emit("response.output_text.done", map[string]any{"item_id": item.ID, "output_index": outputIndex, "content_index": 0, "text": visible.Text}) || !emit("response.content_part.done", map[string]any{"item_id": item.ID, "output_index": outputIndex, "content_index": 0, "part": visible}) || !emit("response.output_item.done", map[string]any{"output_index": outputIndex, "item": item}) {
				return
			}
		case "function_call":
			pending := item
			pending.Status, pending.Arguments = "in_progress", ""
			if !emit("response.output_item.added", map[string]any{"output_index": outputIndex, "item": pending}) {
				return
			}
			if item.Arguments != "" && !emit("response.function_call_arguments.delta", map[string]any{"item_id": item.ID, "output_index": outputIndex, "delta": item.Arguments}) {
				return
			}
			if !emit("response.function_call_arguments.done", map[string]any{"item_id": item.ID, "output_index": outputIndex, "arguments": item.Arguments}) || !emit("response.output_item.done", map[string]any{"output_index": outputIndex, "item": item}) {
				return
			}
		}
	}
	if textStarted {
		found := false
		for index, item := range completed.Output {
			if item.Type == "message" && index == textOutputIndex && item.Content[0].Text == text.String() {
				found = true
			}
		}
		if !found {
			emit("error", map[string]any{"error": compat.ErrorResponseFor(compat.ServerError(http.StatusBadGateway, "provider returned inconsistent stream content")).Error})
			return
		}
	}
	if !emit("response.completed", map[string]any{"response": completed}) {
		return
	}
	if _, err := io.WriteString(w, "data: [DONE]\n\n"); err != nil {
		return
	}
	flusher.Flush()
	if willStore {
		payload, err := json.Marshal(completed)
		stored := conversation.CloneRequest(current)
		stored.Turn.Items = append(append(append([]conversation.Item(nil), history...), storedCurrent...), result.Turn.Items...)
		if err != nil || s.responseStore.Put(responsestore.Record{ID: completed.ID, Client: binding.Client, Model: req.Model, ConversationID: req.ConversationID(), Conversation: stored, Response: payload}) != nil {
			s.logger.Warn("failed to store completed response stream")
		}
	}
	s.observeUsage(routes.ResponsesPath, req.Model, attempt.ProviderName, binding.Client, result.Usage, attempt.Pricing)
	doneEvent := s.auditBaseEvent(r, audit.EventStreamDone, routes.ResponsesPath, req.Model)
	doneEvent.Provider, doneEvent.UpstreamModel, doneEvent.Status = attempt.ProviderName, attempt.UpstreamModel, http.StatusOK
	s.audit.Record(r.Context(), doneEvent)
}

func (s *Server) openResponseDialectStream(ctx context.Context, r *http.Request, externalModel string, request conversation.Request, attempts []router.ProviderRoute) (provider.ChatCompletionStream, providerdialect.StreamDecoder, router.ProviderRoute, error) {
	var lastErr error
	for index, attempt := range attempts {
		if !s.providerHealth.Healthy(attempt.ProviderName) {
			continue
		}
		dialect, err := s.dialects.Get(responseDialectName(attempt.Dialect))
		if err != nil {
			return nil, nil, router.ProviderRoute{}, err
		}
		if hasReasoningReplay(request) && (!attempt.ReasoningReplay || !dialect.Capabilities().ReasoningReplay) {
			return nil, nil, router.ProviderRoute{}, errors.New("reasoning replay is unsupported")
		}
		route := conversation.RouteBinding{Dialect: dialect.Name(), Provider: attempt.ProviderName, UpstreamModel: attempt.UpstreamModel}
		chatRequest, err := dialect.BuildChatRequest(providerdialect.Request{Conversation: request, UpstreamModel: attempt.UpstreamModel, Route: route})
		if err != nil {
			return nil, nil, router.ProviderRoute{}, err
		}
		middleware.SetLogRoute(r.Context(), externalModel, attempt.ProviderName, attempt.UpstreamModel)
		stream, err := attempt.Provider.StreamChatCompletion(ctx, chatRequest)
		if err == nil {
			s.providerHealth.MarkSuccess(attempt.ProviderName)
			s.observeProviderHealth(attempt.ProviderName)
			return stream, dialect.NewStreamDecoder(), attempt, nil
		}
		lastErr = err
		if canFallbackProviderError(err) {
			s.providerHealth.MarkFailure(attempt.ProviderName)
			s.observeProviderHealth(attempt.ProviderName)
		}
		if index == len(attempts)-1 || !canFallbackProviderError(err) {
			return nil, nil, router.ProviderRoute{}, err
		}
		if next := s.nextHealthyProviderName(attempts[index+1:]); next != "" {
			s.observeProviderFallback(r.Context(), routes.ResponsesPath, externalModel, attempt.ProviderName, next)
		}
	}
	if lastErr != nil {
		return nil, nil, router.ProviderRoute{}, lastErr
	}
	return nil, nil, router.ProviderRoute{}, providerUnavailableError()
}
