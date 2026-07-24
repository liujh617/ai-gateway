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
	var extra map[string]json.RawMessage
	if result, ok := fakeToolResult(req); ok {
		content, _ = json.Marshal("Tool result received: " + result)
	} else if len(req.Extra["tools"]) > 0 {
		content = json.RawMessage("null")
		calls, _ := json.Marshal([]any{map[string]any{"id": "call_fake_weather", "type": "function", "function": map[string]string{"name": "get_weather", "arguments": "{\"location\":\"Paris\"}"}}})
		extra = map[string]json.RawMessage{"tool_calls": calls}
	}
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
				Extra:   extra,
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

func (p *Provider) CreateImage(ctx context.Context, req compat.ImageGenerationRequest) (*compat.ImageGenerationResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.Err != nil {
		return nil, p.Err
	}
	return &compat.ImageGenerationResponse{
		Created: time.Now().Unix(),
		Data: []compat.ImageGenerationData{{
			URL: "https://example.com/fake-image.png",
		}},
	}, nil
}

func (p *Provider) CreateModeration(ctx context.Context, req compat.ModerationRequest) (*compat.ModerationResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.Err != nil {
		return nil, p.Err
	}
	return &compat.ModerationResponse{
		ID:    "modr_fake",
		Model: req.Model,
		Results: []compat.ModerationResult{{
			Categories:     map[string]bool{"hate": false, "self-harm": false, "sexual": false, "violence": false},
			CategoryScores: map[string]float64{"hate": 0.01, "self-harm": 0.01, "sexual": 0.01, "violence": 0.01},
			Flagged:        false,
		}},
	}, nil
}

func (p *Provider) CreateTranscription(ctx context.Context, req compat.AudioTranscriptionRequest) (*compat.AudioTranscriptionResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.Err != nil {
		return nil, p.Err
	}
	return &compat.AudioTranscriptionResponse{
		Text: p.ResponseText,
	}, nil
}

func (p *Provider) CreateTranslation(ctx context.Context, req compat.AudioTranslationRequest) (*compat.AudioTranslationResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.Err != nil {
		return nil, p.Err
	}
	return &compat.AudioTranslationResponse{
		Text: p.ResponseText,
	}, nil
}

func (p *Provider) CreateSpeech(ctx context.Context, req compat.SpeechRequest) (*compat.SpeechResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.Err != nil {
		return nil, p.Err
	}
	return &compat.SpeechResponse{
		Data:        []byte("fake-audio-data"),
		ContentType: "audio/mpeg",
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
	return &chatStream{provider: p, model: req.Model, parts: parts, toolMode: len(req.Extra["tools"]) > 0}, nil
}

func (p *Provider) CreateCompletion(ctx context.Context, req compat.CompletionsRequest) (*compat.CompletionsResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.Err != nil {
		return nil, p.Err
	}
	return &compat.CompletionsResponse{
		ID:      "cmpl_fake",
		Object:  "text_completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []compat.CompletionsChoice{{
			Text:         p.ResponseText,
			Index:        0,
			Logprobs:     nil,
			FinishReason: "stop",
		}},
		Usage: &compat.Usage{
			PromptTokens:     1,
			CompletionTokens: 1,
			TotalTokens:      2,
		},
	}, nil
}

func (p *Provider) StreamCompletion(ctx context.Context, req compat.CompletionsRequest) (provider.CompletionStream, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.StreamErr != nil {
		return nil, p.StreamErr
	}
	parts := append([]string(nil), p.StreamParts...)
	return &completionStream{provider: p, model: req.Model, parts: parts}, nil
}

func (p *Provider) CreateEmbedding(ctx context.Context, req compat.EmbeddingRequest) (*compat.EmbeddingResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.Err != nil {
		return nil, p.Err
	}
	return &compat.EmbeddingResponse{
		Object: "list",
		Model:  req.Model,
		Data: []compat.EmbeddingData{{
			Object:    "embedding",
			Index:     0,
			Embedding: []float64{0.1, 0.2, 0.3},
		}},
		Usage: &compat.Usage{
			PromptTokens: 1,
			TotalTokens:  1,
		},
	}, nil
}

type chatStream struct {
	provider *Provider
	model    string
	parts    []string
	index    int
	toolMode bool
}

