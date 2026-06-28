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
	attempts := route.Attempts()
	for index, attempt := range attempts {
		attemptReq := req
		attemptReq.Model = attempt.UpstreamModel
		middleware.SetLogRoute(r.Context(), externalModel, attempt.ProviderName, attempt.UpstreamModel)
		resp, err := attempt.Provider.CreateChatCompletion(ctx, attemptReq)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if index == len(attempts)-1 || !canFallbackProviderError(err) {
			return nil, err
		}
		s.logger.Warn("chat completion provider failed; trying fallback", "provider", attempt.ProviderName, "error", err)
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

	stream, err := s.openChatCompletionStreamWithFallback(ctx, r, route, externalModel, req)
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
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return
			}
			s.logger.Error("stream chat completion failed", "error", err)
			return
		}
		chunk.Model = externalModel
		if err := writeSSE(w, chunk); err != nil {
			s.logger.Debug("failed to write stream chunk", "error", err)
			return
		}
		flusher.Flush()
	}
}

func (s *Server) openChatCompletionStreamWithFallback(ctx context.Context, r *http.Request, route router.ModelRoute, externalModel string, req compat.ChatCompletionRequest) (provider.ChatCompletionStream, error) {
	var lastErr error
	attempts := route.Attempts()
	for index, attempt := range attempts {
		attemptReq := req
		attemptReq.Model = attempt.UpstreamModel
		middleware.SetLogRoute(r.Context(), externalModel, attempt.ProviderName, attempt.UpstreamModel)
		stream, err := attempt.Provider.StreamChatCompletion(ctx, attemptReq)
		if err == nil {
			return stream, nil
		}
		lastErr = err
		if index == len(attempts)-1 || !canFallbackProviderError(err) {
			return nil, err
		}
		s.logger.Warn("stream chat completion provider failed before response; trying fallback", "provider", attempt.ProviderName, "error", err)
	}
	return nil, lastErr
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
