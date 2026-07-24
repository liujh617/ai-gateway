package api

import (
	"context"
	"net/http"

	"open-ai-gateway/internal/middleware"
	"open-ai-gateway/internal/provider"
	"open-ai-gateway/internal/router"
)

// callProvider is a function that calls a provider method with the given request.
type callProvider[Req any, Resp any] func(ctx context.Context, p provider.Provider, req Req) (Resp, error)

// prepareRequest copies a request and sets the upstream model name.
type prepareRequest[Req any] func(req Req, upstreamModel string) Req

// fallbackAttempt holds metadata about a provider attempt for onSuccess callbacks.
type fallbackAttempt struct {
	ProviderName  string
	UpstreamModel string
	Pricing       router.TokenPricing
}

// executeWithFallback runs the provider fallback loop for non-streaming requests.
func executeWithFallback[Req any, Resp any](s *Server,
	ctx context.Context,
	r *http.Request,
	path string,
	externalModel string,
	route router.ModelRoute,
	req Req,
	call callProvider[Req, Resp],
	prepare prepareRequest[Req],
	onSuccess func(Resp, fallbackAttempt),
) (Resp, string, string, error) {
	var zero Resp
	var lastErr error
	var skippedFrom string
	attempts := route.Attempts()
	for index, attempt := range attempts {
		if !s.providerHealth.Healthy(attempt.ProviderName) {
			s.observeProviderHealth(attempt.ProviderName)
			s.observeProviderCircuitOpen(r.Context(), path, externalModel, attempt.ProviderName)
			if skippedFrom == "" {
				skippedFrom = attempt.ProviderName
			}
			continue
		}
		if skippedFrom != "" {
			s.observeProviderFallback(r.Context(), path, externalModel, skippedFrom, attempt.ProviderName)
			skippedFrom = ""
		}
		attemptReq := prepare(req, attempt.UpstreamModel)
		middleware.SetLogRoute(r.Context(), externalModel, attempt.ProviderName, attempt.UpstreamModel)
		resp, err := call(ctx, attempt.Provider, attemptReq)
		if err == nil {
			s.providerHealth.MarkSuccess(attempt.ProviderName)
			s.observeProviderHealth(attempt.ProviderName)
			fa := fallbackAttempt{ProviderName: attempt.ProviderName, UpstreamModel: attempt.UpstreamModel, Pricing: attempt.Pricing}
			if onSuccess != nil {
				onSuccess(resp, fa)
			}
			return resp, attempt.ProviderName, attempt.UpstreamModel, nil
		}
		lastErr = err
		if canFallbackProviderError(err) {
			s.providerHealth.MarkFailure(attempt.ProviderName)
			s.observeProviderHealth(attempt.ProviderName)
		}
		if index == len(attempts)-1 || !canFallbackProviderError(err) {
			return zero, "", "", err
		}
		if nextProviderName := s.nextHealthyProviderName(attempts[index+1:]); nextProviderName != "" {
			s.observeProviderFallback(r.Context(), path, externalModel, attempt.ProviderName, nextProviderName)
		}
	}
	if skippedFrom != "" {
		return zero, "", "", providerUnavailableError()
	}
	return zero, "", "", lastErr
}

// openStream is a function that opens a provider stream.
type openStream[Req any, Stream any] func(ctx context.Context, p provider.Provider, req Req) (Stream, error)

// executeStreamingFallback runs the provider fallback loop for streaming requests.
func executeStreamingFallback[Req any, Stream any](s *Server,
	ctx context.Context,
	r *http.Request,
	path string,
	externalModel string,
	route router.ModelRoute,
	req Req,
	open openStream[Req, Stream],
	prepare prepareRequest[Req],
) (Stream, string, string, router.TokenPricing, error) {
	var zeroStream Stream
	var lastErr error
	var skippedFrom string
	attempts := route.Attempts()
	for index, attempt := range attempts {
		if !s.providerHealth.Healthy(attempt.ProviderName) {
			s.observeProviderHealth(attempt.ProviderName)
			s.observeProviderCircuitOpen(r.Context(), path, externalModel, attempt.ProviderName)
			if skippedFrom == "" {
				skippedFrom = attempt.ProviderName
			}
			continue
		}
		if skippedFrom != "" {
			s.observeProviderFallback(r.Context(), path, externalModel, skippedFrom, attempt.ProviderName)
			skippedFrom = ""
		}
		attemptReq := prepare(req, attempt.UpstreamModel)
		middleware.SetLogRoute(r.Context(), externalModel, attempt.ProviderName, attempt.UpstreamModel)
		stream, err := open(ctx, attempt.Provider, attemptReq)
		if err == nil {
			s.providerHealth.MarkSuccess(attempt.ProviderName)
			s.observeProviderHealth(attempt.ProviderName)
			return stream, attempt.ProviderName, attempt.UpstreamModel, attempt.Pricing, nil
		}
		lastErr = err
		if canFallbackProviderError(err) {
			s.providerHealth.MarkFailure(attempt.ProviderName)
			s.observeProviderHealth(attempt.ProviderName)
		}
		if index == len(attempts)-1 || !canFallbackProviderError(err) {
			return zeroStream, "", "", router.TokenPricing{}, err
		}
		if nextProviderName := s.nextHealthyProviderName(attempts[index+1:]); nextProviderName != "" {
			s.observeProviderFallback(r.Context(), path, externalModel, attempt.ProviderName, nextProviderName)
		}
	}
	if skippedFrom != "" {
		return zeroStream, "", "", router.TokenPricing{}, providerUnavailableError()
	}
	return zeroStream, "", "", router.TokenPricing{}, lastErr
}
