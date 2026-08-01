package compat

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"open-ai-gateway/internal/conversation"
)

type OpenReasoning func(token string) (conversation.Reasoning, error)

type SealReasoning func(reasoning conversation.Reasoning) (string, error)

func (r ResponseRequest) ConversationRequest(open OpenReasoning) (conversation.Request, *Error) {
	if err := r.Validate(); err != nil {
		return conversation.Request{}, err
	}
	request := conversation.Request{
		Model:  r.Model,
		Stream: r.Stream,
		Extra:  responseConversationExtra(r.Extra),
	}
	if r.ParallelToolCalls != nil {
		if request.Extra == nil {
			request.Extra = make(map[string]json.RawMessage)
		}
		request.Extra["parallel_tool_calls"], _ = json.Marshal(*r.ParallelToolCalls)
	}
	if raw := r.Extra["reasoning"]; len(raw) > 0 {
		var value struct {
			Effort string `json:"effort"`
		}
		if json.Unmarshal(raw, &value) != nil {
			return conversation.Request{}, InvalidRequest("invalid reasoning configuration", "reasoning")
		}
		request.ReasoningEffort = value.Effort
	}
	if r.Instructions != "" {
		request.Turn.Items = append(request.Turn.Items, conversation.Message{Role: "system", Text: r.Instructions})
	}
	tools, toolChoice, toolErr := r.conversationTools()
	if toolErr != nil {
		return conversation.Request{}, toolErr
	}
	request.Tools = tools
	request.ToolChoice = toolChoice
	var input string
	if json.Unmarshal(r.Input, &input) == nil {
		if strings.TrimSpace(input) == "" {
			return conversation.Request{}, InvalidRequest("input text is required", "input")
		}
		request.Turn.Items = append(request.Turn.Items, conversation.Message{Role: "user", Text: input})
		return request, nil
	}
	var items []json.RawMessage
	if json.Unmarshal(r.Input, &items) != nil || len(items) == 0 {
		return conversation.Request{}, InvalidRequest("input must be text or a non-empty message array", "input")
	}
	owners := make(map[string]string)
	for index, raw := range items {
		var header struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(raw, &header) != nil {
			return conversation.Request{}, InvalidRequest(fmt.Sprintf("invalid input item at index %d", index), "input")
		}
		switch header.Type {
		case "reasoning":
			var item struct {
				EncryptedContent string `json:"encrypted_content"`
			}
			if json.Unmarshal(raw, &item) != nil || strings.TrimSpace(item.EncryptedContent) == "" || open == nil {
				return conversation.Request{}, InvalidRequest("invalid reasoning item", "input")
			}
			reasoning, err := open(item.EncryptedContent)
			if err != nil || strings.TrimSpace(reasoning.EnvelopeID) == "" {
				return conversation.Request{}, InvalidRequest("invalid reasoning item", "input")
			}
			request.Turn.Items = append(request.Turn.Items, reasoning)
			for _, callID := range reasoning.CallIDs {
				owners[callID] = reasoning.EnvelopeID
			}
		case "function_call":
			var item responseFunctionCallInput
			if json.Unmarshal(raw, &item) != nil || strings.TrimSpace(item.CallID) == "" || strings.TrimSpace(item.Name) == "" || !ValidJSONString(item.Arguments) || (item.Status != "" && item.Status != "completed") {
				return conversation.Request{}, InvalidRequest(fmt.Sprintf("invalid function_call at input index %d", index), "input")
			}
			request.Turn.Items = append(request.Turn.Items, conversation.FunctionCall{CallID: item.CallID, Name: item.Name, Arguments: item.Arguments, ReasoningEnvelopeID: owners[item.CallID]})
		case "function_call_output":
			var item responseFunctionCallOutputInput
			if json.Unmarshal(raw, &item) != nil || strings.TrimSpace(item.CallID) == "" || item.Output == nil {
				return conversation.Request{}, InvalidRequest(fmt.Sprintf("invalid function_call_output at input index %d", index), "input")
			}
			output, _ := json.Marshal(*item.Output)
			request.Turn.Items = append(request.Turn.Items, conversation.FunctionOutput{CallID: item.CallID, Output: output})
		case "additional_tools":
			var item struct {
				Tools json.RawMessage `json:"tools"`
			}
			if json.Unmarshal(raw, &item) != nil {
				return conversation.Request{}, InvalidRequest(fmt.Sprintf("invalid additional_tools at input index %d", index), "input")
			}
			additional, err := parseConversationTools(item.Tools)
			if err != nil {
				return conversation.Request{}, InvalidRequest("invalid function tool", "input")
			}
			request.Tools = append(request.Tools, additional...)
		default:
			if header.Type != "" && header.Type != "message" {
				return conversation.Request{}, InvalidRequest(fmt.Sprintf("unsupported input item type at index %d", index), "input")
			}
			var item responseInputMessage
			if json.Unmarshal(raw, &item) != nil {
				return conversation.Request{}, InvalidRequest(fmt.Sprintf("invalid input item at index %d", index), "input")
			}
			role := strings.TrimSpace(item.Role)
			if role == "developer" {
				role = "system"
			}
			if role != "user" && role != "assistant" && role != "system" {
				return conversation.Request{}, InvalidRequest(fmt.Sprintf("invalid input role at index %d", index), "input")
			}
			text, err := responseMessageText(item.Content)
			if err != nil {
				return conversation.Request{}, InvalidRequest(fmt.Sprintf("invalid text content at input index %d", index), "input")
			}
			request.Turn.Items = append(request.Turn.Items, conversation.Message{Role: role, Text: text})
		}
	}
	if err := validateUniqueTools(request.Tools); err != nil {
		return conversation.Request{}, InvalidRequest(err.Error(), "tools")
	}
	if !validReasoningCallSequence(request.Turn.Items) {
		return conversation.Request{}, InvalidRequest("reasoning item does not match function calls", "input")
	}
	return request, nil
}

