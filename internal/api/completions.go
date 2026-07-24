package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"open-ai-gateway/internal/audit"
	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/middleware"
	"open-ai-gateway/internal/provider"
	"open-ai-gateway/internal/router"
	"open-ai-gateway/internal/routes"
)

func (s *Server) handleCompletions(w http.ResponseWriter, r *http.Request) {
	if err := requireJSONContentType(r); err != nil {
		s.writeError(w, r, err)
		return
	}
	var req compat.CompletionsRequest
	if err := decodeJSONBody(s.requestBody(w, r), &req); err != nil {
		s.writeError(w, r, decodeError(err))
		return
	}
	middleware.SetLogStream(r.Context(), req.Stream)
	if validationErr := req.Validate(); validationErr != nil {
		s.writeAuditedError(w, r, routes.CompletionsPath, req.Model, validationErr)
		return
	}
	if !s.modelAllowedForRequest(r, req.Model) {
		middleware.SetLogRoute(r.Context(), req.Model, "", "")
		s.writeAuditedError(w, r, routes.CompletionsPath, req.Model, compat.ModelNotFound(req.Model))
		return
	}
	route, resolveErr := s.router.ResolveFor(req.Model, "completions")
	if resolveErr != nil {
		middleware.SetLogRoute(r.Context(), req.Model, "", "")
		s.writeAuditedError(w, r, routes.CompletionsPath, req.Model, resolveErr)
		return
	}
	externalModel := req.Model
	requestEvent := s.auditBaseEvent(r, audit.EventRequest, routes.CompletionsPath, externalModel)
	requestEvent.Body = rawBody(req)
	s.audit.Record(r.Context(), requestEvent)
	if req.Stream {
		s.streamCompletion(w, r, route, externalModel, req)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.requestTimeout)
	defer cancel()
	resp, providerName, upstreamModel, err := s.createCompletionWithFallback(ctx, r, route, externalModel, req)
	if err != nil {
		s.writeAuditedError(w, r, routes.CompletionsPath, externalModel, providerError(err))
		return
	}
	resp.Model = externalModel
	responseEvent := s.auditBaseEvent(r, audit.EventResponse, routes.CompletionsPath, externalModel)
	responseEvent.Provider = providerName
	responseEvent.UpstreamModel = upstreamModel
	responseEvent.Status = http.StatusOK
	responseEvent.Body = rawBody(resp)
	s.audit.Record(r.Context(), responseEvent)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) createCompletionWithFallback(ctx context.Context, r *http.Request, route router.ModelRoute, externalModel string, req compat.CompletionsRequest) (*compat.CompletionsResponse, string, string, error) {
	return executeWithFallback(s, ctx, r, routes.CompletionsPath, externalModel, route, req,
		func(ctx context.Context, p provider.Provider, req compat.CompletionsRequest) (*compat.CompletionsResponse, error) {
			return p.CreateCompletion(ctx, req)
		},
		func(req compat.CompletionsRequest, upstreamModel string) compat.CompletionsRequest {
			req.Model = upstreamModel
			return req
		},
		func(resp *compat.CompletionsResponse, fa fallbackAttempt) {
			s.observeUsage(routes.CompletionsPath, externalModel, fa.ProviderName, clientFromContext(r.Context()), resp.Usage, fa.Pricing)
		},
	)
}

func (s *Server) streamCompletion(w http.ResponseWriter, r *http.Request, route router.ModelRoute, externalModel string, req compat.CompletionsRequest) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, r, compat.ServerError(http.StatusInternalServerError, "streaming unsupported"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.streamTimeout)
	defer cancel()
	stream, providerName, upstreamModel, pricing, err := s.openCompletionStreamWithFallback(ctx, r, route, externalModel, req)
	if err != nil {
		s.writeAuditedError(w, r, routes.CompletionsPath, externalModel, providerError(err))
		return
	}
	defer stream.Close()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	for {
		chunk, nextErr := stream.Next(ctx)
		if nextErr != nil {
			if errors.Is(nextErr, io.EOF) {
				_, _ = io.WriteString(w, "data: [DONE]\n\n")
				flusher.Flush()
				doneEvent := s.auditBaseEvent(r, audit.EventStreamDone, routes.CompletionsPath, externalModel)
				doneEvent.Provider, doneEvent.UpstreamModel, doneEvent.Status = providerName, upstreamModel, http.StatusOK
				s.audit.Record(r.Context(), doneEvent)
				return
			}
			if errors.Is(nextErr, context.Canceled) {
				errorEvent := s.auditBaseEvent(r, audit.EventError, routes.CompletionsPath, externalModel)
				errorEvent.Provider, errorEvent.UpstreamModel = providerName, upstreamModel
				errorEvent.Error = "context_canceled"
				s.audit.Record(r.Context(), errorEvent)
				return
			}
			if errors.Is(nextErr, context.DeadlineExceeded) {
				s.providerHealth.MarkFailure(providerName)
				s.observeProviderHealth(providerName)
				errorEvent := s.auditBaseEvent(r, audit.EventError, routes.CompletionsPath, externalModel)
				errorEvent.Provider, errorEvent.UpstreamModel = providerName, upstreamModel
				errorEvent.Error = "context_deadline_exceeded"
				s.audit.Record(r.Context(), errorEvent)
				return
			}
			if canFallbackProviderError(nextErr) {
				s.providerHealth.MarkFailure(providerName)
				s.observeProviderHealth(providerName)
			}
			s.logger.Error("stream completion failed", "error", nextErr)
			errorEvent := s.auditBaseEvent(r, audit.EventError, routes.CompletionsPath, externalModel)
			errorEvent.Provider, errorEvent.UpstreamModel = providerName, upstreamModel
			errorEvent.Error = "stream_error"
			s.audit.Record(r.Context(), errorEvent)
			return
		}
		chunk.Model = externalModel
		s.observeUsage(routes.CompletionsPath, externalModel, providerName, clientFromContext(r.Context()), chunk.Usage, pricing)
		chunkEvent := s.auditBaseEvent(r, audit.EventStreamChunk, routes.CompletionsPath, externalModel)
		chunkEvent.Provider, chunkEvent.UpstreamModel = providerName, upstreamModel
		chunkEvent.Status = http.StatusOK
		chunkEvent.Body = rawBody(chunk)
		s.audit.Record(r.Context(), chunkEvent)
		if err := writeSSE(w, chunk); err != nil {
			s.logger.Debug("failed to write stream chunk", "error", err)
			return
		}
		flusher.Flush()
	}
}

func (s *Server) openCompletionStreamWithFallback(ctx context.Context, r *http.Request, route router.ModelRoute, externalModel string, req compat.CompletionsRequest) (provider.CompletionStream, string, string, router.TokenPricing, error) {
	return executeStreamingFallback(s, ctx, r, routes.CompletionsPath, externalModel, route, req,
		func(ctx context.Context, p provider.Provider, req compat.CompletionsRequest) (provider.CompletionStream, error) {
			return p.StreamCompletion(ctx, req)
		},
		func(req compat.CompletionsRequest, upstreamModel string) compat.CompletionsRequest {
			req.Model = upstreamModel
			return req
		},
	)
}
