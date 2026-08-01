package compat

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type ResponseRequest struct {
	Model                 string
	Input                 json.RawMessage
	Instructions          string
	Stream                bool
	Store                 *bool
	Tools                 json.RawMessage
	ToolChoice            json.RawMessage
	ParallelToolCalls     *bool
	PreviousResponseID    string
	Conversation          json.RawMessage
	Extra                 map[string]json.RawMessage
	previousResponseIDSet bool
	conversationSet       bool
}

func (r *ResponseRequest) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	if raw, ok := fields["model"]; ok {
		if err := json.Unmarshal(raw, &r.Model); err != nil {
			return fmt.Errorf("unmarshal: %w", err)
		}
		delete(fields, "model")
	}
	if raw, ok := fields["input"]; ok {
		r.Input = cloneRawMessage(raw)
		delete(fields, "input")
	}
	if raw, ok := fields["instructions"]; ok {
		if err := json.Unmarshal(raw, &r.Instructions); err != nil {
			return fmt.Errorf("unmarshal: %w", err)
		}
		delete(fields, "instructions")
	}
	if raw, ok := fields["stream"]; ok {
		if err := json.Unmarshal(raw, &r.Stream); err != nil {
			return fmt.Errorf("unmarshal: %w", err)
		}
		delete(fields, "stream")
	}
	if raw, ok := fields["store"]; ok {
		var store bool
		if err := json.Unmarshal(raw, &store); err != nil {
			return fmt.Errorf("unmarshal: %w", err)
		}
		r.Store = &store
		delete(fields, "store")
	}
	if raw, ok := fields["previous_response_id"]; ok {
		r.previousResponseIDSet = true
		if string(raw) != "null" {
			if err := json.Unmarshal(raw, &r.PreviousResponseID); err != nil {
				return fmt.Errorf("unmarshal: %w", err)
			}
		}
		delete(fields, "previous_response_id")
	}
	if raw, ok := fields["tools"]; ok {
		r.Tools = cloneRawMessage(raw)
		delete(fields, "tools")
	}
	if raw, ok := fields["tool_choice"]; ok {
		r.ToolChoice = cloneRawMessage(raw)
		delete(fields, "tool_choice")
	}
	if raw, ok := fields["parallel_tool_calls"]; ok {
		var value bool
		if err := json.Unmarshal(raw, &value); err != nil {
			return fmt.Errorf("unmarshal: %w", err)
		}
		r.ParallelToolCalls = &value
		delete(fields, "parallel_tool_calls")
	}
	if raw, ok := fields["conversation"]; ok {
		r.conversationSet = true
		r.Conversation = cloneRawMessage(raw)
		delete(fields, "conversation")
	}
	if len(fields) > 0 {
		r.Extra = fields
	}
	return nil
}

func (r ResponseRequest) MarshalJSON() ([]byte, error) {
	fields := copyRawFields(r.Extra, nil)
	if err := putJSONField(fields, "model", r.Model); err != nil {
		return nil, err
	}
	fields["input"] = cloneRawMessage(r.Input)
	if r.Instructions != "" {
		if err := putJSONField(fields, "instructions", r.Instructions); err != nil {
			return nil, err
		}
	}
	if r.Stream {
		if err := putJSONField(fields, "stream", true); err != nil {
			return nil, err
		}
	}
	if r.Store != nil {
		if err := putJSONField(fields, "store", *r.Store); err != nil {
			return nil, err
		}
	}
	if r.previousResponseIDSet || r.PreviousResponseID != "" {
		if err := putJSONField(fields, "previous_response_id", r.PreviousResponseID); err != nil {
			return nil, err
		}
	}
	if len(r.Tools) > 0 {
		fields["tools"] = cloneRawMessage(r.Tools)
	}
	if len(r.ToolChoice) > 0 {
		fields["tool_choice"] = cloneRawMessage(r.ToolChoice)
	}
	if r.ParallelToolCalls != nil {
		if err := putJSONField(fields, "parallel_tool_calls", *r.ParallelToolCalls); err != nil {
			return nil, err
		}
	}
	if r.conversationSet {
		fields["conversation"] = cloneRawMessage(r.Conversation)
	}
	return json.Marshal(fields)
}

