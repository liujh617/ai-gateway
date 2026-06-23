package fake

import (
	"context"
	"encoding/json"
	"io"
	"sync/atomic"
	"time"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/provider"
)

type Provider struct {
	ResponseText string
	Err          error
	StreamErr    error
	StreamParts  []string
	Closed       atomic.Bool
}

func New() *Provider {
	return &Provider{
		ResponseText: "Hello from open-ai-gateway.",
		StreamParts:  []string{"Hello", " from", " open-ai-gateway."},
	}
}

func (p *Provider) ListModels(ctx context.Context) ([]compat.Model, error) {
	return []compat.Model{{
		ID:      "test-model",
		Object:  "model",
		Created: 0,
		OwnedBy: "fake",
	}}, nil
}

func (p *Provider) CreateChatCompletion(ctx context.Context, req compat.ChatCompletionRequest) (*compat.ChatCompletionResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.Err != nil {
		return nil, p.Err
	}
	content, _ := json.Marshal(p.ResponseText)
	return &compat.ChatCompletionResponse{
		ID:      "chatcmpl_fake",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []compat.ChatCompletionChoice{{
			Index: 0,
			Message: compat.ChatMessage{
				Role:    "assistant",
				Content: content,
			},
			FinishReason: "stop",
		}},
		Usage: &compat.Usage{
			PromptTokens:     1,
			CompletionTokens: 1,
			TotalTokens:      2,
		},
	}, nil
}

func (p *Provider) StreamChatCompletion(ctx context.Context, req compat.ChatCompletionRequest) (provider.ChatCompletionStream, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.StreamErr != nil {
		return nil, p.StreamErr
	}
	parts := append([]string(nil), p.StreamParts...)
	return &stream{provider: p, model: req.Model, parts: parts}, nil
}

type stream struct {
	provider *Provider
	model    string
	parts    []string
	index    int
}

func (s *stream) Next(ctx context.Context) (*compat.ChatCompletionChunk, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.index >= len(s.parts) {
		return nil, io.EOF
	}
	part := s.parts[s.index]
	s.index++
	return &compat.ChatCompletionChunk{
		ID:      "chatcmpl_fake",
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   s.model,
		Choices: []compat.ChatCompletionChunkChoice{{
			Index: 0,
			Delta: compat.ChatMessageDelta{
				Content: part,
			},
			FinishReason: nil,
		}},
	}, nil
}

func (s *stream) Close() error {
	s.provider.Closed.Store(true)
	return nil
}
