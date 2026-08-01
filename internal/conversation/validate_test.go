package conversation

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateRequestAcceptsReasoningToolTurnAndPinsRoute(t *testing.T) {
	route := RouteBinding{Dialect: "deepseek", Provider: "deepseek", UpstreamModel: "deepseek-chat"}
	req := Request{Model: "codex-model", Turn: Turn{Items: []Item{
		Message{Role: "user", Text: "weather"},
		Reasoning{EnvelopeID: "env_1", Content: "private", CallIDs: []string{"call_1", "call_2"}, Route: route},
		FunctionCall{CallID: "call_1", Name: "weather", Arguments: `{"city":"Paris"}`, ReasoningEnvelopeID: "env_1"},
		FunctionCall{CallID: "call_2", Name: "weather", Arguments: `{"city":"Tokyo"}`, ReasoningEnvelopeID: "env_1"},
		FunctionOutput{CallID: "call_1", Output: json.RawMessage(`"sunny"`)},
		FunctionOutput{CallID: "call_2", Output: json.RawMessage(`"rain"`)},
	}}}
	if err := ValidateRequest(req, testLimits()); err != nil {
		t.Fatal(err)
	}
	got, ok, err := req.PinnedRoute()
	if err != nil || !ok || got != route {
		t.Fatalf("route=%#v ok=%t err=%v", got, ok, err)
	}
}

func TestValidateRequestAcceptsOrdinaryFunctionCallWithoutReasoning(t *testing.T) {
	req := Request{Model: "model", Turn: Turn{Items: []Item{
		FunctionCall{CallID: "call_1", Name: "lookup", Arguments: `{}`},
		FunctionOutput{CallID: "call_1", Output: json.RawMessage(`{"ok":true}`)},
	}}}
	if err := ValidateRequest(req, testLimits()); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := req.PinnedRoute(); err != nil || ok {
		t.Fatalf("ok=%t err=%v", ok, err)
	}
}

func TestValidateRequestRejectsInvalidCorrelationAndLimits(t *testing.T) {
	route := RouteBinding{Dialect: "deepseek", Provider: "deepseek", UpstreamModel: "deepseek-chat"}
	tests := []struct {
		name   string
		req    Request
		limits Limits
		want   string
	}{
		{name: "duplicate call", req: requestWithItems(
			FunctionCall{CallID: "call_1", Name: "f", Arguments: `{}`},
			FunctionCall{CallID: "call_1", Name: "f", Arguments: `{}`},
		), limits: testLimits(), want: "duplicate function call"},
		{name: "duplicate output", req: requestWithItems(
			FunctionCall{CallID: "call_1", Name: "f", Arguments: `{}`},
			FunctionOutput{CallID: "call_1", Output: json.RawMessage(`1`)},
			FunctionOutput{CallID: "call_1", Output: json.RawMessage(`2`)},
		), limits: testLimits(), want: "duplicate function output"},
		{name: "unknown output", req: requestWithItems(
			FunctionOutput{CallID: "call_1", Output: json.RawMessage(`1`)},
		), limits: testLimits(), want: "unknown function call"},
		{name: "output before call", req: requestWithItems(
			FunctionOutput{CallID: "call_1", Output: json.RawMessage(`1`)},
			FunctionCall{CallID: "call_1", Name: "f", Arguments: `{}`},
		), limits: testLimits(), want: "unknown function call"},
		{name: "incomplete reasoning calls", req: requestWithItems(
			Reasoning{EnvelopeID: "env_1", Content: "private", CallIDs: []string{"call_1", "call_2"}, Route: route},
			FunctionCall{CallID: "call_1", Name: "f", Arguments: `{}`, ReasoningEnvelopeID: "env_1"},
		), limits: testLimits(), want: "does not match function calls"},
		{name: "wrong reasoning owner", req: requestWithItems(
			Reasoning{EnvelopeID: "env_1", Content: "private", CallIDs: []string{"call_1"}, Route: route},
			FunctionCall{CallID: "call_1", Name: "f", Arguments: `{}`, ReasoningEnvelopeID: "env_2"},
		), limits: testLimits(), want: "does not match function calls"},
		{name: "reordered reasoning calls", req: requestWithItems(
			Reasoning{EnvelopeID: "env_1", Content: "private", CallIDs: []string{"call_1", "call_2"}, Route: route},
			FunctionCall{CallID: "call_2", Name: "f", Arguments: `{}`, ReasoningEnvelopeID: "env_1"},
			FunctionCall{CallID: "call_1", Name: "f", Arguments: `{}`, ReasoningEnvelopeID: "env_1"},
		), limits: testLimits(), want: "does not match function calls"},
		{name: "orphan reasoning owner", req: requestWithItems(
			FunctionCall{CallID: "call_1", Name: "f", Arguments: `{}`, ReasoningEnvelopeID: "env_1"},
		), limits: testLimits(), want: "reasoning owner"},
		{name: "item limit", req: requestWithItems(
			Message{Role: "user", Text: "one"}, Message{Role: "user", Text: "two"},
		), limits: Limits{MaxItems: 1, MaxToolCallsPerTurn: 4, MaxReasoningBytes: 100}, want: "too many conversation items"},
		{name: "tool call limit", req: requestWithItems(
			Reasoning{EnvelopeID: "env_1", Content: "private", CallIDs: []string{"call_1", "call_2"}, Route: route},
			FunctionCall{CallID: "call_1", Name: "f", Arguments: `{}`, ReasoningEnvelopeID: "env_1"},
			FunctionCall{CallID: "call_2", Name: "f", Arguments: `{}`, ReasoningEnvelopeID: "env_1"},
		), limits: Limits{MaxItems: 10, MaxToolCallsPerTurn: 1, MaxReasoningBytes: 100}, want: "too many tool calls"},
		{name: "reasoning byte limit", req: requestWithItems(
			Reasoning{EnvelopeID: "env_1", Content: "private", Route: route},
		), limits: Limits{MaxItems: 10, MaxToolCallsPerTurn: 4, MaxReasoningBytes: 3}, want: "reasoning content exceeds limit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRequest(tt.req, tt.limits)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error=%v want substring %q", err, tt.want)
			}
		})
	}
}

