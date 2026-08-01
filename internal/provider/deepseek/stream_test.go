package deepseek

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/conversation"
)

func TestStreamDecoderAccumulatesPrivateReasoningAndTools(t *testing.T) {
	decoder := NewDialectWithIDGenerator(func() (string, error) { return "env_stream", nil }).NewStreamDecoder()
	finish := "tool_calls"
	chunks := []compat.ChatCompletionChunk{
		{Choices: []compat.ChatCompletionChunkChoice{{
			Index: 0,
			Delta: compat.ChatMessageDelta{Role: "assistant", Extra: map[string]json.RawMessage{
				"reasoning_content": json.RawMessage(`"private "`),
			}},
		}}},
		{Choices: []compat.ChatCompletionChunkChoice{{
			Index: 0,
			Delta: compat.ChatMessageDelta{Content: "visible", Extra: map[string]json.RawMessage{
				"reasoning_content": json.RawMessage(`"reasoning"`),
				"tool_calls":        json.RawMessage(`[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":"}}]`),
			}},
		}}},
		{Choices: []compat.ChatCompletionChunkChoice{{
			Index: 0,
			Delta: compat.ChatMessageDelta{Extra: map[string]json.RawMessage{
				"tool_calls": json.RawMessage(`[{"index":0,"function":{"arguments":"1}"}}]`),
			}},
		}}},
		{Choices: []compat.ChatCompletionChunkChoice{{Index: 0, FinishReason: &finish}}},
		{Usage: &compat.Usage{TotalTokens: 7}},
	}
	var visible strings.Builder
	for _, chunk := range chunks {
		events, err := decoder.Push(chunk)
		if err != nil {
			t.Fatal(err)
		}
		for _, event := range events {
			visible.WriteString(event.TextDelta)
		}
	}
	if visible.String() != "visible" || strings.Contains(visible.String(), "private") {
		t.Fatalf("visible=%q", visible.String())
	}
	got, err := decoder.Finish()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Turn.Items) != 3 || got.Usage.TotalTokens != 7 {
		t.Fatalf("response=%#v", got)
	}
	reasoning := got.Turn.Items[0].(conversation.Reasoning)
	if reasoning.Content != "private reasoning" || reasoning.AssistantContent != "visible" || reasoning.CallIDs[0] != "call_1" {
		t.Fatalf("reasoning=%#v", reasoning)
	}
	if got.Turn.Items[1].(conversation.Message).Text != "visible" || got.Turn.Items[2].(conversation.FunctionCall).Arguments != `{"q":1}` {
		t.Fatalf("turn=%#v", got.Turn)
	}
}

func TestStreamDecoderRejectsIncompleteReasoningToolCall(t *testing.T) {
	decoder := NewDialect().NewStreamDecoder()
	finish := "tool_calls"
	_, err := decoder.Push(compat.ChatCompletionChunk{Choices: []compat.ChatCompletionChunkChoice{{
		Index: 0,
		Delta: compat.ChatMessageDelta{Extra: map[string]json.RawMessage{
			"tool_calls": json.RawMessage(`[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]`),
		}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decoder.Push(compat.ChatCompletionChunk{Choices: []compat.ChatCompletionChunkChoice{{Index: 0, FinishReason: &finish}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := decoder.Finish(); !errors.Is(err, ErrIncompleteReasoningToolCall) {
		t.Fatalf("error=%v", err)
	}
}