// ConversationID returns the conversation identifier, or empty string if not set.
func (r ResponseRequest) ConversationID() string {
	if !r.conversationSet || len(r.Conversation) == 0 {
		return ""
	}
	var id string
	if json.Unmarshal(r.Conversation, &id) == nil && id != "" {
		return id
	}
	return ""
}

func (r ResponseRequest) Validate() *Error {
	if strings.TrimSpace(r.Model) == "" {
		return InvalidRequest("missing required field: model", "model")
	}
	if r.previousResponseIDSet && strings.TrimSpace(r.PreviousResponseID) == "" {
		return InvalidRequest("previous_response_id must be a non-empty string", "previous_response_id")
	}
	if len(r.Input) == 0 || string(r.Input) == "null" {
		return InvalidRequest("missing required field: input", "input")
	}
	return nil
}

func (r ResponseRequest) ChatRequest() (ChatCompletionRequest, *Error) {
	if err := r.Validate(); err != nil {
		return ChatCompletionRequest{}, err
	}
	extra, _, toolErr := r.chatToolFields()
	if toolErr != nil {
		return ChatCompletionRequest{}, toolErr
	}
	messages := make([]ChatMessage, 0, 2)
	if r.Instructions != "" {
		content, _ := json.Marshal(r.Instructions)
		messages = append(messages, ChatMessage{Role: "system", Content: content})
	}
	var input string
	if err := json.Unmarshal(r.Input, &input); err == nil {
		if strings.TrimSpace(input) == "" {
			return ChatCompletionRequest{}, InvalidRequest("input text is required", "input")
		}
		content, _ := json.Marshal(input)
		messages = append(messages, ChatMessage{Role: "user", Content: content})
		return ChatCompletionRequest{Model: r.Model, Messages: messages, Stream: r.Stream, Extra: extra}, nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(r.Input, &items); err != nil || len(items) == 0 {
		return ChatCompletionRequest{}, InvalidRequest("input must be text or a non-empty message array", "input")
	}
	allTools := []responseFunctionTool{}
	seenCallIDs := map[string]bool{}
	bufferedOutputs := map[string]responseFunctionCallOutputInput{}
	bufferedOutputOrder := []string{}
	for i, raw := range items {
		var header struct {
			Type string `json:"type"`
		}
		_ = json.Unmarshal(raw, &header)
		if header.Type == "additional_tools" || header.Type == "reasoning" {
			if header.Type == "additional_tools" {
				var at struct {
					Tools json.RawMessage `json:"tools"`
				}
				if json.Unmarshal(raw, &at) == nil && len(at.Tools) > 0 {
					var tools []responseFunctionTool
					if json.Unmarshal(at.Tools, &tools) == nil {
						allTools = append(allTools, tools...)
					}
				}
			}
			continue
		}
		if header.Type == "function_call" {
			var item responseFunctionCallInput
			if json.Unmarshal(raw, &item) != nil || strings.TrimSpace(item.CallID) == "" || strings.TrimSpace(item.Name) == "" || !ValidJSONString(item.Arguments) || (item.Status != "" && item.Status != "completed") {
				return ChatCompletionRequest{}, InvalidRequest(fmt.Sprintf("invalid function_call at input index %d", i), "input")
			}
			callID := strings.TrimSpace(item.CallID)
			seenCallIDs[callID] = true
			call := map[string]any{"id": callID, "type": "function", "function": map[string]any{"name": item.Name, "arguments": item.Arguments}}
			calls, _ := json.Marshal([]any{call})
			messages = append(messages, ChatMessage{Role: "assistant", Content: json.RawMessage("null"), Extra: map[string]json.RawMessage{"tool_calls": calls}})
			if out, ok := bufferedOutputs[callID]; ok {
				callIDJSON, _ := json.Marshal(callID)
				outputJSON, _ := json.Marshal(*out.Output)
				messages = append(messages, ChatMessage{Role: "tool", Content: outputJSON, Extra: map[string]json.RawMessage{"tool_call_id": callIDJSON}})
				delete(bufferedOutputs, callID)
			}
			continue
		}
		if header.Type == "function_call_output" {
			var item responseFunctionCallOutputInput
			if json.Unmarshal(raw, &item) != nil || item.Output == nil {
				return ChatCompletionRequest{}, InvalidRequest(fmt.Sprintf("invalid function_call_output at input index %d", i), "input")
			}
			callID := strings.TrimSpace(item.CallID)
			if callID == "" {
				return ChatCompletionRequest{}, InvalidRequest(fmt.Sprintf("invalid function_call_output at input index %d", i), "input")
			}
			callIDJSON, _ := json.Marshal(callID)
			outputJSON, _ := json.Marshal(*item.Output)
			toolMsg := ChatMessage{Role: "tool", Content: outputJSON, Extra: map[string]json.RawMessage{"tool_call_id": callIDJSON}}
			if seenCallIDs[callID] {
				messages = append(messages, toolMsg)
			} else {
				if _, exists := bufferedOutputs[callID]; !exists {
					bufferedOutputOrder = append(bufferedOutputOrder, callID)
				}
				bufferedOutputs[callID] = item
			}
			continue
		}
		var item responseInputMessage
		if err := json.Unmarshal(raw, &item); err != nil {
			return ChatCompletionRequest{}, InvalidRequest(fmt.Sprintf("invalid input item at index %d", i), "input")
		}
		role := strings.TrimSpace(item.Role)
		if role == "developer" {
			role = "system"
		}
		if role != "user" && role != "assistant" && role != "system" {
			return ChatCompletionRequest{}, InvalidRequest(fmt.Sprintf("invalid input role at index %d", i), "input")
		}
		text, err := responseMessageText(item.Content)
		if err != nil {
			return ChatCompletionRequest{}, InvalidRequest(fmt.Sprintf("invalid text content at input idx %d: err=%s raw=%s", i, err.Error(), string(item.Content)), "input")
		}
		if text == "" {
			continue
		}
		content, _ := json.Marshal(text)
		messages = append(messages, ChatMessage{Role: role, Content: content})
	}
	if len(bufferedOutputs) > 0 {
		if r.PreviousResponseID == "" && r.ConversationID() == "" {
			return ChatCompletionRequest{}, InvalidRequest("function_call_output references an unknown call", "input")
		}
		for _, callID := range bufferedOutputOrder {
			out, ok := bufferedOutputs[callID]
			if !ok {
				continue
			}
			callIDJSON, _ := json.Marshal(callID)
			outputJSON, _ := json.Marshal(*out.Output)
			messages = append(messages, ChatMessage{Role: "tool", Content: outputJSON, Extra: map[string]json.RawMessage{"tool_call_id": callIDJSON}})
		}
	}
	// Merge tools from additional_tools input items.
	if len(allTools) > 0 {
		var merged []any
		if raw, ok := extra["tools"]; ok && len(raw) > 0 {
			json.Unmarshal(raw, &merged)
		}
		for _, t := range allTools {
			name := strings.TrimSpace(t.Name)
			if name == "" || name != t.Name {
				continue
			}
			params := t.Parameters
			if params == nil {
				params = json.RawMessage(`{"type":"object"}`)
			}
			merged = append(merged, map[string]any{"type": "function", "function": map[string]any{"name": name, "description": t.Description, "parameters": params, "strict": true}})
		}
		if len(merged) > 0 {
			extra["tools"], _ = json.Marshal(merged)
		}
	}
	return ChatCompletionRequest{Model: r.Model, Messages: messages, Stream: r.Stream, Extra: extra}, nil
}

type responseFunctionTool struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
	Strict      *bool           `json:"strict"`
}
type responseFunctionCallInput struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Status    string `json:"status"`
}
type responseFunctionCallOutputInput struct {
	Type   string  `json:"type"`
	CallID string  `json:"call_id"`
	Output *string `json:"output"`
}

