package compat

import (
	"encoding/json"
	"testing"
	"time"
)

func TestResponseRequestStringInputToChat(t *testing.T) {
	var req ResponseRequest
	if err := json.Unmarshal([]byte(`{"model":"test-model","instructions":"be concise","input":"hello"}`), &req); err != nil {
		t.Fatal(err)
	}
	chat, compatErr := req.ChatRequest()
	if compatErr != nil {
		t.Fatal(compatErr)
	}
	if chat.Model != "test-model" || len(chat.Messages) != 2 {
		t.Fatalf("unexpected chat request: %#v", chat)
	}
	if chat.Messages[0].Role != "developer" || string(chat.Messages[0].Content) != `"be concise"` {
		t.Fatalf("unexpected instructions: %#v", chat.Messages[0])
	}
	if chat.Messages[1].Role != "user" || string(chat.Messages[1].Content) != `"hello"` {
		t.Fatalf("unexpected input: %#v", chat.Messages[1])
	}
}

func TestResponseRequestMessagePartsToChat(t *testing.T) {
	var req ResponseRequest
	body := `{"model":"test-model","stream":true,"store":false,"input":[{"role":"user","content":[{"type":"input_text","text":"hello "},{"type":"input_text","text":"world"}]}]}`
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatal(err)
	}
	chat, compatErr := req.ChatRequest()
	if compatErr != nil {
		t.Fatal(compatErr)
	}
	if !chat.Stream || len(chat.Messages) != 1 || string(chat.Messages[0].Content) != `"hello world"` {
		t.Fatalf("unexpected chat request: %#v", chat)
	}
}

func TestResponseRequestRejectsUnsupportedFields(t *testing.T) {
	for _, body := range []string{
		`{"model":"m","input":"x","previous_response_id":"resp_1"}`,
		`{"model":"m","input":"x","store":true}`,
		`{"model":"m","input":[{"role":"user","content":[{"type":"input_image","image_url":"x"}]}]}`,
	} {
		var req ResponseRequest
		if err := json.Unmarshal([]byte(body), &req); err != nil {
			t.Fatal(err)
		}
		if _, compatErr := req.ChatRequest(); compatErr == nil || compatErr.Status != 400 || compatErr.Type != "invalid_request_error" {
			t.Fatalf("expected invalid_request_error for %s, got %#v", body, compatErr)
		}
	}
}

func TestNewResponseEnvelopeMapsTextAndUsage(t *testing.T) {
	chat := &ChatCompletionResponse{
		Choices: []ChatCompletionChoice{{Index: 0, Message: ChatMessage{Role: "assistant", Content: json.RawMessage(`"hello"`)}, FinishReason: "stop"}},
		Usage:   &Usage{PromptTokens: 3, CompletionTokens: 2, TotalTokens: 5},
	}
	got, compatErr := NewResponseEnvelope("test-model", chat, time.Unix(123, 0), "resp_1", "msg_1")
	if compatErr != nil {
		t.Fatal(compatErr)
	}
	if got.Object != "response" || got.Status != "completed" || got.Model != "test-model" || got.CreatedAt != 123 {
		t.Fatalf("unexpected response: %#v", got)
	}
	if len(got.Output) != 1 || got.Output[0].Content[0].Text != "hello" {
		t.Fatalf("unexpected output: %#v", got.Output)
	}
	if got.Usage == nil || got.Usage.InputTokens != 3 || got.Usage.OutputTokens != 2 || got.Usage.TotalTokens != 5 {
		t.Fatalf("unexpected usage: %#v", got.Usage)
	}
}

func TestNewResponseEnvelopeRejectsAmbiguousChatOutput(t *testing.T) {
	for _, choices := range [][]ChatCompletionChoice{nil, {{}, {}}} {
		_, compatErr := NewResponseEnvelope("m", &ChatCompletionResponse{Choices: choices}, time.Time{}, "resp", "msg")
		if compatErr == nil {
			t.Fatal("expected conversion error")
		}
	}
}
