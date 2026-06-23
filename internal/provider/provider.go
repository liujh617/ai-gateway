package provider

import (
	"context"
	"io"

	"open-ai-gateway/internal/compat"
)

type Provider interface {
	ListModels(ctx context.Context) ([]compat.Model, error)
	CreateChatCompletion(ctx context.Context, req compat.ChatCompletionRequest) (*compat.ChatCompletionResponse, error)
	StreamChatCompletion(ctx context.Context, req compat.ChatCompletionRequest) (ChatCompletionStream, error)
}

type ChatCompletionStream interface {
	Next(ctx context.Context) (*compat.ChatCompletionChunk, error)
	Close() error
}

var ErrStreamClosed = io.ErrClosedPipe
