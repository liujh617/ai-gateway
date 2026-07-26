package anthropic

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
	"open-ai-gateway/internal/upstreamurl"
)

const defaultBaseURL = "https://api.anthropic.com/v1"

type Provider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func New(baseURL, apiKey string, timeout time.Duration) (*Provider, error) {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	var err error
	baseURL, err = upstreamurl.NormalizeHTTPBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Provider{
		baseURL: baseURL,
		apiKey:  apiKey,
		client:  &http.Client{Timeout: timeout},
	}, nil
}

func (p *Provider) ListModels(ctx context.Context) ([]compat.Model, error) {
	return nil, nil
}

func (p *Provider) CreateChatCompletion(ctx context.Context, req compat.ChatCompletionRequest) (*compat.ChatCompletionResponse, error) {
	req.Stream = false
	ar := openaiToAnthropic(req)
	body, err := json.Marshal(ar)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	p.setHeaders(httpReq)
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, httpx.TransportError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, anthropicErrorToOpenAI(errBody, resp.StatusCode)
	}
	if err := httpx.RequireJSONResponse(resp); err != nil {
		return nil, err
	}
	var arResp MessageResponse
	if err := httpx.DecodeLimited(resp.Body, &arResp); err != nil {
		return nil, err
	}
	return anthropicToOpenAI(arResp, req.Model), nil
}

func (p *Provider) StreamChatCompletion(ctx context.Context, req compat.ChatCompletionRequest) (provider.ChatCompletionStream, error) {
	req.Stream = true
	ar := openaiToAnthropic(req)
	body, err := json.Marshal(ar)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	p.setHeaders(httpReq)
	httpReq.Header.Set("Accept", "text/event-stream")
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, httpx.TransportError(err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, anthropicErrorToOpenAI(errBody, resp.StatusCode)
	}
	return &chatCompletionStream{
		model:  req.Model,
		reader: resp.Body,
		stream: NewMessageStream(resp.Body),
	}, nil
}

func (p *Provider) setHeaders(req *http.Request) {
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")
}

// ProxyAnthropicRequest forwards a native Anthropic API request to the upstream
// and returns the response. This enables Claude Code and other Anthropic SDK clients.
func (p *Provider) ProxyAnthropicRequest(r *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body.Close()

	upstreamURL := p.baseURL + r.URL.Path
	if r.URL.RawQuery != "" {
		upstreamURL += "?" + r.URL.RawQuery
	}
	req, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")
	return p.client.Do(req)
}

// Stub methods
func (p *Provider) CreateEmbedding(ctx context.Context, req compat.EmbeddingRequest) (*compat.EmbeddingResponse, error) {
	return nil, notSupported
}
func (p *Provider) CreateImage(ctx context.Context, req compat.ImageGenerationRequest) (*compat.ImageGenerationResponse, error) {
	return nil, notSupported
}
func (p *Provider) CreateModeration(ctx context.Context, req compat.ModerationRequest) (*compat.ModerationResponse, error) {
	return nil, notSupported
}
func (p *Provider) CreateTranscription(ctx context.Context, req compat.AudioTranscriptionRequest) (*compat.AudioTranscriptionResponse, error) {
	return nil, notSupported
}
func (p *Provider) CreateTranslation(ctx context.Context, req compat.AudioTranslationRequest) (*compat.AudioTranslationResponse, error) {
	return nil, notSupported
}
func (p *Provider) CreateSpeech(ctx context.Context, req compat.SpeechRequest) (*compat.SpeechResponse, error) {
	return nil, notSupported
}
func (p *Provider) CreateCompletion(ctx context.Context, req compat.CompletionsRequest) (*compat.CompletionsResponse, error) {
	return nil, notSupported
}
func (p *Provider) StreamCompletion(ctx context.Context, req compat.CompletionsRequest) (provider.CompletionStream, error) {
	return nil, notSupported
}
func (p *Provider) UploadFile(ctx context.Context, req compat.FileUploadRequest) (*compat.FileObject, error) {
	return nil, notSupported
}
func (p *Provider) ListFiles(ctx context.Context) (*compat.FileList, error) { return nil, notSupported }
func (p *Provider) RetrieveFile(ctx context.Context, fileID string) (*compat.FileObject, error) {
	return nil, notSupported
}
func (p *Provider) DeleteFile(ctx context.Context, fileID string) (*compat.FileDeleteResponse, error) {
	return nil, notSupported
}
func (p *Provider) DownloadFile(ctx context.Context, fileID string) ([]byte, string, error) {
	return nil, "", notSupported
}
func (p *Provider) CreateBatch(ctx context.Context, req compat.BatchRequest) (*compat.Batch, error) {
	return nil, notSupported
}
func (p *Provider) ListBatches(ctx context.Context, after string, limit int) (*compat.BatchList, error) {
	return nil, notSupported
}
func (p *Provider) RetrieveBatch(ctx context.Context, batchID string) (*compat.Batch, error) {
	return nil, notSupported
}
func (p *Provider) CancelBatch(ctx context.Context, batchID string) (*compat.Batch, error) {
	return nil, notSupported
}
func (p *Provider) CreateFineTuningJob(ctx context.Context, req compat.FineTuningJobRequest) (*compat.FineTuningJob, error) {
	return nil, notSupported
}
func (p *Provider) ListFineTuningJobs(ctx context.Context, after string, limit int) (*compat.FineTuningJobList, error) {
	return nil, notSupported
}
func (p *Provider) RetrieveFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) {
	return nil, notSupported
}
func (p *Provider) ListFineTuningJobEvents(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobEventList, error) {
	return nil, notSupported
}
func (p *Provider) CancelFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) {
	return nil, notSupported
}
func (p *Provider) ListFineTuningJobCheckpoints(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobCheckpointList, error) {
	return nil, notSupported
}