func TestPinnedRouteRejectsMixedReasoningRoutes(t *testing.T) {
	first := RouteBinding{Dialect: "deepseek", Provider: "one", UpstreamModel: "model"}
	second := RouteBinding{Dialect: "deepseek", Provider: "two", UpstreamModel: "model"}
	req := requestWithItems(
		Reasoning{EnvelopeID: "env_1", Content: "a", CallIDs: []string{"call_1"}, Route: first},
		FunctionCall{CallID: "call_1", Name: "f", Arguments: `{}`, ReasoningEnvelopeID: "env_1"},
		FunctionOutput{CallID: "call_1", Output: json.RawMessage(`1`)},
		Reasoning{EnvelopeID: "env_2", Content: "b", CallIDs: []string{"call_2"}, Route: second},
		FunctionCall{CallID: "call_2", Name: "f", Arguments: `{}`, ReasoningEnvelopeID: "env_2"},
	)
	if err := ValidateRequest(req, testLimits()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := req.PinnedRoute(); err == nil || !strings.Contains(err.Error(), "multiple reasoning routes") {
		t.Fatalf("error=%v", err)
	}
}

func TestCloneRequestDeepCopiesMutableData(t *testing.T) {
	callIDs := []string{"call_1"}
	output := json.RawMessage(`{"ok":true}`)
	parameters := json.RawMessage(`{"type":"object"}`)
	extra := json.RawMessage(`{"trace":"one"}`)
	req := Request{
		Model: "model",
		Turn: Turn{Items: []Item{
			Reasoning{EnvelopeID: "env_1", Content: "private", CallIDs: callIDs, Route: RouteBinding{Dialect: "deepseek", Provider: "p", UpstreamModel: "m"}},
			FunctionCall{CallID: "call_1", Name: "f", Arguments: `{}`, ReasoningEnvelopeID: "env_1"},
			FunctionOutput{CallID: "call_1", Output: output},
			Opaque{Kind: "dialect", Data: json.RawMessage(`{"x":1}`)},
		}},
		Tools:      []FunctionTool{{Name: "f", Parameters: parameters}},
		ToolChoice: json.RawMessage(`"auto"`),
		Extra:      map[string]json.RawMessage{"metadata": extra},
	}
	clone := CloneRequest(req)
	callIDs[0] = "mutated"
	output[2] = 'x'
	parameters[2] = 'x'
	extra[2] = 'x'
	req.ToolChoice[1] = 'x'
	req.Turn.Items[3].(Opaque).Data[2] = 'x'

	reasoning := clone.Turn.Items[0].(Reasoning)
	functionOutput := clone.Turn.Items[2].(FunctionOutput)
	opaque := clone.Turn.Items[3].(Opaque)
	if reasoning.CallIDs[0] != "call_1" || string(functionOutput.Output) != `{"ok":true}` || string(clone.Tools[0].Parameters) != `{"type":"object"}` || string(clone.Extra["metadata"]) != `{"trace":"one"}` || string(clone.ToolChoice) != `"auto"` || string(opaque.Data) != `{"x":1}` {
		t.Fatalf("clone=%#v", clone)
	}
}

func requestWithItems(items ...Item) Request {
	return Request{Model: "model", Turn: Turn{Items: items}}
}

func testLimits() Limits {
	return Limits{MaxItems: 100, MaxToolCallsPerTurn: 16, MaxReasoningBytes: 1024}
}