func (r ResponseRequest) chatToolFields() (map[string]json.RawMessage, map[string]bool, *Error) {
	extra := map[string]json.RawMessage{}
	names := map[string]bool{}
	if len(r.Tools) > 0 {
		var tools []responseFunctionTool
		if json.Unmarshal(r.Tools, &tools) != nil || len(tools) == 0 {
			return nil, nil, InvalidRequest("tools must be a non-empty array", "tools")
		}
		chatTools := make([]any, 0, len(tools))
		for _, tool := range tools {
			name := strings.TrimSpace(tool.Name)
			var parameters map[string]any
			if tool.Type != "function" || name == "" || name != tool.Name || names[name] || json.Unmarshal(tool.Parameters, &parameters) != nil || parameters == nil || !strings.HasPrefix(strings.TrimSpace(string(tool.Parameters)), "{") {
				return nil, nil, InvalidRequest("invalid function tool", "tools")
			}
			names[name] = true
			strict := true
			if tool.Strict != nil {
				strict = *tool.Strict
			}
			function := map[string]any{"name": name, "parameters": parameters, "strict": strict}
			if tool.Description != "" {
				function["description"] = tool.Description
			}
			chatTools = append(chatTools, map[string]any{"type": "function", "function": function})
		}
		extra["tools"], _ = json.Marshal(chatTools)
	}
	if len(r.ToolChoice) > 0 && len(names) > 0 {
		var choice string
		if json.Unmarshal(r.ToolChoice, &choice) == nil {
			if choice != "auto" && choice != "none" && choice != "required" {
				return nil, nil, InvalidRequest("unsupported tool_choice", "tool_choice")
			}
			if len(names) == 0 && choice != "none" {
				return nil, nil, InvalidRequest("tool_choice requires tools", "tool_choice")
			}
			extra["tool_choice"] = cloneRawMessage(r.ToolChoice)
		} else {
			var forced struct{ Type, Name string }
			if json.Unmarshal(r.ToolChoice, &forced) != nil || forced.Type != "function" || !names[forced.Name] {
				return nil, nil, InvalidRequest("unsupported tool_choice", "tool_choice")
			}
			extra["tool_choice"], _ = json.Marshal(map[string]any{"type": "function", "function": map[string]string{"name": forced.Name}})
		}
	}
	if r.ParallelToolCalls != nil {
		extra["parallel_tool_calls"], _ = json.Marshal(*r.ParallelToolCalls)
	}
	if len(extra) == 0 {
		extra = nil
	}
	return extra, names, nil
}

