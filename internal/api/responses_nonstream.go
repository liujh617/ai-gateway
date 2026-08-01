package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"open-ai-gateway/internal/audit"
	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/conversation"
	"open-ai-gateway/internal/middleware"
	"open-ai-gateway/internal/provider/deepseek"
	providerdialect "open-ai-gateway/internal/provider/dialect"
	"open-ai-gateway/internal/reasoningenvelope"
	"open-ai-gateway/internal/responsestore"
	"open-ai-gateway/internal/router"
	"open-ai-gateway/internal/routes"
)

func (s *Server) handleNonStreamingResponse(w http.ResponseWriter, r *http.Request, req compat.ResponseRequest) {
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
		middleware.SetLogRoute(r.Context(), req.Model, "", "")
		s.writeAuditedError(w, r, routes.ResponsesPath, req.Model, resolveErr)
		return
	}
	attempts, routeErr := responseAttempts(modelRoute, combined)
	if routeErr != nil {
		s.writeAuditedError(w, r, routes.ResponsesPath, req.Model, compat.InvalidRequest("invalid reasoning item", "input"))
		return
	}

	requestEvent := s.auditBaseEvent(r, audit.EventRequest, routes.ResponsesPath, req.Model)
	requestEvent.PreviousResponseID = req.PreviousResponseID
	requestEvent.Body = rawBody(req)
	s.audit.Record(r.Context(), requestEvent)

	ctx, cancel := context.WithTimeout(r.Context(), s.requestTimeout)
	defer cancel()
	result, usedAttempt, executeErr := s.executeResponseDialect(ctx, r, req.Model, combined, attempts)
	if executeErr != nil {
		s.writeAuditedError(w, r, routes.ResponsesPath, req.Model, responseDialectError(executeErr))
		return
	}
	actualRoute := conversation.RouteBinding{Dialect: responseDialectName(usedAttempt.Dialect), Provider: usedAttempt.ProviderName, UpstreamModel: usedAttempt.UpstreamModel}
	attachReasoningRoute(&result.Turn, actualRoute)

	responseID := responseIdentifier("resp")
	response, conversionErr := compat.NewResponseEnvelopeFromTurn(req.Model, result.Turn, result.Usage, time.Now(), responseID, responseIdentifier("msg"), func(item conversation.Reasoning) (string, error) {
		if s.reasoningEnvelope == nil {
			return "", errors.New("reasoning envelope codec is unavailable")
		}
		return s.reasoningEnvelope.Seal(payloadFromReasoning(item), binding)
	})
	if conversionErr != nil {
		s.writeAuditedError(w, r, routes.ResponsesPath, req.Model, conversionErr)
		return
	}
	response.PreviousResponseID = nil
	if req.PreviousResponseID != "" {
		response.PreviousResponseID = req.PreviousResponseID
	}
	response.ConversationID = req.ConversationID()
	shouldStore := req.Store == nil || *req.Store
	willStore := shouldStore && s.responseStore != nil && s.responseStore.Enabled()
	response.Store = willStore
	if willStore {
		payload, err := json.Marshal(response)
		if err != nil {
			s.writeAuditedError(w, r, routes.ResponsesPath, req.Model, compat.ServerError(http.StatusInternalServerError, "failed to store response state"))
			return
		}
		stored := conversation.CloneRequest(current)
		stored.Turn.Items = append(append(append([]conversation.Item(nil), history...), storedCurrent...), result.Turn.Items...)
		if err := s.responseStore.Put(responsestore.Record{ID: response.ID, Client: binding.Client, Model: req.Model, ConversationID: req.ConversationID(), Conversation: stored, Response: payload}); err != nil {
			if errors.Is(err, responsestore.ErrContextTooLarge) {
				s.writeAuditedError(w, r, routes.ResponsesPath, req.Model, compat.InvalidRequest("response context is too large", "previous_response_id"))
				return
			}
			s.writeAuditedError(w, r, routes.ResponsesPath, req.Model, compat.ServerError(http.StatusInternalServerError, "failed to store response state"))
			return
		}
	}
	s.observeUsage(routes.ResponsesPath, req.Model, usedAttempt.ProviderName, binding.Client, result.Usage, usedAttempt.Pricing)
	responseEvent := s.auditBaseEvent(r, audit.EventResponse, routes.ResponsesPath, req.Model)
	responseEvent.Provider, responseEvent.UpstreamModel, responseEvent.Status = usedAttempt.ProviderName, usedAttempt.UpstreamModel, http.StatusOK
	responseEvent.Body = rawBody(response)
	s.audit.Record(r.Context(), responseEvent)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func (s *Server) executeResponseDialect(ctx context.Context, r *http.Request, externalModel string, request conversation.Request, attempts []router.ProviderRoute) (providerdialect.Response, router.ProviderRoute, error) {
	var lastErr error
	for index, attempt := range attempts {
		if !s.providerHealth.Healthy(attempt.ProviderName) {
			s.observeProviderHealth(attempt.ProviderName)
			s.observeProviderCircuitOpen(r.Context(), routes.ResponsesPath, externalModel, attempt.ProviderName)
			continue
		}
		dialect, err := s.dialects.Get(responseDialectName(attempt.Dialect))
		if err != nil {
			return providerdialect.Response{}, router.ProviderRoute{}, err
		}
		if hasReasoningReplay(request) && (!attempt.ReasoningReplay || !dialect.Capabilities().ReasoningReplay) {
			return providerdialect.Response{}, router.ProviderRoute{}, deepseek.ErrReasoningRouteMismatch
		}
		route := conversation.RouteBinding{Dialect: dialect.Name(), Provider: attempt.ProviderName, UpstreamModel: attempt.UpstreamModel}
		chatRequest, err := dialect.BuildChatRequest(providerdialect.Request{Conversation: request, UpstreamModel: attempt.UpstreamModel, Route: route})
		if err != nil {
			return providerdialect.Response{}, router.ProviderRoute{}, err
		}
		middleware.SetLogRoute(r.Context(), externalModel, attempt.ProviderName, attempt.UpstreamModel)
		chatResponse, err := attempt.Provider.CreateChatCompletion(ctx, chatRequest)
		if err != nil {
			lastErr = err
			if canFallbackProviderError(err) {
				s.providerHealth.MarkFailure(attempt.ProviderName)
				s.observeProviderHealth(attempt.ProviderName)
			}
			if index == len(attempts)-1 || !canFallbackProviderError(err) {
				return providerdialect.Response{}, router.ProviderRoute{}, err
			}
			if next := s.nextHealthyProviderName(attempts[index+1:]); next != "" {
				s.observeProviderFallback(r.Context(), routes.ResponsesPath, externalModel, attempt.ProviderName, next)
			}
			continue
		}
		s.providerHealth.MarkSuccess(attempt.ProviderName)
		s.observeProviderHealth(attempt.ProviderName)
		parsed, err := dialect.ParseChatResponse(*chatResponse)
		if err != nil {
			return providerdialect.Response{}, router.ProviderRoute{}, err
		}
		return parsed, attempt, nil
	}
	if lastErr != nil {
		return providerdialect.Response{}, router.ProviderRoute{}, lastErr
	}
	return providerdialect.Response{}, router.ProviderRoute{}, providerUnavailableError()
}