func validReasoningCallSequence(items []conversation.Item) bool {
	for index, raw := range items {
		reasoning, ok := raw.(conversation.Reasoning)
		if !ok || len(reasoning.CallIDs) == 0 {
			continue
		}
		callStart := index + 1
		if callStart < len(items) {
			if message, ok := items[callStart].(conversation.Message); ok && message.Role == "assistant" && message.Text == reasoning.AssistantContent && message.Text != "" {
				callStart++
			}
		}
		if len(items)-callStart < len(reasoning.CallIDs) {
			return false
		}
		declared := make(map[string]struct{}, len(reasoning.CallIDs))
		for _, callID := range reasoning.CallIDs {
			if _, exists := declared[callID]; exists {
				return false
			}
			declared[callID] = struct{}{}
		}
		for offset := 0; offset < len(reasoning.CallIDs); offset++ {
			call, ok := items[callStart+offset].(conversation.FunctionCall)
			if !ok || call.ReasoningEnvelopeID != reasoning.EnvelopeID || call.CallID != reasoning.CallIDs[offset] {
				return false
			}
			delete(declared, call.CallID)
		}
		if len(declared) != 0 {
			return false
		}
	}
	return true
}

func (r ResponseRequest) conversationTools() ([]conversation.FunctionTool, json.RawMessage, *Error) {
	tools, err := parseConversationTools(r.Tools)
	if err != nil {
		return nil, nil, InvalidRequest("invalid function tool", "tools")
	}
	_, _, compatErr := r.chatToolFields()
	if compatErr != nil {
		return nil, nil, compatErr
	}
	var choice json.RawMessage
	if len(r.ToolChoice) > 0 && len(tools) > 0 {
		var value string
		if json.Unmarshal(r.ToolChoice, &value) == nil {
			choice = cloneRawMessage(r.ToolChoice)
		} else {
			var forced struct{ Type, Name string }
			if json.Unmarshal(r.ToolChoice, &forced) != nil {
				return nil, nil, InvalidRequest("unsupported tool_choice", "tool_choice")
			}
			choice, _ = json.Marshal(map[string]any{"type": "function", "function": map[string]string{"name": forced.Name}})
		}
	}
	return tools, choice, nil
}

