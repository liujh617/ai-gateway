package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/provider"
	"open-ai-gateway/internal/provider/httpx"
	"open-ai-gateway/internal/requestctx"
	"open-ai-gateway/internal/upstreamurl"
	"open-ai-gateway/internal/version"
)

type Provider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func New(baseURL, apiKey string, timeout time.Duration) (*Provider, error) {
	baseURL, err := upstreamurl.NormalizeHTTPBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Provider{
		baseURL: baseURL,
		apiKey:  apiKey,
		client: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

func (p *Provider) ListModels(ctx context.Context) ([]compat.Model, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.endpoint("/models"), nil)
	if err != nil {
		return nil, err
	}
	p.setHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, httpx.TransportError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, httpx.UpstreamError(resp)
	}
	if err := httpx.RequireJSONResponse(resp); err != nil {
		return nil, err
	}

	var out compat.ModelListResponse
	if err := httpx.DecodeLimited(resp.Body, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

func (p *Provider) CreateChatCompletion(ctx context.Context, req compat.ChatCompletionRequest) (*compat.ChatCompletionResponse, error) {
	req.Stream = false
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint("/chat/completions"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	p.setJSONHeaders(httpReq)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, httpx.TransportError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, httpx.UpstreamError(resp)
	}
	if err := httpx.RequireJSONResponse(resp); err != nil {
		return nil, err
	}

	var out compat.ChatCompletionResponse
	if err := httpx.DecodeLimited(resp.Body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (p *Provider) StreamChatCompletion(ctx context.Context, req compat.ChatCompletionRequest) (provider.ChatCompletionStream, error) {
	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint("/chat/completions"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	p.setJSONHeaders(httpReq)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, httpx.TransportError(err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, httpx.UpstreamError(resp)
	}
	if err := httpx.RequireEventStreamResponse(resp); err != nil {
		resp.Body.Close()
		return nil, err
	}

	return httpx.NewChatCompletionStream(resp.Body), nil
}

func (p *Provider) CreateEmbedding(ctx context.Context, req compat.EmbeddingRequest) (*compat.EmbeddingResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint("/embeddings"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	p.setJSONHeaders(httpReq)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, httpx.TransportError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, httpx.UpstreamError(resp)
	}
	if err := httpx.RequireJSONResponse(resp); err != nil {
		return nil, err
	}

	var out compat.EmbeddingResponse
	if err := httpx.DecodeLimited(resp.Body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (p *Provider) CreateCompletion(ctx context.Context, req compat.CompletionsRequest) (*compat.CompletionsResponse, error) {
	req.Stream = false
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint("/completions"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	p.setJSONHeaders(httpReq)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, httpx.TransportError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, httpx.UpstreamError(resp)
	}
	if err := httpx.RequireJSONResponse(resp); err != nil {
		return nil, err
	}

	var out compat.CompletionsResponse
	if err := httpx.DecodeLimited(resp.Body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (p *Provider) StreamCompletion(ctx context.Context, req compat.CompletionsRequest) (provider.CompletionStream, error) {
	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint("/completions"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	p.setJSONHeaders(httpReq)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, httpx.TransportError(err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, httpx.UpstreamError(resp)
	}
	if err := httpx.RequireEventStreamResponse(resp); err != nil {
		resp.Body.Close()
		return nil, err
	}

	return httpx.NewCompletionStream(resp.Body), nil
}

func (p *Provider) CreateImage(ctx context.Context, req compat.ImageGenerationRequest) (*compat.ImageGenerationResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint("/images/generations"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	p.setJSONHeaders(httpReq)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, httpx.TransportError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, httpx.UpstreamError(resp)
	}
	if err := httpx.RequireJSONResponse(resp); err != nil {
		return nil, err
	}

	var out compat.ImageGenerationResponse
	if err := httpx.DecodeLimited(resp.Body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (p *Provider) CreateModeration(ctx context.Context, req compat.ModerationRequest) (*compat.ModerationResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint("/moderations"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	p.setJSONHeaders(httpReq)
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, httpx.TransportError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, httpx.UpstreamError(resp)
	}
	if err := httpx.RequireJSONResponse(resp); err != nil {
		return nil, err
	}
	var out compat.ModerationResponse
	if err := httpx.DecodeLimited(resp.Body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (p *Provider) endpoint(path string) string {
	return p.baseURL + path
}

func (p *Provider) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", version.UserAgent())
	if requestID := requestctx.RequestID(req.Context()); requestID != "" {
		req.Header.Set(requestctx.RequestIDHeader, requestID)
	}
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
}

func (p *Provider) setJSONHeaders(req *http.Request) {
	p.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")
}
