package azureopenai_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/provider/azureopenai"
	"open-ai-gateway/internal/requestctx"
	"open-ai-gateway/internal/version"
)

func TestCreateChatCompletionForwardsAzureRequest(t *testing.T) {
	var got compat.ChatCompletionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openai/deployments/chat-deployment/chat/completions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if apiVersion := r.URL.Query().Get("api-version"); apiVersion != "2024-02-15-preview" {
			t.Fatalf("api-version = %q", apiVersion)
		}
		assertCommonHeaders(t, r, "application/json")
		if requestID := r.Header.Get(requestctx.RequestIDHeader); requestID != "gateway-request-1" {
			t.Fatalf("request id = %q", requestID)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"chatcmpl_azure","object":"chat.completion","created":1,"model":"chat-deployment","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}`)
	}))
	defer server.Close()

	p := newProvider(t, server.URL)
	ctx := requestctx.WithRequestID(context.Background(), "gateway-request-1")
	resp, err := p.CreateChatCompletion(ctx, compat.ChatCompletionRequest{
		Model: "chat-deployment",
		Messages: []compat.ChatMessage{{
			Role:    "user",
			Content: json.RawMessage(`"hello"`),
		}},
		Stream: true,
		Extra: map[string]json.RawMessage{
			"tool_choice": json.RawMessage(`"auto"`),
		},
	})
	if err != nil {
		t.Fatalf("CreateChatCompletion: %v", err)
	}
	if got.Model != "chat-deployment" {
		t.Fatalf("model = %q", got.Model)
	}
	if got.Stream {
		t.Fatal("non-stream request forwarded with stream=true")
	}
	if string(got.Extra["tool_choice"]) != `"auto"` {
		t.Fatalf("tool_choice = %s", got.Extra["tool_choice"])
	}
	if resp.ID != "chatcmpl_azure" {
		t.Fatalf("response id = %q", resp.ID)
	}
}

func TestStreamChatCompletionForwardsAzureRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openai/deployments/chat-deployment/chat/completions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if apiVersion := r.URL.Query().Get("api-version"); apiVersion != "2024-02-15-preview" {
			t.Fatalf("api-version = %q", apiVersion)
		}
		assertCommonHeaders(t, r, "text/event-stream")
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"id\":\"chunk\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"chat-deployment\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"},\"finish_reason\":null}]}\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	p := newProvider(t, server.URL)
	stream, err := p.StreamChatCompletion(context.Background(), chatRequest())
	if err != nil {
		t.Fatalf("StreamChatCompletion: %v", err)
	}
	defer stream.Close()
	chunk, err := stream.Next(context.Background())
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if chunk.Choices[0].Delta.Content != "hi" {
		t.Fatalf("content = %q", chunk.Choices[0].Delta.Content)
	}
	if _, err := stream.Next(context.Background()); err != io.EOF {
		t.Fatalf("done err = %v, want EOF", err)
	}
}