func responseAttempts(route router.ModelRoute, request conversation.Request) ([]router.ProviderRoute, error) {
	pinned, ok, err := request.PinnedRoute()
	if err != nil {
		return nil, err
	}
	if !ok {
		return route.Attempts(), nil
	}
	attempt, found := route.MatchAttempt(pinned.Provider, pinned.UpstreamModel, pinned.Dialect)
	if !found || !attempt.ReasoningReplay {
		return nil, errors.New("reasoning route is unavailable")
	}
	return []router.ProviderRoute{attempt}, nil
}

func (s *Server) responseConversationHistory(r *http.Request, previousResponseID, conversationID, model string) ([]conversation.Item, *compat.Error) {
	if previousResponseID == "" && conversationID == "" {
		return nil, nil
	}
	if s.responseStore == nil || !s.responseStore.Enabled() {
		return nil, compat.InvalidRequest("response store is disabled", "previous_response_id")
	}
	client := clientFromContext(r.Context())
	if previousResponseID != "" {
		record, reason, ok := s.responseStore.Get(previousResponseID, client, model)
		if ok {
			return recordConversationItems(record)
		}
		if reason == responsestore.MissModel {
			return nil, compat.InvalidRequest("previous response model does not match request model", "previous_response_id")
		}
		param := "previous_response_id"
		return nil, compat.NewError(http.StatusNotFound, "invalid_request_error", "previous response not found", &param)
	}
	records := s.responseStore.ConversationResponses(conversationID, client)
	if len(records) == 0 {
		return nil, nil
	}
	return recordConversationItems(records[len(records)-1])
}