func parseConversationTools(raw json.RawMessage) ([]conversation.FunctionTool, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var values []responseFunctionTool
	if json.Unmarshal(raw, &values) != nil || len(values) == 0 {
		return nil, fmt.Errorf("tools must be a non-empty array")
	}
	tools := make([]conversation.FunctionTool, 0, len(values))
	for _, value := range values {
		var parameters map[string]any
		if value.Type != "function" || strings.TrimSpace(value.Name) == "" || json.Unmarshal(value.Parameters, &parameters) != nil || parameters == nil {
			return nil, fmt.Errorf("invalid function tool")
		}
		strict := true
		if value.Strict != nil {
			strict = *value.Strict
		}
		tools = append(tools, conversation.FunctionTool{Name: value.Name, Description: value.Description, Parameters: cloneRawMessage(value.Parameters), Strict: strict})
	}
	return tools, nil
}

func validateUniqueTools(tools []conversation.FunctionTool) error {
	seen := make(map[string]struct{}, len(tools))
	for _, tool := range tools {
		if _, exists := seen[tool.Name]; exists {
			return fmt.Errorf("duplicate function tool %q", tool.Name)
		}
		seen[tool.Name] = struct{}{}
	}
	return nil
}

func responseConversationExtra(values map[string]json.RawMessage) map[string]json.RawMessage {
	result := make(map[string]json.RawMessage)
	for key, value := range values {
		switch key {
		case "include", "text", "client_metadata", "prompt_cache_key", "reasoning":
			continue
		case "max_output_tokens":
			result["max_tokens"] = cloneRawMessage(value)
		default:
			result[key] = cloneRawMessage(value)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func NewResponseEnvelopeFromTurn(externalModel string, turn conversation.Turn, usage *Usage, now time.Time, responseID, messageID string, seal SealReasoning) (*Response, *Error) {
	output := make([]ResponseOutputMessage, 0, len(turn.Items))
	for index, raw := range turn.Items {
		switch item := raw.(type) {
		case conversation.Reasoning:
			if seal == nil {
				return nil, ServerError(500, "failed to seal reasoning item")
			}
			token, err := seal(item)
			if err != nil || token == "" {
				return nil, ServerError(500, "failed to seal reasoning item")
			}
			summary := []ResponseReasoningSummary{}
			output = append(output, ResponseOutputMessage{
				ID: fmt.Sprintf("rs_%s_%d", strings.TrimPrefix(responseID, "resp_"), index), Type: "reasoning",
				Summary: &summary, EncryptedContent: token,
			})
		case conversation.Message:
			if item.Role != "assistant" || item.Text == "" {
				return nil, ServerError(502, "provider returned unsupported response content")
			}
			output = append(output, ResponseOutputMessage{ID: messageID, Type: "message", Status: "completed", Role: "assistant", Content: []ResponseOutputText{{Type: "output_text", Text: item.Text, Annotations: []any{}}}})
		case conversation.FunctionCall:
			output = append(output, ResponseOutputMessage{ID: fmt.Sprintf("fc_%s_%d", strings.TrimPrefix(responseID, "resp_"), index), Type: "function_call", Status: "completed", CallID: item.CallID, Name: item.Name, Arguments: item.Arguments})
		default:
			return nil, ServerError(502, "provider returned unsupported response content")
		}
	}
	if len(output) == 0 {
		return nil, ServerError(502, "provider returned unsupported response content")
	}
	response := &Response{
		ID: responseID, Object: "response", CreatedAt: now.Unix(), Status: "completed",
		Model: externalModel, Output: output, ParallelToolCalls: true, Store: false, Tools: []any{},
	}
	if usage != nil {
		response.Usage = &ResponseUsage{InputTokens: usage.PromptTokens, OutputTokens: usage.CompletionTokens, TotalTokens: usage.TotalTokens}
	}
	return response, nil
}
