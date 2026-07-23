package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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
	var out compat.ChatCompletionResponse
	if err := p.doJSONRequest(ctx, "/chat/completions", req, &out); err != nil {
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
	var out compat.EmbeddingResponse
	if err := p.doJSONRequest(ctx, "/embeddings", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (p *Provider) CreateCompletion(ctx context.Context, req compat.CompletionsRequest) (*compat.CompletionsResponse, error) {
	req.Stream = false
	var out compat.CompletionsResponse
	if err := p.doJSONRequest(ctx, "/completions", req, &out); err != nil {
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
	var out compat.ImageGenerationResponse
	if err := p.doJSONRequest(ctx, "/images/generations", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (p *Provider) CreateModeration(ctx context.Context, req compat.ModerationRequest) (*compat.ModerationResponse, error) {
	var out compat.ModerationResponse
	if err := p.doJSONRequest(ctx, "/moderations", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (p *Provider) CreateTranscription(ctx context.Context, req compat.AudioTranscriptionRequest) (*compat.AudioTranscriptionResponse, error) {
	body, contentType, err := httpx.BuildAudioMultipartBody(req.File, req.Filename, req.Model, req.Language, req.Prompt, req.ResponseFormat, req.Temperature)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint("/audio/transcriptions"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	p.setHeaders(httpReq)
	httpReq.Header.Set("Content-Type", contentType)
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
	var out compat.AudioTranscriptionResponse
	if err := httpx.DecodeLimited(resp.Body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (p *Provider) CreateTranslation(ctx context.Context, req compat.AudioTranslationRequest) (*compat.AudioTranslationResponse, error) {
	body, contentType, err := httpx.BuildAudioMultipartBody(req.File, req.Filename, req.Model, "", req.Prompt, req.ResponseFormat, req.Temperature)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint("/audio/translations"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	p.setHeaders(httpReq)
	httpReq.Header.Set("Content-Type", contentType)
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
	var out compat.AudioTranslationResponse
	if err := httpx.DecodeLimited(resp.Body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (p *Provider) CreateSpeech(ctx context.Context, req compat.SpeechRequest) (*compat.SpeechResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint("/audio/speech"), bytes.NewReader(body))
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
	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "audio/mpeg"
	}
	return &compat.SpeechResponse{
		Data:        audioData,
		ContentType: contentType,
	}, nil
}

func (p *Provider) doJSONRequest(ctx context.Context, path string, reqBody, respBody any) error {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint(path), bytes.NewReader(body))
	if err != nil {
		return err
	}
	p.setJSONHeaders(httpReq)
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return httpx.TransportError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return httpx.UpstreamError(resp)
	}
	if err := httpx.RequireJSONResponse(resp); err != nil {
		return err
	}
	return httpx.DecodeLimited(resp.Body, respBody)
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
