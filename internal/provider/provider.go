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
	CreateCompletion(ctx context.Context, req compat.CompletionsRequest) (*compat.CompletionsResponse, error)
	StreamCompletion(ctx context.Context, req compat.CompletionsRequest) (CompletionStream, error)
	CreateEmbedding(ctx context.Context, req compat.EmbeddingRequest) (*compat.EmbeddingResponse, error)
}

type ChatCompletionStream interface {
	Next(ctx context.Context) (*compat.ChatCompletionChunk, error)
	Close() error
}

type CompletionStream interface {
	Next(ctx context.Context) (*compat.CompletionsChunk, error)
	Close() error
}

var ErrStreamClosed = io.ErrClosedPipe
