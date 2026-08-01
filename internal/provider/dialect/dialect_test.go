package dialect

import (
	"encoding/json"
	"strings"
	"testing"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/conversation"
)

func TestRegistryRejectsInvalidAndDuplicateDialects(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(nil); err == nil {
		t.Fatal("nil dialect accepted")
	}
	openai := NewOpenAICompatible()
	if err := registry.Register(openai); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(openai); err == nil {
		t.Fatal("duplicate dialect accepted")
	}
	got, err := registry.Get("openai-compatible")
	if err != nil || got.Name() != "openai-compatible" || got.Capabilities().ReasoningReplay {
		t.Fatalf("dialect=%v err=%v", got, err)
	}
	if _, err := registry.Get("missing"); err == nil {
		t.Fatal("missing dialect found")
	}
}

func TestOpenAICompatibleBuildsChatRequest(t *testing.T) {
	dialect := NewOpenAICompatible()
	req := Request{UpstreamModel: "upstream-model", Conversation: conversation.Request{
		Model: "external-model", Stream: true, ReasoningEffort: "high",
		Turn: conversation.Turn{Items: []conversation.Item{
			conversation.Message{Role: "system", Text: "be concise"},
			conversation.Message{Role: "user", Text: "weather"},
			conversation.FunctionCall{CallID: "call_1", Name: "weather", Arguments: `{"city":"Paris"}`},
			conversation.FunctionOutput{CallID: "call_1", Output: json.RawMessage(`"sunny"`)},
		}},
		Tools:      []conversation.FunctionTool{{Name: "weather", Description: "Get weather", Parameters: json.RawMessage(`{"type":"object"}`), Strict: true}},
		ToolChoice: json.RawMessage(`"auto"`),
		Extra:      map[string]json.RawMessage{"temperature": json.RawMessage(`0.2`), "custom": json.RawMessage(`true`)},
	}}
	got, err := dialect.BuildChatRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	if got.Model != "upstream-model" || !got.Stream || len(got.Messages) != 4 {
		t.Fatalf("request=%#v", got)
	}
	if got.Messages[2].Role != "assistant" || !strings.Contains(string(got.Messages[2].Extra["tool_calls"]), `"id":"call_1"`) || got.Messages[3].Role != "tool" {
		t.Fatalf("messages=%#v", got.Messages)
	}
	if got.Temperature == nil || *got.Temperature != 0.2 {
		t.Fatalf("temperature=%v", got.Temperature)
	}
	for key, want := range map[string]string{"tool_choice": `"auto"`, "reasoning_effort": `"high"`, "custom": `true`} {
		if string(got.Extra[key]) != want {
			t.Fatalf("extra[%s]=%s want=%s", key, got.Extra[key], want)
		}
	}
	if !strings.Contains(string(got.Extra["tools"]), `"strict":true`) {
		t.Fatalf("tools=%s", got.Extra["tools"])
	}
}

func TestOpenAICompatibleRejectsReasoningReplay(t *testing.T) {
	_, err := NewOpenAICompatible().BuildChatRequest(Request{UpstreamModel: "model", Conversation: conversation.Request{
		Model: "model", Turn: conversation.Turn{Items: []conversation.Item{
			conversation.Reasoning{EnvelopeID: "env_1", Content: "private"},
		}},
	}})
	if err == nil {
		t.Fatal("reasoning replay accepted")
	}
}

func TestOpenAICompatibleParsesResponse(t *testing.T) {
	response := compat.ChatCompletionResponse{
		Choices: []compat.ChatCompletionChoice{{Index: 0, Message: compat.ChatMessage{
			Role: "assistant", Content: json.RawMessage(`"done"`),
			Extra: map[string]json.RawMessage{"tool_calls": json.RawMessage(`[{"id":"call_1","type":"function","function":{"name":"next","arguments":"{}"}}]`)},
		}, FinishReason: "tool_calls"}},
		Usage: &compat.Usage{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5},
	}
	got, err := NewOpenAICompatible().ParseChatResponse(response)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Turn.Items) != 2 || got.Turn.Items[0].(conversation.Message).Text != "done" || got.Turn.Items[1].(conversation.FunctionCall).CallID != "call_1" || got.Usage.TotalTokens != 5 {
		t.Fatalf("response=%#v", got)
	}
}