func TestCreateEmbeddingForwardsAzureRequest(t *testing.T) {
	var got compat.EmbeddingRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openai/deployments/embedding-deployment/embeddings" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if apiVersion := r.URL.Query().Get("api-version"); apiVersion != "2024-02-15-preview" {
			t.Fatalf("api-version = %q", apiVersion)
		}
		assertCommonHeaders(t, r, "application/json")
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"object":"list","model":"embedding-deployment","data":[{"object":"embedding","index":0,"embedding":[0.1]}],"usage":{"prompt_tokens":1,"total_tokens":1}}`)
	}))
	defer server.Close()

	p := newProvider(t, server.URL)
	resp, err := p.CreateEmbedding(context.Background(), compat.EmbeddingRequest{
		Model: "embedding-deployment",
		Input: json.RawMessage(`"hello"`),
		Extra: map[string]json.RawMessage{
			"dimensions": json.RawMessage(`512`),
		},
	})
	if err != nil {
		t.Fatalf("CreateEmbedding: %v", err)
	}
	if got.Model != "embedding-deployment" {
		t.Fatalf("model = %q", got.Model)
	}
	if string(got.Extra["dimensions"]) != `512` {
		t.Fatalf("dimensions = %s", got.Extra["dimensions"])
	}
	if resp.Model != "embedding-deployment" {
		t.Fatalf("response model = %q", resp.Model)
	}
}

func TestNewRejectsMissingAPIVersion(t *testing.T) {
	_, err := azureopenai.New("https://example.openai.azure.com", "key", "", 0)
	if err == nil {
		t.Fatal("expected api_version error")
	}
	if !strings.Contains(err.Error(), "api_version") {
		t.Fatalf("error = %v", err)
	}
}

func TestNewRejectsBaseURLWithQueryOrFragment(t *testing.T) {
	for _, baseURL := range []string{
		"https://example.openai.azure.com?tenant=one",
		"https://example.openai.azure.com#frag",
	} {
		t.Run(baseURL, func(t *testing.T) {
			_, err := azureopenai.New(baseURL, "key", "2024-02-15-preview", 0)
			if err == nil {
				t.Fatal("expected base_url error")
			}
		})
	}
}

func TestCreateChatCompletionMapsUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, `{"error":{"message":"slow down","type":"rate_limit_error","code":"rate_limit_exceeded"}}`)
	}))
	defer server.Close()

	p := newProvider(t, server.URL)
	_, err := p.CreateChatCompletion(context.Background(), chatRequest())
	if err == nil {
		t.Fatal("expected error")
	}
	compatErr, ok := err.(*compat.Error)
	if !ok {
		t.Fatalf("error type = %T", err)
	}
	if compatErr.Status != http.StatusTooManyRequests || compatErr.Type != "rate_limit_error" {
		t.Fatalf("mapped error = %+v", compatErr)
	}
}

func TestCreateChatCompletionTransportTimeoutIsDeadlineExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer server.Close()

	p, err := azureopenai.New(server.URL, "azure-key", "2024-02-15-preview", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	_, err = p.CreateChatCompletion(context.Background(), chatRequest())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want DeadlineExceeded", err)
	}
}

func TestListModelsReturnsEmptyList(t *testing.T) {
	p, err := azureopenai.New("https://example.openai.azure.com", "azure-key", "2024-02-15-preview", 0)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 0 {
		t.Fatalf("models = %+v", models)
	}
}

func assertCommonHeaders(t *testing.T, r *http.Request, accept string) {
	t.Helper()
	if got := r.Header.Get("Accept"); got != accept {
		t.Fatalf("Accept = %q", got)
	}
	if got := r.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := r.Header.Get("api-key"); got != "azure-key" {
		t.Fatalf("api-key = %q", got)
	}
	if got := r.Header.Get("Authorization"); got != "" {
		t.Fatalf("Authorization = %q", got)
	}
	if got := r.Header.Get("User-Agent"); got != version.UserAgent() {
		t.Fatalf("User-Agent = %q", got)
	}
}

func newProvider(t *testing.T, baseURL string) *azureopenai.Provider {
	t.Helper()
	p, err := azureopenai.New(baseURL, "azure-key", "2024-02-15-preview", 0)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	return p
}

func chatRequest() compat.ChatCompletionRequest {
	return compat.ChatCompletionRequest{
		Model: "chat-deployment",
		Messages: []compat.ChatMessage{{
			Role:    "user",
			Content: json.RawMessage(`"hello"`),
		}},
	}
}

func TestCreateSpeechForwardsAzureRequest(t *testing.T) {
	var got compat.SpeechRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openai/deployments/tts-deployment/audio/speech" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("api-version") != "2024-02-15-preview" {
			t.Fatalf("api-version = %s", r.URL.Query().Get("api-version"))
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("content-type = %q", ct)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write([]byte("azure-audio-data"))
	}))
	defer server.Close()

	p := newProvider(t, server.URL)
	resp, err := p.CreateSpeech(context.Background(), compat.SpeechRequest{
		Model: "tts-deployment",
		Input: "hello azure",
		Voice: "nova",
	})
	if err != nil {
		t.Fatalf("CreateSpeech: %v", err)
	}
	if string(resp.Data) != "azure-audio-data" {
		t.Fatalf("data = %q", resp.Data)
	}
	if resp.ContentType != "audio/mpeg" {
		t.Fatalf("content-type = %q", resp.ContentType)
	}
}

func TestCreateTranscriptionForwardsAzureRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openai/deployments/whisper-deployment/audio/transcriptions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("api-version") != "2024-02-15-preview" {
			t.Fatalf("api-version = %s", r.URL.Query().Get("api-version"))
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Fatalf("content-type = %q", r.Header.Get("Content-Type"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"text":"azure transcription"}`))
	}))
	defer server.Close()

	p := newProvider(t, server.URL)
	resp, err := p.CreateTranscription(context.Background(), compat.AudioTranscriptionRequest{
		Model:    "whisper-deployment",
		File:     []byte("audio-data"),
		Filename: "test.mp3",
	})
	if err != nil {
		t.Fatalf("CreateTranscription: %v", err)
	}
	if resp.Text != "azure transcription" {
		t.Fatalf("text = %q", resp.Text)
	}
}

func TestCreateTranslationForwardsAzureRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openai/deployments/whisper-deployment/audio/translations" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("api-version") != "2024-02-15-preview" {
			t.Fatalf("api-version = %s", r.URL.Query().Get("api-version"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"text":"azure translation"}`))
	}))
	defer server.Close()

	p := newProvider(t, server.URL)
	resp, err := p.CreateTranslation(context.Background(), compat.AudioTranslationRequest{
		Model:    "whisper-deployment",
		File:     []byte("audio-data"),
		Filename: "test.mp3",
	})
	if err != nil {
		t.Fatalf("CreateTranslation: %v", err)
	}
	if resp.Text != "azure translation" {
		t.Fatalf("text = %q", resp.Text)
	}
}

func TestCreateCompletionForwardsAzureRequest(t *testing.T) {
	var got compat.CompletionsRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openai/deployments/completion-deployment/completions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("api-version") != "2024-02-15-preview" {
			t.Fatalf("api-version = %s", r.URL.Query().Get("api-version"))
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"cmpl_1","object":"text_completion","created":1,"model":"completion-deployment","choices":[{"index":0,"text":"hello","finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	p := newProvider(t, server.URL)
	resp, err := p.CreateCompletion(context.Background(), compat.CompletionsRequest{Model: "completion-deployment", Prompt: json.RawMessage(`"hello"`)})
	if err != nil {
		t.Fatalf("CreateCompletion: %v", err)
	}
	if resp.Model != "completion-deployment" || len(resp.Choices) != 1 {
		t.Fatalf("response = %#v", resp)
	}
}

func TestCreateImageForwardsAzureRequest(t *testing.T) {
	var got compat.ImageGenerationRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openai/deployments/dalle-deployment/images/generations" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("api-version") != "2024-02-15-preview" {
			t.Fatalf("api-version = %s", r.URL.Query().Get("api-version"))
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"created":1,"data":[{"url":"https://example.com/img.png"}]}`))
	}))
	defer server.Close()

	p := newProvider(t, server.URL)
	resp, err := p.CreateImage(context.Background(), compat.ImageGenerationRequest{Model: "dalle-deployment", Prompt: "a cat"})
	if err != nil {
		t.Fatalf("CreateImage: %v", err)
	}
	if len(resp.Data) != 1 || resp.Data[0].URL != "https://example.com/img.png" {
		t.Fatalf("response = %#v", resp)
	}
}

func TestCreateModerationForwardsAzureRequest(t *testing.T) {
	var got compat.ModerationRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openai/deployments/moderation-deployment/moderations" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("api-version") != "2024-02-15-preview" {
			t.Fatalf("api-version = %s", r.URL.Query().Get("api-version"))
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"modr_1","model":"text-moderation-stable","results":[{"flagged":false,"categories":{},"category_scores":{}}]}`))
	}))
	defer server.Close()

	p := newProvider(t, server.URL)
	resp, err := p.CreateModeration(context.Background(), compat.ModerationRequest{Model: "moderation-deployment", Input: json.RawMessage(`"hello"`)})
	if err != nil {
		t.Fatalf("CreateModeration: %v", err)
	}
	if resp.ID != "modr_1" || len(resp.Results) != 1 {
		t.Fatalf("response = %#v", resp)
	}
}
