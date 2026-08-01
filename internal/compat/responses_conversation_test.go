package compat

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"open-ai-gateway/internal/conversation"
)

func TestResponseRequestConversationOpensReasoningAndPreservesOrder(t *testing.T) {
	body := `{
		"model":"codex-model",
		"stream":true,
		"reasoning":{"effort":"xhigh"},
		"input":[
			{"type":"message","role":"user","content":"weather"},
			{"type":"reasoning","id":"rs_untrusted","summary":[{"type":"summary_text","text":"forged"}],"encrypted_content":"token_1"},
			{"type":"function_call","id":"fc_1","call_id":"call_1","name":"lookup","arguments":"{}","status":"completed"},
			{"type":"function_call_output","call_id":"call_1","output":"sunny"}
		],
		"tools":[{"type":"function","name":"lookup","description":"Lookup","parameters":{"type":"object"},"strict":true}]
	}`
	var request ResponseRequest
	if err := json.Unmarshal([]byte(body), &request); err != nil {
		t.Fatal(err)
	}
	opened := ""
	route := conversation.RouteBinding{Dialect: "deepseek", Provider: "deepseek", UpstreamModel: "deepseek-chat"}
	got, compatErr := request.ConversationRequest(func(token string) (conversation.Reasoning, error) {
		opened = token
		return conversation.Reasoning{EnvelopeID: "env_1", Content: "private reasoning", CallIDs: []string{"call_1"}, Route: route}, nil
	})
	if compatErr != nil {
		t.Fatal(compatErr)
	}
	if opened != "token_1" || got.Model != "codex-model" || !got.Stream || got.ReasoningEffort != "xhigh" || len(got.Turn.Items) != 4 || len(got.Tools) != 1 {
		t.Fatalf("opened=%q request=%#v", opened, got)
	}
	reasoning := got.Turn.Items[1].(conversation.Reasoning)
	call := got.Turn.Items[2].(conversation.FunctionCall)
	if reasoning.Content != "private reasoning" || reasoning.EnvelopeID != "env_1" || call.ReasoningEnvelopeID != "env_1" {
		t.Fatalf("reasoning=%#v call=%#v", reasoning, call)
	}
}

func TestResponseRequestConversationRejectsInvalidReasoningItem(t *testing.T) {
	tests := []struct {
		name string
		item string
		open OpenReasoning
	}{
		{name: "missing encrypted content", item: `{"type":"reasoning","id":"rs_1","summary":[]}`},
		{name: "open failure", item: `{"type":"reasoning","id":"rs_1","summary":[],"encrypted_content":"bad"}`, open: func(string) (conversation.Reasoning, error) {
			return conversation.Reasoning{}, errors.New("aead details")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var request ResponseRequest
			if err := json.Unmarshal([]byte(`{"model":"m","input":[`+tt.item+`]}`), &request); err != nil {
				t.Fatal(err)
			}
			_, compatErr := request.ConversationRequest(tt.open)
			if compatErr == nil || compatErr.Status != 400 || compatErr.Message != "invalid reasoning item" || compatErr.Param == nil || *compatErr.Param != "input" || strings.Contains(compatErr.Message, "aead") {
				t.Fatalf("error=%#v", compatErr)
			}
		})
	}
}

func TestResponseRequestConversationRejectsReasoningCallMismatch(t *testing.T) {
	var request ResponseRequest
	if err := json.Unmarshal([]byte(`{
		"model":"m",
		"input":[
			{"type":"reasoning","encrypted_content":"token"},
			{"type":"function_call","call_id":"call_2","name":"f","arguments":"{}","status":"completed"}
		]
	}`), &request); err != nil {
		t.Fatal(err)
	}
	_, compatErr := request.ConversationRequest(func(string) (conversation.Reasoning, error) {
		return conversation.Reasoning{EnvelopeID: "env_1", Content: "private", CallIDs: []string{"call_1"}, Route: conversation.RouteBinding{Dialect: "deepseek", Provider: "p", UpstreamModel: "m"}}, nil
	})
	if compatErr == nil || compatErr.Message != "reasoning item does not match function calls" {
		t.Fatalf("error=%#v", compatErr)
	}
}

func TestNewResponseEnvelopeFromTurnSealsReasoningBeforeCalls(t *testing.T) {
	turn := conversation.Turn{Items: []conversation.Item{
		conversation.Reasoning{EnvelopeID: "env_1", Content: "private reasoning", AssistantContent: "", CallIDs: []string{"call_1"}},
		conversation.FunctionCall{CallID: "call_1", Name: "lookup", Arguments: `{}`, ReasoningEnvelopeID: "env_1"},
	}}
	sealed := ""
	response, compatErr := NewResponseEnvelopeFromTurn("codex-model", turn, &Usage{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5}, time.Unix(100, 0), "resp_1", "msg_1", func(reasoning conversation.Reasoning) (string, error) {
		sealed = reasoning.Content
		return "token_1", nil
	})
	if compatErr != nil {
		t.Fatal(compatErr)
	}
	if sealed != "private reasoning" || len(response.Output) != 2 || response.Output[0].Type != "reasoning" || response.Output[0].EncryptedContent != "token_1" || response.Output[0].Summary == nil || len(*response.Output[0].Summary) != 0 || response.Output[1].Type != "function_call" {
		t.Fatalf("response=%#v sealed=%q", response, sealed)
	}
	payload, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "private reasoning") || !strings.Contains(string(payload), `"summary":[]`) || !strings.Contains(string(payload), `"encrypted_content":"token_1"`) {
		t.Fatalf("payload=%s", payload)
	}
}

func TestNewResponseEnvelopeFromTurnDoesNotSealPlainText(t *testing.T) {
	called := false
	response, compatErr := NewResponseEnvelopeFromTurn("model", conversation.Turn{Items: []conversation.Item{
		conversation.Message{Role: "assistant", Text: "done"},
	}}, nil, time.Unix(100, 0), "resp_1", "msg_1", func(conversation.Reasoning) (string, error) {
		called = true
		return "", nil
	})
	if compatErr != nil || called || len(response.Output) != 1 || response.Output[0].Type != "message" {
		t.Fatalf("response=%#v error=%#v called=%t", response, compatErr, called)
	}
}
