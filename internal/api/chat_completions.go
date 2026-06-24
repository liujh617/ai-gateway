package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/provider"
)

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	var req compat.ChatCompletionRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		s.WriteError(w, compat.InvalidRequest("invalid JSON request body", "body"))
		return
	}
	if err := req.Validate(); err != nil {
		s.WriteError(w, err)
		return
	}

	route, resolveErr := s.router.Resolve(req.Model)
	if resolveErr != nil {
		s.WriteError(w, resolveErr)
		return
	}

	externalModel := req.Model
	req.Model = route.UpstreamModel

	if req.Stream {
		s.streamChatCompletion(w, r, route.Provider, externalModel, req)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.requestTimeout)
	defer cancel()

	resp, err := route.Provider.CreateChatCompletion(ctx, req)
	if err != nil {
		s.WriteError(w, providerError(err))
		return
	}
	resp.Model = externalModel

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) streamChatCompletion(w http.ResponseWriter, r *http.Request, p provider.Provider, externalModel string, req compat.ChatCompletionRequest) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.WriteError(w, compat.ServerError(http.StatusInternalServerError, "streaming unsupported"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.streamTimeout)
	defer cancel()

	stream, err := p.StreamChatCompletion(ctx, req)
	if err != nil {
		s.WriteError(w, providerError(err))
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
