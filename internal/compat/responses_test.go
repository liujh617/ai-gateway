package compat

import (
	"encoding/json"
	"strings"
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

func TestResponseRequestConvertsFunctionTools(t *testing.T) {
	body := `{"model":"m","input":"weather","tools":[{"type":"function","name":"get_weather","description":"Get weather","parameters":{"type":"object"}}],"tool_choice":{"type":"function","name":"get_weather"},"parallel_tool_calls":false}`
	var req ResponseRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatal(err)
	}
	chat, compatErr := req.ChatRequest()
	if compatErr != nil {
		t.Fatal(compatErr)
	}
	tools := string(chat.Extra["tools"])
	if !strings.Contains(tools, `"strict":true`) || !strings.Contains(tools, `"function":{"description":"Get weather","name":"get_weather"`) {
		t.Fatalf("tools=%s", tools)
	}
	if got := string(chat.Extra["tool_choice"]); got != `{"function":{"name":"get_weather"},"type":"function"}` {
		t.Fatalf("tool_choice=%s", got)
	}
	if got := string(chat.Extra["parallel_tool_calls"]); got != "false" {
		t.Fatalf("parallel_tool_calls=%s", got)
	}
}

func TestResponseRequestConvertsFunctionCallAndOutput(t *testing.T) {
	body := `{"model":"m","input":[{"type":"function_call","id":"fc_1","call_id":"call_1","name":"get_weather","arguments":"{\"location\":\"Paris\"}","status":"completed"},{"type":"function_call_output","call_id":"call_1","output":"25C"}]}`
	var req ResponseRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatal(err)
	}
	chat, compatErr := req.ChatRequest()
	if compatErr != nil {
		t.Fatal(compatErr)
	}
	if len(chat.Messages) != 2 || chat.Messages[0].Role != "assistant" || chat.Messages[1].Role != "tool" {
		t.Fatalf("messages=%#v", chat.Messages)
	}
	if !strings.Contains(string(chat.Messages[0].Extra["tool_calls"]), `"id":"call_1"`) {
		t.Fatalf("tool_calls=%s", chat.Messages[0].Extra["tool_calls"])
	}
	if string(chat.Messages[1].Extra["tool_call_id"]) != `"call_1"` || string(chat.Messages[1].Content) != `"25C"` {
		t.Fatalf("tool message=%#v", chat.Messages[1])
	}
}

func TestResponseRequestRejectsInvalidFunctionCorrelation(t *testing.T) {
	bodies := []string{
		`{"model":"m","input":"x","tools":[{"type":"web_search"}]}`,
		`{"model":"m","input":"x","tools":[{"type":"function","name":"f","parameters":[]}]}`,
		`{"model":"m","input":"x","tools":[{"type":"function","name":"f","parameters":{}}],"tool_choice":{"type":"function","name":"missing"}}`,
		`{"model":"m","input":[{"type":"function_call_output","call_id":"call_1","output":"x"}]}`,
		`{"model":"m","input":[{"type":"function_call","call_id":"call_1","name":"f","arguments":"not-json"}]}`,
	}
	for _, body := range bodies {
		var req ResponseRequest
		if err := json.Unmarshal([]byte(body), &req); err != nil {
			t.Fatal(err)
		}
		if _, compatErr := req.ChatRequest(); compatErr == nil || compatErr.Status != 400 {
			t.Fatalf("body=%s err=%#v", body, compatErr)
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
