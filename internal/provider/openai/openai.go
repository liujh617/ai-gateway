package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/provider"
	"open-ai-gateway/internal/requestctx"
	"open-ai-gateway/internal/version"
)

const maxResponseBodyBytes = 10 << 20

type Provider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func New(baseURL, apiKey string, timeout time.Duration) (*Provider, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, errors.New("base_url is required")
	}
	if _, err := url.ParseRequestURI(baseURL); err != nil {
		return nil, fmt.Errorf("invalid base_url: %w", err)
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
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, upstreamError(resp)
	}
	if err := requireJSONResponse(resp); err != nil {
		return nil, err
	}

	var out compat.ModelListResponse
	if err := decodeLimited(resp.Body, &out); err != nil {
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
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, upstreamError(resp)
	}
	if err := requireJSONResponse(resp); err != nil {
		return nil, err
	}

	var out compat.ChatCompletionResponse
	if err := decodeLimited(resp.Body, &out); err != nil {
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
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, upstreamError(resp)
	}

	return &stream{
		body:   resp.Body,
		reader: bufio.NewReader(resp.Body),
	}, nil
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
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, upstreamError(resp)
	}
	if err := requireJSONResponse(resp); err != nil {
		return nil, err
	}

	var out compat.EmbeddingResponse
	if err := decodeLimited(resp.Body, &out); err != nil {
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

type stream struct {
	body   io.ReadCloser
	reader *bufio.Reader
}

func (s *stream) Next(ctx context.Context) (*compat.ChatCompletionChunk, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		payload, err := s.nextPayload()
		if err != nil {
			return nil, err
		}
		if payload == "" {
			continue
		}
		if payload == "[DONE]" {
			return nil, io.EOF
		}

		var chunk compat.ChatCompletionChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return nil, err
		}
		return &chunk, nil
	}
}

func (s *stream) nextPayload() (string, error) {
	var data []string
	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				if len(data) == 0 {
					return "", io.EOF
				}
				return strings.Join(data, "\n"), nil
			}
			return "", err
		}

		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if len(data) == 0 {
				continue
			}
			return strings.Join(data, "\n"), nil
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.HasPrefix(value, " ") {
			value = strings.TrimPrefix(value, " ")
		}
		switch field {
		case "data":
			data = append(data, value)
		case "event", "id", "retry":
			continue
		default:
			continue
		}
	}
}

func (s *stream) Close() error {
	return s.body.Close()
}

func decodeLimited(r io.Reader, out any) error {
	limited := &io.LimitedReader{R: r, N: maxResponseBodyBytes + 1}
	decoder := json.NewDecoder(limited)
	if err := decoder.Decode(out); err != nil {
		return err
	}
	if limited.N <= 0 {
		return fmt.Errorf("upstream response body exceeds %d bytes", maxResponseBodyBytes)
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("upstream response body must contain a single JSON value")
		}
		return err
	}
	if limited.N <= 0 {
		return fmt.Errorf("upstream response body exceeds %d bytes", maxResponseBodyBytes)
	}
	return nil
}

func requireJSONResponse(resp *http.Response) error {
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || !strings.EqualFold(mediaType, "application/json") {
		return fmt.Errorf("upstream response Content-Type must be application/json")
	}
	return nil
}

func upstreamError(resp *http.Response) error {
	var upstream compat.ErrorResponse
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	if len(body) > 0 {
		_ = json.Unmarshal(body, &upstream)
	}

	message := http.StatusText(resp.StatusCode)
	if upstream.Error.Message != "" {
		message = upstream.Error.Message
	}

	errorType := upstream.Error.Type
	if errorType == "" {
		errorType = defaultErrorType(resp.StatusCode)
	}
	status := resp.StatusCode
	if resp.StatusCode >= 500 && resp.StatusCode != http.StatusGatewayTimeout {
		status = http.StatusBadGateway
	}
	return &compat.Error{
		Status:  status,
		Message: message,
		Type:    errorType,
		Param:   upstream.Error.Param,
		Code:    upstream.Error.Code,
	}
}

func defaultErrorType(status int) string {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return "authentication_error"
	case http.StatusTooManyRequests:
		return "rate_limit_error"
	case http.StatusBadRequest, http.StatusNotFound:
		return "invalid_request_error"
	default:
		return "server_error"
	}
}