func recordConversationItems(record responsestore.Record) ([]conversation.Item, *compat.Error) {
	if len(record.Conversation.Turn.Items) > 0 {
		return conversation.CloneRequest(record.Conversation).Turn.Items, nil
	}
	items, err := transcriptConversationItems(record.Transcript)
	if err != nil {
		return nil, compat.ServerError(http.StatusInternalServerError, "stored response state is invalid")
	}
	return items, nil
}

func transcriptConversationItems(messages []compat.ChatMessage) ([]conversation.Item, error) {
	items := make([]conversation.Item, 0, len(messages))
	for _, message := range messages {
		role := message.Role
		if role == "developer" {
			role = "system"
		}
		if role == "tool" {
			var callID string
			if json.Unmarshal(message.Extra["tool_call_id"], &callID) != nil || callID == "" || len(message.Content) == 0 {
				return nil, errors.New("invalid stored tool output")
			}
			items = append(items, conversation.FunctionOutput{CallID: callID, Output: append(json.RawMessage(nil), message.Content...)})
			continue
		}
		if role != "system" && role != "user" && role != "assistant" {
			return nil, fmt.Errorf("invalid stored role %q", role)
		}
		if text, ok := chatMessageText(message.Content); ok && text != "" {
			items = append(items, conversation.Message{Role: role, Text: text})
		}
		if raw := message.Extra["tool_calls"]; len(raw) > 0 {
			var calls []struct {
				ID       string                           `json:"id"`
				Type     string                           `json:"type"`
				Function struct{ Name, Arguments string } `json:"function"`
			}
			if json.Unmarshal(raw, &calls) != nil {
				return nil, errors.New("invalid stored function calls")
			}
			for _, call := range calls {
				items = append(items, conversation.FunctionCall{CallID: call.ID, Name: call.Function.Name, Arguments: call.Function.Arguments})
			}
		}
	}
	return items, nil
}

func chatMessageText(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return "", true
	}
	var text string
	return text, json.Unmarshal(raw, &text) == nil
}

func reasoningFromPayload(payload reasoningenvelope.Payload) conversation.Reasoning {
	return conversation.Reasoning{EnvelopeID: payload.EnvelopeID, Content: payload.ReasoningContent, AssistantContent: payload.AssistantContent, CallIDs: append([]string(nil), payload.CallIDs...), Route: conversation.RouteBinding{Dialect: payload.Route.Dialect, Provider: payload.Route.Provider, UpstreamModel: payload.Route.UpstreamModel}}
}

func payloadFromReasoning(item conversation.Reasoning) reasoningenvelope.Payload {
	return reasoningenvelope.Payload{EnvelopeID: item.EnvelopeID, Route: reasoningenvelope.RouteBinding{Dialect: item.Route.Dialect, Provider: item.Route.Provider, UpstreamModel: item.Route.UpstreamModel}, ReasoningContent: item.Content, AssistantContent: item.AssistantContent, CallIDs: append([]string(nil), item.CallIDs...)}
}

func attachReasoningRoute(turn *conversation.Turn, route conversation.RouteBinding) {
	for index, raw := range turn.Items {
		if item, ok := raw.(conversation.Reasoning); ok {
			item.Route = route
			turn.Items[index] = item
		}
	}
}

func hasReasoningReplay(request conversation.Request) bool {
	for _, item := range request.Turn.Items {
		if reasoning, ok := item.(conversation.Reasoning); ok && len(reasoning.CallIDs) > 0 {
			return true
		}
	}
	return false
}

func responseDialectName(name string) string {
	if name == "" {
		return "openai-compatible"
	}
	return name
}

func responseDialectError(err error) *compat.Error {
	switch {
	case errors.Is(err, deepseek.ErrReasoningRequired):
		return compat.InvalidRequest("reasoning item is required for this tool call", "input")
	case errors.Is(err, deepseek.ErrReasoningRouteMismatch), errors.Is(err, reasoningenvelope.ErrInvalid):
		return compat.InvalidRequest("invalid reasoning item", "input")
	case errors.Is(err, deepseek.ErrIncompleteReasoningToolCall):
		return compat.ServerError(http.StatusBadGateway, deepseek.ErrIncompleteReasoningToolCall.Error())
	default:
		return providerError(err)
	}
}
