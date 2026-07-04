package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/middleware"
	"open-ai-gateway/internal/provider"
	"open-ai-gateway/internal/router"
	"open-ai-gateway/internal/routes"
)

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if err := requireJSONContentType(r); err != nil {
		s.writeError(w, r, err)
		return
	}
	var req compat.ChatCompletionRequest
	if err := decodeJSONBody(s.requestBody(w, r), &req); err != nil {
		s.writeError(w, r, decodeError(err))
		return
	}
	middleware.SetLogStream(r.Context(), req.Stream)
	if err := req.Validate(); err != nil {
		s.writeError(w, r, err)
		return
	}
	if !s.modelAllowedForRequest(r, req.Model) {
		middleware.SetLogRoute(r.Context(), req.Model, "", "")
		s.writeError(w, r, compat.ModelNotFound(req.Model))
		return
	}

	route, resolveErr := s.router.ResolveFor(req.Model, "chat")
	if resolveErr != nil {
		middleware.SetLogRoute(r.Context(), req.Model, "", "")
		s.writeError(w, r, resolveErr)
		return
	}

	externalModel := req.Model

	if req.Stream {
		s.streamChatCompletion(w, r, route, externalModel, req)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.requestTimeout)
	defer cancel()

	resp, err := s.createChatCompletionWithFallback(ctx, r, route, externalModel, req)
	if err != nil {
		s.writeError(w, r, providerError(err))
		return
	}
	resp.Model = externalModel

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) createChatCompletionWithFallback(ctx context.Context, r *http.Request, route router.ModelRoute, externalModel string, req compat.ChatCompletionRequest) (*compat.ChatCompletionResponse, error) {
	var lastErr error
	var skippedFrom string
	attempts := route.Attempts()
	for index, attempt := range attempts {
		if !s.providerHealth.Healthy(attempt.ProviderName) {
			s.observeProviderHealth(attempt.ProviderName)
			s.observeProviderCircuitOpen(r.Context(), routes.ChatCompletionsPath, externalModel, attempt.ProviderName)
			if skippedFrom == "" {
				skippedFrom = attempt.ProviderName
			}
			s.logger.Warn("chat completion provider circuit open; trying fallback", "provider", attempt.ProviderName)
			continue
		}
		if skippedFrom != "" {
			s.observeProviderFallback(r.Context(), routes.ChatCompletionsPath, externalModel, skippedFrom, attempt.ProviderName)
			skippedFrom = ""
		}
		attemptReq := req
		attemptReq.Model = attempt.UpstreamModel
		middleware.SetLogRoute(r.Context(), externalModel, attempt.ProviderName, attempt.UpstreamModel)
		resp, err := attempt.Provider.CreateChatCompletion(ctx, attemptReq)
		if err == nil {
			s.providerHealth.MarkSuccess(attempt.ProviderName)
			s.observeProviderHealth(attempt.ProviderName)
			s.observeUsage(routes.ChatCompletionsPath, externalModel, attempt.ProviderName, clientFromContext(r.Context()), resp.Usage, attempt.Pricing)
			return resp, nil
		}
		lastErr = err
		if canFallbackProviderError(err) {
			s.providerHealth.MarkFailure(attempt.ProviderName)
			s.observeProviderHealth(attempt.ProviderName)
		}
		if index == len(attempts)-1 || !canFallbackProviderError(err) {
			return nil, err
		}
		if nextProviderName := s.nextHealthyProviderName(attempts[index+1:]); nextProviderName != "" {
			s.observeProviderFallback(r.Context(), routes.ChatCompletionsPath, externalModel, attempt.ProviderName, nextProviderName)
		}
		s.logger.Warn("chat completion provider failed; trying fallback", "provider", attempt.ProviderName, "error", err)
	}
	if skippedFrom != "" {
		return nil, providerUnavailableError()
	}
	return nil, lastErr
}

