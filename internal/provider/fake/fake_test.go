package fake

import (
	"context"
	"encoding/json"
	"testing"

	"open-ai-gateway/internal/compat"
)

func TestFunctionToolRoundTripBehavior(t *testing.T) {
	p := New()
	tools := json.RawMessage(`[{"type":"function","function":{"name":"get_weather","parameters":{},"strict":true}}]`)
	first, err := p.CreateChatCompletion(context.Background(), compat.ChatCompletionRequest{Model: "m", Messages: []compat.ChatMessage{{Role: "user", Content: json.RawMessage(`"weather"`)}}, Extra: map[string]json.RawMessage{"tools": tools}})
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(first.Choices[0].Message.Extra["tool_calls"]) {
		t.Fatalf("tool_calls=%s", first.Choices[0].Message.Extra["tool_calls"])
	}
	second, err := p.CreateChatCompletion(context.Background(), compat.ChatCompletionRequest{Model: "m", Messages: []compat.ChatMessage{{Role: "tool", Content: json.RawMessage(`"25C"`), Extra: map[string]json.RawMessage{"tool_call_id": json.RawMessage(`"call_fake_weather"`)}}}, Extra: map[string]json.RawMessage{"tools": tools}})
	if err != nil {
		t.Fatal(err)
	}
	var text string
	if json.Unmarshal(second.Choices[0].Message.Content, &text) != nil || text != "Tool result received: 25C" {
		t.Fatalf("content=%s", second.Choices[0].Message.Content)
	}
}
