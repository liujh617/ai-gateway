package conversation

import (
	"encoding/json"
	"fmt"
	"strings"
)

type callState struct {
	output bool
}

func ValidateRequest(request Request, limits Limits) error {
	if strings.TrimSpace(request.Model) == "" {
		return fmt.Errorf("conversation model is required")
	}
	if limits.MaxItems <= 0 || limits.MaxToolCallsPerTurn <= 0 || limits.MaxReasoningBytes <= 0 {
		return fmt.Errorf("conversation limits must be positive")
	}
	if len(request.Turn.Items) > limits.MaxItems {
		return fmt.Errorf("too many conversation items")
	}
	expectedOwners := make(map[string]string)
	seenEnvelopes := make(map[string]struct{})
	calls := make(map[string]callState)
	for index, raw := range request.Turn.Items {
		switch item := raw.(type) {
		case Message:
			if item.Role != "system" && item.Role != "user" && item.Role != "assistant" {
				return fmt.Errorf("invalid message role at item %d", index)
			}
		case Reasoning:
			if err := validateReasoning(request.Turn.Items, index, item, limits, expectedOwners, seenEnvelopes); err != nil {
				return err
			}
		case FunctionCall:
			callID := strings.TrimSpace(item.CallID)
			if callID == "" || callID != item.CallID || strings.TrimSpace(item.Name) == "" || item.Name != strings.TrimSpace(item.Name) || !json.Valid([]byte(item.Arguments)) {
				return fmt.Errorf("invalid function call at item %d", index)
			}
			if _, exists := calls[callID]; exists {
				return fmt.Errorf("duplicate function call %q", callID)
			}
			owner, owned := expectedOwners[callID]
			if item.ReasoningEnvelopeID != "" {
				if !owned || owner != item.ReasoningEnvelopeID {
					return fmt.Errorf("function call %q has invalid reasoning owner", callID)
				}
			} else if owned {
				return fmt.Errorf("reasoning item does not match function calls")
			}
			calls[callID] = callState{}
		case FunctionOutput:
			callID := strings.TrimSpace(item.CallID)
			state, exists := calls[callID]
			if callID == "" || callID != item.CallID || !exists {
				return fmt.Errorf("function output references unknown function call %q", callID)
			}
			if state.output {
				return fmt.Errorf("duplicate function output for %q", callID)
			}
			if len(item.Output) == 0 || !json.Valid(item.Output) {
				return fmt.Errorf("invalid function output for %q", callID)
			}
			state.output = true
			calls[callID] = state
		case Opaque:
			if strings.TrimSpace(item.Kind) == "" || len(item.Data) == 0 || !json.Valid(item.Data) {
				return fmt.Errorf("invalid opaque item at index %d", index)
			}
		default:
			return fmt.Errorf("unsupported conversation item at index %d", index)
		}
	}
	return validateTools(request.Tools)
}

func validateReasoning(items []Item, index int, item Reasoning, limits Limits, expectedOwners map[string]string, seenEnvelopes map[string]struct{}) error {
	if strings.TrimSpace(item.EnvelopeID) == "" || item.EnvelopeID != strings.TrimSpace(item.EnvelopeID) || item.Content == "" {
		return fmt.Errorf("invalid reasoning item at index %d", index)
	}
	if len(item.Content) > limits.MaxReasoningBytes {
		return fmt.Errorf("reasoning content exceeds limit")
	}
	if _, exists := seenEnvelopes[item.EnvelopeID]; exists {
		return fmt.Errorf("duplicate reasoning envelope %q", item.EnvelopeID)
	}
	seenEnvelopes[item.EnvelopeID] = struct{}{}
	if len(item.CallIDs) == 0 {
		return nil
	}
	if len(item.CallIDs) > limits.MaxToolCallsPerTurn {
		return fmt.Errorf("too many tool calls in reasoning turn")
	}
	if strings.TrimSpace(item.Route.Dialect) == "" || strings.TrimSpace(item.Route.Provider) == "" || strings.TrimSpace(item.Route.UpstreamModel) == "" {
		return fmt.Errorf("reasoning route is required for tool calls")
	}
	declared := make(map[string]struct{}, len(item.CallIDs))
	for _, callID := range item.CallIDs {
		if strings.TrimSpace(callID) == "" || callID != strings.TrimSpace(callID) {
			return fmt.Errorf("reasoning item has invalid call ID")
		}
		if _, exists := declared[callID]; exists {
			return fmt.Errorf("reasoning item has duplicate call ID %q", callID)
		}
		declared[callID] = struct{}{}
	}
	callStart := index + 1
	if callStart < len(items) {
		if message, ok := items[callStart].(Message); ok && message.Role == "assistant" && message.Text == item.AssistantContent && message.Text != "" {
			callStart++
		}
	}
	if len(items)-callStart < len(declared) {
		return fmt.Errorf("reasoning item does not match function calls")
	}
	matched := make(map[string]struct{}, len(declared))
	for offset := 0; offset < len(declared); offset++ {
		call, ok := items[callStart+offset].(FunctionCall)
		if !ok || call.ReasoningEnvelopeID != item.EnvelopeID {
			return fmt.Errorf("reasoning item does not match function calls")
		}
		if _, ok := declared[call.CallID]; !ok {
			return fmt.Errorf("reasoning item does not match function calls")
		}
		if _, duplicate := matched[call.CallID]; duplicate {
			return fmt.Errorf("reasoning item does not match function calls")
		}
		matched[call.CallID] = struct{}{}
	}
	for callID := range declared {
		expectedOwners[callID] = item.EnvelopeID
	}
	return nil
}

func validateTools(tools []FunctionTool) error {
	seen := make(map[string]struct{}, len(tools))
	for index, tool := range tools {
		if strings.TrimSpace(tool.Name) == "" || tool.Name != strings.TrimSpace(tool.Name) {
			return fmt.Errorf("invalid function tool at index %d", index)
		}
		if _, exists := seen[tool.Name]; exists {
			return fmt.Errorf("duplicate function tool %q", tool.Name)
		}
		seen[tool.Name] = struct{}{}
		if len(tool.Parameters) == 0 || !json.Valid(tool.Parameters) {
			return fmt.Errorf("invalid function tool parameters for %q", tool.Name)
		}
	}
	return nil
}

func (request Request) PinnedRoute() (RouteBinding, bool, error) {
	var route RouteBinding
	found := false
	for _, raw := range request.Turn.Items {
		item, ok := raw.(Reasoning)
		if !ok || len(item.CallIDs) == 0 {
			continue
		}
		if !found {
			route = item.Route
			found = true
			continue
		}
		if route != item.Route {
			return RouteBinding{}, false, fmt.Errorf("multiple reasoning routes are active")
		}
	}
	return route, found, nil
}