func ValidJSONString(value string) bool {
	var raw any
	return json.Unmarshal([]byte(value), &raw) == nil
}

type responseInputMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type responseInputText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func responseMessageText(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if strings.TrimSpace(text) == "" {
			return "", fmt.Errorf("empty text")
		}
		return text, nil
	}
	// Array of content parts, e.g. [{"type":"input_text","text":"hello"}].
	var parts []responseInputText
	if err := json.Unmarshal(raw, &parts); err == nil && len(parts) > 0 {
		var out strings.Builder
		for _, part := range parts {
			if (part.Type != "input_text" && part.Type != "output_text") || strings.TrimSpace(part.Text) == "" {
				return "", fmt.Errorf("unsupported content part")
			}
			out.WriteString(part.Text)
		}
		return out.String(), nil
	}
	// Single content part object, e.g. {"type":"input_text","text":"hello"}.
	var single responseInputText
	if err := json.Unmarshal(raw, &single); err == nil && (single.Type == "input_text" || single.Type == "output_text") && strings.TrimSpace(single.Text) != "" {
		return single.Text, nil
	}
	return "", fmt.Errorf("invalid content")
}

type Response struct {
	ID                 string                  `json:"id"`
	Object             string                  `json:"object"`
	CreatedAt          int64                   `json:"created_at"`
	Status             string                  `json:"status"`
	Error              any                     `json:"error"`
	IncompleteDetails  any                     `json:"incomplete_details"`
	Instructions       any                     `json:"instructions"`
	Model              string                  `json:"model"`
	Output             []ResponseOutputMessage `json:"output"`
	ParallelToolCalls  bool                    `json:"parallel_tool_calls"`
	PreviousResponseID any                     `json:"previous_response_id"`
	ConversationID     string                  `json:"conversation_id,omitempty"`
	Store              bool                    `json:"store"`
	Tools              []any                   `json:"tools"`
	Usage              *ResponseUsage          `json:"usage"`
}

type DeletedResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Deleted bool   `json:"deleted"`
}

type ResponseOutputMessage struct {
	ID               string                      `json:"id"`
	Type             string                      `json:"type"`
	Status           string                      `json:"status,omitempty"`
	Role             string                      `json:"role,omitempty"`
	Content          []ResponseOutputText        `json:"content,omitempty"`
	CallID           string                      `json:"call_id,omitempty"`
	Name             string                      `json:"name,omitempty"`
	Arguments        string                      `json:"arguments,omitempty"`
	Summary          *[]ResponseReasoningSummary `json:"summary,omitempty"`
	EncryptedContent string                      `json:"encrypted_content,omitempty"`
}

type ResponseReasoningSummary struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ResponseOutputText struct {
	Type        string `json:"type"`
	Text        string `json:"text"`
	Annotations []any  `json:"annotations"`
}

type ResponseUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

func NewResponseEnvelope(externalModel string, chat *ChatCompletionResponse, now time.Time, responseID, messageID string) (*Response, *Error) {
	if chat == nil || len(chat.Choices) != 1 || chat.Choices[0].Index != 0 {
		return nil, ServerError(502, "provider returned unsupported response choices")
	}
	choice := chat.Choices[0]
	if choice.Message.Role != "assistant" {
		return nil, ServerError(502, "provider returned unsupported response content")
	}
	output := make([]ResponseOutputMessage, 0, 2)
	var text string
	trimmedContent := strings.TrimSpace(string(choice.Message.Content))
	if trimmedContent != "" && trimmedContent != "null" {
		if err := json.Unmarshal(choice.Message.Content, &text); err != nil {
			return nil, ServerError(502, "provider returned unsupported response content")
		}
		if text != "" {
			output = append(output, ResponseOutputMessage{ID: messageID, Type: "message", Status: "completed", Role: "assistant", Content: []ResponseOutputText{{Type: "output_text", Text: text, Annotations: []any{}}}})
		}
	}
	toolCallsRaw := choice.Message.Extra["tool_calls"]
	if len(toolCallsRaw) > 0 {
		var calls []chatResponseToolCall
		if json.Unmarshal(toolCallsRaw, &calls) != nil || len(calls) == 0 {
			return nil, ServerError(502, "provider returned unsupported function calls")
		}
		seen := map[string]bool{}
		for i, call := range calls {
			if strings.TrimSpace(call.ID) == "" || seen[call.ID] || call.Type != "function" || strings.TrimSpace(call.Function.Name) == "" || !ValidJSONString(call.Function.Arguments) {
				return nil, ServerError(502, "provider returned unsupported function calls")
			}
			seen[call.ID] = true
			output = append(output, ResponseOutputMessage{ID: fmt.Sprintf("fc_%s_%d", strings.TrimPrefix(responseID, "resp_"), i), Type: "function_call", Status: "completed", CallID: call.ID, Name: call.Function.Name, Arguments: call.Function.Arguments})
		}
	}
	if len(output) == 0 {
		return nil, ServerError(502, "provider returned unsupported response content")
	}
	response := &Response{
		ID: responseID, Object: "response", CreatedAt: now.Unix(), Status: "completed",
		Model: externalModel, Output: output,
		ParallelToolCalls: true, Store: false, Tools: []any{},
	}
	if chat.Usage != nil {
		response.Usage = &ResponseUsage{InputTokens: chat.Usage.PromptTokens, OutputTokens: chat.Usage.CompletionTokens, TotalTokens: chat.Usage.TotalTokens}
	}
	return response, nil
}

type chatResponseToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}