func (s *Server) streamChatCompletion(w http.ResponseWriter, r *http.Request, route router.ModelRoute, externalModel string, req compat.ChatCompletionRequest) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, r, compat.ServerError(http.StatusInternalServerError, "streaming unsupported"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.streamTimeout)
	defer cancel()

	stream, providerName, pricing, err := s.openChatCompletionStreamWithFallback(ctx, r, route, externalModel, req)
	if err != nil {
		s.writeError(w, r, providerError(err))
		return
	}
	defer func() {
		if err := stream.Close(); err != nil {
			s.logger.Debug("failed to close chat completion stream", "error", err)
		}
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	for {
		chunk, err := stream.Next(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				_, _ = io.WriteString(w, "data: [DONE]\n\n")
				flusher.Flush()
				return
			}
			if errors.Is(err, context.Canceled) {
				return
			}
			if errors.Is(err, context.DeadlineExceeded) {
				s.providerHealth.MarkFailure(providerName)
				s.observeProviderHealth(providerName)
				return
			}
			if canFallbackProviderError(err) {
				s.providerHealth.MarkFailure(providerName)
				s.observeProviderHealth(providerName)
			}
			s.logger.Error("stream chat completion failed", "error", err)
			return
		}
		chunk.Model = externalModel
		s.observeUsage(routes.ChatCompletionsPath, externalModel, providerName, clientFromContext(r.Context()), chunk.Usage, pricing)
		if err := writeSSE(w, chunk); err != nil {
			s.logger.Debug("failed to write stream chunk", "error", err)
			return
		}
		flusher.Flush()
	}
}

func (s *Server) openChatCompletionStreamWithFallback(ctx context.Context, r *http.Request, route router.ModelRoute, externalModel string, req compat.ChatCompletionRequest) (provider.ChatCompletionStream, string, router.TokenPricing, error) {
	var lastErr error
	var skippedFrom string
	attempts := route.Attempts()
	for index, attempt := range attempts {
		if !s.providerHealth.Healthy(attempt.ProviderName) {
			s.observeProviderHealth(attempt.ProviderName)
			s.observeProviderCircuitOpen(r.Context(), routes.ChatCompletionsPath, externalModel, attempt.ProviderName)
			if skippedFrom == "" {
				skippedFrom = attempt.ProviderName
			}
			s.logger.Warn("stream chat completion provider circuit open before response; trying fallback", "provider", attempt.ProviderName)
			continue
		}
		if skippedFrom != "" {
			s.observeProviderFallback(r.Context(), routes.ChatCompletionsPath, externalModel, skippedFrom, attempt.ProviderName)
			skippedFrom = ""
		}
		attemptReq := req
		attemptReq.Model = attempt.UpstreamModel
		middleware.SetLogRoute(r.Context(), externalModel, attempt.ProviderName, attempt.UpstreamModel)
		stream, err := attempt.Provider.StreamChatCompletion(ctx, attemptReq)
		if err == nil {
			s.providerHealth.MarkSuccess(attempt.ProviderName)
			s.observeProviderHealth(attempt.ProviderName)
			return stream, attempt.ProviderName, attempt.Pricing, nil
		}
		lastErr = err
		if canFallbackProviderError(err) {
			s.providerHealth.MarkFailure(attempt.ProviderName)
			s.observeProviderHealth(attempt.ProviderName)
		}
		if index == len(attempts)-1 || !canFallbackProviderError(err) {
			return nil, "", router.TokenPricing{}, err
		}
		if nextProviderName := s.nextHealthyProviderName(attempts[index+1:]); nextProviderName != "" {
			s.observeProviderFallback(r.Context(), routes.ChatCompletionsPath, externalModel, attempt.ProviderName, nextProviderName)
		}
		s.logger.Warn("stream chat completion provider failed before response; trying fallback", "provider", attempt.ProviderName, "error", err)
	}
	if skippedFrom != "" {
		return nil, "", router.TokenPricing{}, providerUnavailableError()
	}
	return nil, "", router.TokenPricing{}, lastErr
}

func writeSSE(w io.Writer, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", payload)
	return err
}

func providerError(err error) *compat.Error {
	var compatErr *compat.Error
	if errors.As(err, &compatErr) {
		return compatErr
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return compat.ServerError(http.StatusGatewayTimeout, "provider timeout")
	}
	return compat.ServerError(http.StatusBadGateway, "provider error")
}

func providerUnavailableError() *compat.Error {
	return compat.ServerError(http.StatusServiceUnavailable, "provider unavailable")
}

func (s *Server) nextHealthyProviderName(attempts []router.ProviderRoute) string {
	for _, attempt := range attempts {
		if s.providerHealth.Healthy(attempt.ProviderName) {
			return attempt.ProviderName
		}
		s.observeProviderHealth(attempt.ProviderName)
	}
	return ""
}

func canFallbackProviderError(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var compatErr *compat.Error
	if errors.As(err, &compatErr) {
		return compatErr.Status == http.StatusTooManyRequests || compatErr.Status >= 500
	}
	return true
}

func decodeError(err error) *compat.Error {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return compat.RequestTooLarge("request body too large")
	}
	return compat.InvalidRequest("invalid JSON request body", "body")
}
