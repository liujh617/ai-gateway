package compat

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type ResponseRequest struct {
	Model        string
	Input        json.RawMessage
	Instructions string
	Stream       bool
	Store        *bool
	Extra        map[string]json.RawMessage
}

func (r *ResponseRequest) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if raw, ok := fields["model"]; ok {
		if err := json.Unmarshal(raw, &r.Model); err != nil {
			return err
		}
		delete(fields, "model")
	}
	if raw, ok := fields["input"]; ok {
		r.Input = cloneRawMessage(raw)
		delete(fields, "input")
	}
	if raw, ok := fields["instructions"]; ok {
		if err := json.Unmarshal(raw, &r.Instructions); err != nil {
			return err
		}
		delete(fields, "instructions")
	}
	if raw, ok := fields["stream"]; ok {
		if err := json.Unmarshal(raw, &r.Stream); err != nil {
			return err
		}
		delete(fields, "stream")
	}
	if raw, ok := fields["store"]; ok {
		var store bool
		if err := json.Unmarshal(raw, &store); err != nil {
			return err
		}
		r.Store = &store
		delete(fields, "store")
	}
	if len(fields) > 0 {
		r.Extra = fields
	}
	return nil
}

func (r ResponseRequest) MarshalJSON() ([]byte, error) {
	fields := make(map[string]json.RawMessage, 5)
	model, _ := json.Marshal(r.Model)
	fields["model"] = model
	fields["input"] = cloneRawMessage(r.Input)
	if r.Instructions != "" {
		instructions, _ := json.Marshal(r.Instructions)
		fields["instructions"] = instructions
	}
	if r.Stream {
		stream, _ := json.Marshal(true)
		fields["stream"] = stream
	}
	if r.Store != nil {
		store, _ := json.Marshal(*r.Store)
		fields["store"] = store
	}
	for key, value := range r.Extra {
		fields[key] = cloneRawMessage(value)
	}
	return json.Marshal(fields)
}

func (r ResponseRequest) Validate() *Error {
	if len(r.Extra) > 0 {
		keys := make([]string, 0, len(r.Extra))
		for key := range r.Extra {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		return InvalidRequest("unsupported field: "+keys[0], keys[0])
	}
	if strings.TrimSpace(r.Model) == "" {
		return InvalidRequest("missing required field: model", "model")
	}
	if r.Store != nil && *r.Store {
		return InvalidRequest("unsupported field: store", "store")
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
	messages := make([]ChatMessage, 0, 2)
	if r.Instructions != "" {
		content, _ := json.Marshal(r.Instructions)
		messages = append(messages, ChatMessage{Role: "developer", Content: content})
	}
	var input string
	if err := json.Unmarshal(r.Input, &input); err == nil {
		if strings.TrimSpace(input) == "" {
			return ChatCompletionRequest{}, InvalidRequest("input text is required", "input")
		}
		content, _ := json.Marshal(input)
		messages = append(messages, ChatMessage{Role: "user", Content: content})
		return ChatCompletionRequest{Model: r.Model, Messages: messages, Stream: r.Stream}, nil
	}
	var items []responseInputMessage
	if err := json.Unmarshal(r.Input, &items); err != nil || len(items) == 0 {
		return ChatCompletionRequest{}, InvalidRequest("input must be text or a non-empty message array", "input")
	}
	for i, item := range items {
		role := strings.TrimSpace(item.Role)
		if role != "user" && role != "assistant" && role != "system" && role != "developer" {
			return ChatCompletionRequest{}, InvalidRequest(fmt.Sprintf("invalid input role at index %d", i), "input")
		}
		text, err := responseMessageText(item.Content)
		if err != nil {
			return ChatCompletionRequest{}, InvalidRequest(fmt.Sprintf("invalid text content at input index %d", i), "input")
		}
		content, _ := json.Marshal(text)
		messages = append(messages, ChatMessage{Role: role, Content: content})
	}
	return ChatCompletionRequest{Model: r.Model, Messages: messages, Stream: r.Stream}, nil
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
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if strings.TrimSpace(text) == "" {
			return "", fmt.Errorf("empty text")
		}
		return text, nil
	}
	var parts []responseInputText
	if err := json.Unmarshal(raw, &parts); err != nil || len(parts) == 0 {
		return "", fmt.Errorf("invalid content")
	}
	var out strings.Builder
	for _, part := range parts {
		if part.Type != "input_text" || strings.TrimSpace(part.Text) == "" {
			return "", fmt.Errorf("unsupported content part")
		}
		out.WriteString(part.Text)
	}
	return out.String(), nil
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
	Store              bool                    `json:"store"`
	Tools              []any                   `json:"tools"`
	Usage              *ResponseUsage          `json:"usage"`
}

type ResponseOutputMessage struct {
	ID      string               `json:"id"`
	Type    string               `json:"type"`
	Status  string               `json:"status"`
	Role    string               `json:"role"`
	Content []ResponseOutputText `json:"content"`
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
	if choice.Message.Role != "assistant" || len(choice.Message.Extra) != 0 {
		return nil, ServerError(502, "provider returned unsupported response content")
	}
	var text string
	if err := json.Unmarshal(choice.Message.Content, &text); err != nil {
		return nil, ServerError(502, "provider returned unsupported response content")
	}
	response := &Response{
		ID: responseID, Object: "response", CreatedAt: now.Unix(), Status: "completed",
		Model: externalModel, Output: []ResponseOutputMessage{{
			ID: messageID, Type: "message", Status: "completed", Role: "assistant",
			Content: []ResponseOutputText{{Type: "output_text", Text: text, Annotations: []any{}}},
		}},
		ParallelToolCalls: true, Store: false, Tools: []any{},
	}
	if chat.Usage != nil {
		response.Usage = &ResponseUsage{InputTokens: chat.Usage.PromptTokens, OutputTokens: chat.Usage.CompletionTokens, TotalTokens: chat.Usage.TotalTokens}
	}
	return response, nil
}
