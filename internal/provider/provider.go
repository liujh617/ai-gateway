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
	CreateImage(ctx context.Context, req compat.ImageGenerationRequest) (*compat.ImageGenerationResponse, error)
	CreateModeration(ctx context.Context, req compat.ModerationRequest) (*compat.ModerationResponse, error)
	CreateTranscription(ctx context.Context, req compat.AudioTranscriptionRequest) (*compat.AudioTranscriptionResponse, error)
	CreateTranslation(ctx context.Context, req compat.AudioTranslationRequest) (*compat.AudioTranslationResponse, error)
	CreateSpeech(ctx context.Context, req compat.SpeechRequest) (*compat.SpeechResponse, error)
	CreateBatch(ctx context.Context, req compat.BatchRequest) (*compat.Batch, error)
	ListBatches(ctx context.Context, after string, limit int) (*compat.BatchList, error)
	RetrieveBatch(ctx context.Context, batchID string) (*compat.Batch, error)
	CancelBatch(ctx context.Context, batchID string) (*compat.Batch, error)
	UploadFile(ctx context.Context, req compat.FileUploadRequest) (*compat.FileObject, error)
	ListFiles(ctx context.Context) (*compat.FileList, error)
	RetrieveFile(ctx context.Context, fileID string) (*compat.FileObject, error)
	DeleteFile(ctx context.Context, fileID string) (*compat.FileDeleteResponse, error)
	DownloadFile(ctx context.Context, fileID string) ([]byte, string, error)
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