func TestOpenAICompatibleStreamDecoderRebuildsTurn(t *testing.T) {
	decoder := NewOpenAICompatible().NewStreamDecoder()
	finish := "tool_calls"
	chunks := []compat.ChatCompletionChunk{
		{Choices: []compat.ChatCompletionChunkChoice{{Index: 0, Delta: compat.ChatMessageDelta{Role: "assistant", Content: "hel"}}}},
		{Choices: []compat.ChatCompletionChunkChoice{{Index: 0, Delta: compat.ChatMessageDelta{Content: "lo", Extra: map[string]json.RawMessage{"tool_calls": json.RawMessage(`[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":"}}]`)}}}}},
		{Choices: []compat.ChatCompletionChunkChoice{{Index: 0, Delta: compat.ChatMessageDelta{Extra: map[string]json.RawMessage{"tool_calls": json.RawMessage(`[{"index":0,"function":{"arguments":"\"x\"}"}}]`)}}}}},
		{Choices: []compat.ChatCompletionChunkChoice{{Index: 0, Delta: compat.ChatMessageDelta{}, FinishReason: &finish}}},
		{Usage: &compat.Usage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3}},
	}
	var text strings.Builder
	for _, chunk := range chunks {
		events, err := decoder.Push(chunk)
		if err != nil {
			t.Fatal(err)
		}
		for _, event := range events {
			text.WriteString(event.TextDelta)
		}
	}
	if text.String() != "hello" {
		t.Fatalf("text=%q", text.String())
	}
	got, err := decoder.Finish()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Turn.Items) != 2 || got.Turn.Items[0].(conversation.Message).Text != "hello" {
		t.Fatalf("turn=%#v", got.Turn)
	}
	call := got.Turn.Items[1].(conversation.FunctionCall)
	if call.CallID != "call_1" || call.Name != "lookup" || call.Arguments != `{"q":"x"}` || got.Usage.TotalTokens != 3 {
		t.Fatalf("call=%#v usage=%#v", call, got.Usage)
	}
}

func TestOpenAICompatibleStreamDecoderRejectsIncompleteStream(t *testing.T) {
	decoder := NewOpenAICompatible().NewStreamDecoder()
	if _, err := decoder.Push(compat.ChatCompletionChunk{Choices: []compat.ChatCompletionChunkChoice{{Index: 0, Delta: compat.ChatMessageDelta{Content: "partial"}}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := decoder.Finish(); err == nil {
		t.Fatal("unfinished stream accepted")
	}
}

func TestOpenAICompatibleStreamDecoderPreservesOutOfOrderParallelCallArrival(t *testing.T) {
	decoder := NewOpenAICompatible().NewStreamDecoder()
	finish := "tool_calls"
	chunks := []compat.ChatCompletionChunk{
		{Choices: []compat.ChatCompletionChunkChoice{{Index: 0, Delta: compat.ChatMessageDelta{Extra: map[string]json.RawMessage{"tool_calls": json.RawMessage(`[{"index":1,"id":"call_2","type":"function","function":{"name":"second","arguments":"{}"}}]`)}}}}},
		{Choices: []compat.ChatCompletionChunkChoice{{Index: 0, Delta: compat.ChatMessageDelta{Extra: map[string]json.RawMessage{"tool_calls": json.RawMessage(`[{"index":0,"id":"call_1","type":"function","function":{"name":"first","arguments":"{}"}}]`)}}}}},
		{Choices: []compat.ChatCompletionChunkChoice{{Index: 0, FinishReason: &finish}}},
	}
	var deltas []string
	for _, chunk := range chunks {
		events, err := decoder.Push(chunk)
		if err != nil {
			t.Fatal(err)
		}
		for _, event := range events {
			if event.FunctionCallDelta != nil {
				deltas = append(deltas, event.FunctionCallDelta.CallID)
			}
		}
	}
	got, err := decoder.Finish()
	if err != nil {
		t.Fatal(err)
	}
	if len(deltas) != 2 || deltas[0] != "call_2" || deltas[1] != "call_1" {
		t.Fatalf("deltas=%#v", deltas)
	}
	if len(got.Turn.Items) != 2 || got.Turn.Items[0].(conversation.FunctionCall).CallID != "call_2" || got.Turn.Items[1].(conversation.FunctionCall).CallID != "call_1" {
		t.Fatalf("turn=%#v", got.Turn)
	}
}
