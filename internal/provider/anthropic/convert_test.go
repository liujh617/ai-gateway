package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"open-ai-gateway/internal/compat"
)

func TestOpenAIToAnthropic(t *testing.T) {
	req := compat.ChatCompletionRequest{
		Model: "claude-sonnet-5",
		Messages: []compat.ChatMessage{
			{Role: "system", Content: json.RawMessage(`"You are helpful"`)},
			{Role: "user", Content: json.RawMessage(`"Hello"`)},
		},
	}
	ar := openaiToAnthropic(req)
	if ar.Model != "claude-sonnet-5" {
		t.Fatalf("model=%q", ar.Model)
	}
	if ar.System != "You are helpful" {
		t.Fatalf("system=%q", ar.System)
	}
	if len(ar.Messages) != 1 || ar.Messages[0].Content[0].Type != "text" {
		t.Fatalf("messages=%#v", ar.Messages)
	}
}

func TestAnthropicToOpenAI(t *testing.T) {
	resp := MessageResponse{
		ID:    "msg_1",
		Model: "claude-3",
		Role:  "assistant",
		Content: []ContentBlock{
			{Type: "text", Text: "Hi!"},
		},
		StopReason: "end_turn",
		Usage:      &Usage{InputTokens: 5, OutputTokens: 2},
	}
	openaiResp := anthropicToOpenAI(resp, "claude-3")
	if openaiResp.Choices[0].FinishReason != "stop" {
		t.Fatalf("finish=%q", openaiResp.Choices[0].FinishReason)
	}
	if openaiResp.Usage.TotalTokens != 7 {
		t.Fatalf("usage=%#v", openaiResp.Usage)
	}
}

func TestAnthropicToOpenAIToolUse(t *testing.T) {
	resp := MessageResponse{
		ID:    "msg_2",
		Model: "claude-3",
		Content: []ContentBlock{
			{Type: "text", Text: ""},
			{Type: "tool_use", ID: "toolu_1", Name: "get_weather", Input: json.RawMessage(`{"location":"Paris"}`)},
		},
		StopReason: "tool_use",
	}
	openaiResp := anthropicToOpenAI(resp, "claude-3")
	if openaiResp.Choices[0].FinishReason != "tool_calls" {
		t.Fatalf("finish=%q", openaiResp.Choices[0].FinishReason)
	}
	if openaiResp.Choices[0].Message.Extra["tool_calls"] == nil {
		t.Fatal("no tool_calls")
	}
}

func TestAnthropicErrorToOpenAI(t *testing.T) {
	body := []byte(`{"type":"error","error":{"type":"rate_limit_error","message":"Rate limited"}}`)
	err := anthropicErrorToOpenAI(body, 429)
	if err.Type != "rate_limit_error" || err.Status != 429 {
		t.Fatalf("error=%#v", err)
	}
}

func TestStreamParser(t *testing.T) {
	input := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-3\"}}\n\nevent: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n"
	stream := NewMessageStream(strings.NewReader(input))
	events := []string{}
	for {
		event, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		events = append(events, event.Type)
	}
	if len(events) != 3 || events[0] != "message_start" {
		t.Fatalf("events=%v", events)
	}
}

func TestProviderCreateChatCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","model":"claude-3","content":[{"type":"text","text":"Hello!"}],"stop_reason":"end_turn","usage":{"input_tokens":5,"output_tokens":2}}`))
	}))
	defer server.Close()

	p, err := New(server.URL, "test-key", 0)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	resp, err := p.CreateChatCompletion(context.Background(), compat.ChatCompletionRequest{
		Model: "claude-3",
		Messages: []compat.ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Hi"`)},
		},
	})
	if err != nil {
		t.Fatalf("CreateChatCompletion: %v", err)
	}
	if resp.Choices[0].Message.Content == nil {
		t.Fatal("empty response")
	}
}

func TestStreamConvertContentBlockStartToolUse(t *testing.T) {
	event := &MessageStreamEvent{
		Type:  "content_block_start",
		Index: 0,
		ContentBlock: &ContentBlock{
			Type: "tool_use",
			ID:   "toolu_01A",
			Name: "get_weather",
		},
	}
	chunk := convertStreamEvent(event, "claude-3")
	extra := chunk.Choices[0].Delta.Extra
	if extra == nil || extra["tool_calls"] == nil {
		t.Fatal("missing tool_calls in delta")
	}
	var calls []map[string]any
	if err := json.Unmarshal(extra["tool_calls"], &calls); err != nil {
		t.Fatalf("unmarshal tool_calls: %v", err)
	}
	if len(calls) != 1 || calls[0]["id"] != "toolu_01A" {
		t.Fatalf("calls=%v", calls)
	}
}

func TestStreamConvertToolUseArgsDelta(t *testing.T) {
	event := &MessageStreamEvent{
		Type:  "content_block_delta",
		Index: 0,
		Delta: &StreamDelta{
			Type:        "input_json_delta",
			PartialJSON: `"{"location":"Paris"}"`,
		},
	}
	chunk := convertStreamEvent(event, "claude-3")
	extra := chunk.Choices[0].Delta.Extra
	if extra == nil || extra["tool_calls"] == nil {
		t.Fatal("missing tool_calls in args delta")
	}
}

func TestStreamConvertMessageDelta(t *testing.T) {
	event := &MessageStreamEvent{
		Type: "message_delta",
		Delta: &StreamDelta{
			StopReason: "end_turn",
		},
		Usage: &Usage{InputTokens: 10, OutputTokens: 5},
	}
	chunk := convertStreamEvent(event, "claude-3")
	if chunk.Choices[0].FinishReason == nil || *chunk.Choices[0].FinishReason != "stop" {
		t.Fatalf("finish_reason=%v", chunk.Choices[0].FinishReason)
	}
	if chunk.Usage == nil || chunk.Usage.TotalTokens != 15 {
		t.Fatalf("usage=%#v", chunk.Usage)
	}
}

func TestStreamChatCompletionFullToolUseFlow(t *testing.T) {
	streamBody := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-3\"}}\n\nevent: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"toolu_01\",\"name\":\"get_weather\"}}\n\nevent: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"location\\\":\\\"Paris\\\"}\"}}\n\nevent: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\nevent: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"tool_use\"},\"usage\":{\"input_tokens\":10,\"output_tokens\":30}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(streamBody))
	}))
	defer server.Close()
	p, _ := New(server.URL, "test-key", 0)
	stream, err := p.StreamChatCompletion(context.Background(), compat.ChatCompletionRequest{
		Model:    "claude-3",
		Messages: []compat.ChatMessage{{Role: "user", Content: json.RawMessage(`"Weather?"`)}},
	})
	if err != nil {
		t.Fatalf("StreamChatCompletion: %v", err)
	}
	defer stream.Close()
	count := 0
	for {
		_, err := stream.Next(context.Background())
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		count++
	}
	if count < 4 {
		t.Fatalf("got %d chunks", count)
	}
}
