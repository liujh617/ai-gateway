package deepseek

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/conversation"
	providerdialect "open-ai-gateway/internal/provider/dialect"
)

func TestDialectBuildsThinkingContinuationRequest(t *testing.T) {
	route := conversation.RouteBinding{Dialect: "deepseek", Provider: "deepseek-primary", UpstreamModel: "deepseek-chat"}
	request := providerdialect.Request{UpstreamModel: "deepseek-chat", Route: route, Conversation: conversation.Request{
		Model: "codex-model", Stream: true, ReasoningEffort: "medium",
		Turn: conversation.Turn{Items: []conversation.Item{
			conversation.Message{Role: "system", Text: "be concise"},
			conversation.Message{Role: "user", Text: "weather"},
			conversation.Reasoning{EnvelopeID: "env_1", Content: "private reasoning", AssistantContent: "", CallIDs: []string{"call_1"}, Route: route},
			conversation.FunctionCall{CallID: "call_1", Name: "weather", Arguments: `{"city":"Paris"}`, ReasoningEnvelopeID: "env_1"},
			conversation.FunctionOutput{CallID: "call_1", Output: json.RawMessage(`"sunny"`)},
		}},
		Tools:      []conversation.FunctionTool{{Name: "weather", Parameters: json.RawMessage(`{"type":"object"}`), Strict: true}},
		ToolChoice: json.RawMessage(`"auto"`),
		Extra: map[string]json.RawMessage{
			"temperature": json.RawMessage(`0.2`), "include": json.RawMessage(`["reasoning.encrypted_content"]`),
			"store": json.RawMessage(`false`), "text": json.RawMessage(`{"format":{"type":"text"}}`),
			"client_metadata": json.RawMessage(`{"x":1}`), "prompt_cache_key": json.RawMessage(`"cache"`),
			"reasoning": json.RawMessage(`{"effort":"medium"}`),
		},
	}}
	got, err := NewDialectWithIDGenerator(func() (string, error) { return "env_new", nil }).BuildChatRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	if got.Model != "deepseek-chat" || !got.Stream || len(got.Messages) != 4 || got.Temperature == nil || *got.Temperature != 0.2 {
		t.Fatalf("request=%#v", got)
	}
	assistant := got.Messages[2]
	if assistant.Role != "assistant" || string(assistant.Content) != `""` || string(assistant.Extra["reasoning_content"]) != `"private reasoning"` || !strings.Contains(string(assistant.Extra["tool_calls"]), `"id":"call_1"`) {
		t.Fatalf("assistant=%#v", assistant)
	}
	if got.Messages[3].Role != "tool" || string(got.Messages[3].Extra["tool_call_id"]) != `"call_1"` {
		t.Fatalf("tool=%#v", got.Messages[3])
	}
	if string(got.Extra["thinking"]) != `{"type":"enabled"}` || string(got.Extra["reasoning_effort"]) != `"high"` {
		t.Fatalf("extra=%#v", got.Extra)
	}
	for _, key := range []string{"tool_choice", "include", "store", "text", "client_metadata", "prompt_cache_key", "reasoning"} {
		if _, exists := got.Extra[key]; exists {
			t.Fatalf("responses-only field %q leaked: %#v", key, got.Extra)
		}
	}
}

func TestDialectRejectsMissingReasoningAndRouteMismatch(t *testing.T) {
	dialect := NewDialectWithIDGenerator(func() (string, error) { return "env", nil })
	route := conversation.RouteBinding{Dialect: "deepseek", Provider: "one", UpstreamModel: "model"}
	tests := []struct {
		name  string
		items []conversation.Item
		want  error
	}{
		{name: "missing reasoning", items: []conversation.Item{
			conversation.FunctionCall{CallID: "call_1", Name: "f", Arguments: `{}`},
		}, want: ErrReasoningRequired},
		{name: "route mismatch", items: []conversation.Item{
			conversation.Reasoning{EnvelopeID: "env_1", Content: "private", CallIDs: []string{"call_1"}, Route: conversation.RouteBinding{Dialect: "deepseek", Provider: "two", UpstreamModel: "model"}},
			conversation.FunctionCall{CallID: "call_1", Name: "f", Arguments: `{}`, ReasoningEnvelopeID: "env_1"},
		}, want: ErrReasoningRouteMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := dialect.BuildChatRequest(providerdialect.Request{UpstreamModel: "model", Route: route, Conversation: conversation.Request{Model: "external", Turn: conversation.Turn{Items: tt.items}}})
			if !errors.Is(err, tt.want) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestDialectParsesReasoningToolResponse(t *testing.T) {
	dialect := NewDialectWithIDGenerator(func() (string, error) { return "env_new", nil })
	response := compat.ChatCompletionResponse{Choices: []compat.ChatCompletionChoice{{Index: 0, Message: compat.ChatMessage{
		Role: "assistant", Content: json.RawMessage(`""`), Extra: map[string]json.RawMessage{
			"reasoning_content": json.RawMessage(`"private reasoning"`),
			"tool_calls":        json.RawMessage(`[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}},{"id":"call_2","type":"function","function":{"name":"lookup","arguments":"{\"q\":2}"}}]`),
		},
	}, FinishReason: "tool_calls"}}, Usage: &compat.Usage{TotalTokens: 9}}
	got, err := dialect.ParseChatResponse(response)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Turn.Items) != 3 {
		t.Fatalf("turn=%#v", got.Turn)
	}
	reasoning := got.Turn.Items[0].(conversation.Reasoning)
	if reasoning.EnvelopeID != "env_new" || reasoning.Content != "private reasoning" || len(reasoning.CallIDs) != 2 || reasoning.CallIDs[1] != "call_2" || reasoning.AssistantContent != "" {
		t.Fatalf("reasoning=%#v", reasoning)
	}
	for _, raw := range got.Turn.Items[1:] {
		if raw.(conversation.FunctionCall).ReasoningEnvelopeID != "env_new" {
			t.Fatalf("call=%#v", raw)
		}
	}
}

func TestDialectRejectsToolResponseWithoutReasoning(t *testing.T) {
	response := compat.ChatCompletionResponse{
		Choices: []compat.ChatCompletionChoice{{
			Index: 0,
			Message: compat.ChatMessage{
				Role: "assistant", Content: json.RawMessage(`""`),
				Extra: map[string]json.RawMessage{
					"tool_calls": json.RawMessage(`[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]`),
				},
			},
		}},
	}
	_, err := NewDialect().ParseChatResponse(response)
	if !errors.Is(err, ErrIncompleteReasoningToolCall) || err.Error() != "provider returned an incomplete reasoning tool call" {
		t.Fatalf("error=%v", err)
	}
}

func TestDialectDeclaresReasoningReplayCapability(t *testing.T) {
	dialect := NewDialect()
	if dialect.Name() != "deepseek" || !dialect.Capabilities().ReasoningReplay {
		t.Fatalf("name=%q capabilities=%#v", dialect.Name(), dialect.Capabilities())
	}
}