func (s *chatStream) Next(ctx context.Context) (*compat.ChatCompletionChunk, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.toolMode {
		if s.index >= 2 {
			return nil, io.EOF
		}
		argument := `{"location":`
		if s.index == 1 {
			argument = `"Paris"}`
		}
		calls, _ := json.Marshal([]any{map[string]any{"index": 0, "id": "call_fake_weather", "type": "function", "function": map[string]string{"name": "get_weather", "arguments": argument}}})
		s.index++
		return &compat.ChatCompletionChunk{Model: s.model, Choices: []compat.ChatCompletionChunkChoice{{Index: 0, Delta: compat.ChatMessageDelta{Extra: map[string]json.RawMessage{"tool_calls": calls}}}}}, nil
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

func (s *chatStream) Close() error {
	s.provider.Closed.Store(true)
	return nil
}

type completionStream struct {
	provider *Provider
	model    string
	parts    []string
	index    int
}

func (s *completionStream) Next(ctx context.Context) (*compat.CompletionsChunk, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.index >= len(s.parts) {
		return nil, io.EOF
	}
	part := s.parts[s.index]
	s.index++
	return &compat.CompletionsChunk{
		ID:      "cmpl_fake",
		Object:  "text_completion.chunk",
		Created: time.Now().Unix(),
		Model:   s.model,
		Choices: []compat.CompletionsChunkChoice{{
			Text:  part,
			Index: 0,
		}},
	}, nil
}

func (s *completionStream) Close() error {
	s.provider.Closed.Store(true)
	return nil
}

func fakeToolResult(req compat.ChatCompletionRequest) (string, bool) {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role != "tool" {
			continue
		}
		var result string
		if json.Unmarshal(req.Messages[i].Content, &result) == nil {
			return result, true
		}
	}
	return "", false
}

func (p *Provider) CreateBatch(ctx context.Context, req compat.BatchRequest) (*compat.Batch, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.Err != nil {
		return nil, p.Err
	}
	return &compat.Batch{
		ID:               "batch_fake",
		Object:           "batch",
		Endpoint:         req.Endpoint,
		InputFileID:      req.InputFileID,
		CompletionWindow: req.CompletionWindow,
		Status:           "validating",
		CreatedAt:        time.Now().Unix(),
	}, nil
}

func (p *Provider) ListBatches(ctx context.Context, after string, limit int) (*compat.BatchList, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.Err != nil {
		return nil, p.Err
	}
	return &compat.BatchList{Object: "list", Data: []compat.Batch{}}, nil
}

func (p *Provider) RetrieveBatch(ctx context.Context, batchID string) (*compat.Batch, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.Err != nil {
		return nil, p.Err
	}
	return &compat.Batch{
		ID:      batchID,
		Object:  "batch",
		Status:  "completed",
	}, nil
}

func (p *Provider) CancelBatch(ctx context.Context, batchID string) (*compat.Batch, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.Err != nil {
		return nil, p.Err
	}
	return &compat.Batch{
		ID:      batchID,
		Object:  "batch",
		Status:  "cancelling",
	}, nil
}

func (p *Provider) UploadFile(ctx context.Context, req compat.FileUploadRequest) (*compat.FileObject, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.Err != nil {
		return nil, p.Err
	}
	return &compat.FileObject{
		ID:        "file_fake",
		Object:    "file",
		Bytes:     len(req.File),
		CreatedAt: time.Now().Unix(),
		Filename:  req.Filename,
		Purpose:   req.Purpose,
		Status:    "uploaded",
	}, nil
}

func (p *Provider) ListFiles(ctx context.Context) (*compat.FileList, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.Err != nil {
		return nil, p.Err
	}
	return &compat.FileList{Object: "list", Data: []compat.FileObject{}}, nil
}

func (p *Provider) RetrieveFile(ctx context.Context, fileID string) (*compat.FileObject, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.Err != nil {
		return nil, p.Err
	}
	return &compat.FileObject{ID: fileID, Object: "file", Filename: "test.jsonl", Purpose: "fine-tune", Bytes: 100, CreatedAt: time.Now().Unix(), Status: "processed"}, nil
}

func (p *Provider) DeleteFile(ctx context.Context, fileID string) (*compat.FileDeleteResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.Err != nil {
		return nil, p.Err
	}
	return &compat.FileDeleteResponse{ID: fileID, Object: "file", Deleted: true}, nil
}

func (p *Provider) DownloadFile(ctx context.Context, fileID string) ([]byte, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	if p.Err != nil {
		return nil, "", p.Err
	}
	return []byte("fake-file-content"), "application/octet-stream", nil
}