var notSupported = provider.ErrUnsupported{Provider: "anthropic"}

type chatCompletionStream struct {
	model  string
	reader io.ReadCloser
	stream *MessageStream
}

func (s *chatCompletionStream) Next(ctx context.Context) (*compat.ChatCompletionChunk, error) {
	event, err := s.stream.Next()
	if err != nil {
		return nil, err
	}
	return convertStreamEvent(event, s.model), nil
}

func (s *chatCompletionStream) Close() error {
	return s.reader.Close()
}

func convertStreamEvent(event *MessageStreamEvent, model string) *compat.ChatCompletionChunk {
	switch event.Type {
	case "message_start":
		return &compat.ChatCompletionChunk{
			ID:      event.Message.ID,
			Object:  "chat.completion.chunk",
			Created: 0,
			Model:   model,
			Choices: []compat.ChatCompletionChunkChoice{{
				Index: 0,
				Delta: compat.ChatMessageDelta{
					Role: "assistant",
				},
			}},
		}
	case "content_block_start":
		if event.ContentBlock.Type == "text" {
			return &compat.ChatCompletionChunk{
				ID:      "",
				Object:  "chat.completion.chunk",
				Created: 0,
				Model:   model,
				Choices: []compat.ChatCompletionChunkChoice{{
					Index: 0,
					Delta: compat.ChatMessageDelta{
						Content: "",
					},
				}},
			}
		}
		if event.ContentBlock.Type == "tool_use" {
			return &compat.ChatCompletionChunk{
				ID:      "",
				Object:  "chat.completion.chunk",
				Created: 0,
				Model:   model,
				Choices: []compat.ChatCompletionChunkChoice{{
					Index: 0,
					Delta: compat.ChatMessageDelta{
						Extra: map[string]json.RawMessage{
							"tool_calls": toToolCallDelta(event.ContentBlock),
						},
					},
				}},
			}
		}
	case "content_block_delta":
		if event.Delta.Type == "text_delta" {
			return &compat.ChatCompletionChunk{
				ID:      "",
				Object:  "chat.completion.chunk",
				Created: 0,
				Model:   model,
				Choices: []compat.ChatCompletionChunkChoice{{
					Index: 0,
					Delta: compat.ChatMessageDelta{
						Content: event.Delta.Text,
					},
				}},
			}
		}
		if event.Delta.Type == "input_json_delta" {
			return &compat.ChatCompletionChunk{
				ID:      "",
				Object:  "chat.completion.chunk",
				Created: 0,
				Model:   model,
				Choices: []compat.ChatCompletionChunkChoice{{
					Index: 0,
					Delta: compat.ChatMessageDelta{
						Extra: map[string]json.RawMessage{
							"tool_calls": toArgsDelta(event.Index, event.Delta.PartialJSON),
						},
					},
				}},
			}
		}
	case "message_delta":
		if event.Usage != nil {
			return &compat.ChatCompletionChunk{
				ID:      "",
				Object:  "chat.completion.chunk",
				Created: 0,
				Model:   model,
				Choices: []compat.ChatCompletionChunkChoice{{
					Index:        0,
					FinishReason: strPtr(anthropicFinishReason(event.Delta.StopReason)),
				}},
				Usage: &compat.Usage{
					PromptTokens:     event.Usage.InputTokens,
					CompletionTokens: event.Usage.OutputTokens,
					TotalTokens:      event.Usage.InputTokens + event.Usage.OutputTokens,
				},
			}
		}
	}
	return &compat.ChatCompletionChunk{
		ID:      "",
		Object:  "chat.completion.chunk",
		Created: 0,
		Model:   model,
		Choices: []compat.ChatCompletionChunkChoice{{Index: 0}},
	}
}

func toToolCallDelta(block *ContentBlock) json.RawMessage {
	calls := []map[string]any{{
		"index": 0,
		"id":    block.ID,
		"type":  "function",
		"function": map[string]string{
			"name":      block.Name,
			"arguments": "",
		},
	}}
	data, _ := json.Marshal(calls)
	return data
}

func toArgsDelta(index int, partial string) json.RawMessage {
	calls := []map[string]any{{
		"index": index,
		"function": map[string]string{
			"arguments": partial,
		},
	}}
	data, _ := json.Marshal(calls)
	return data
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
