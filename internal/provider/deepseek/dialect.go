package deepseek

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/conversation"
	providerdialect "open-ai-gateway/internal/provider/dialect"
)

var (
	ErrReasoningRequired           = providerdialect.ErrReasoningRequired
	ErrReasoningRouteMismatch      = providerdialect.ErrReasoningRouteMismatch
	ErrIncompleteReasoningToolCall = providerdialect.ErrIncompleteReasoningToolCall
)

type IDGenerator func() (string, error)

type deepseekDialect struct {
	newID IDGenerator
}

func NewDialect() providerdialect.Dialect {
	return NewDialectWithIDGenerator(randomEnvelopeID)
}

func NewDialectWithIDGenerator(generator IDGenerator) providerdialect.Dialect {
	if generator == nil {
		generator = randomEnvelopeID
	}
	return &deepseekDialect{newID: generator}
}

func (d *deepseekDialect) Name() string { return "deepseek" }

func (d *deepseekDialect) Capabilities() providerdialect.Capabilities {
	return providerdialect.Capabilities{ReasoningReplay: true, ProducesReasoning: true, BufferStreamUntilFinish: true}
}

func (d *deepseekDialect) BuildChatRequest(input providerdialect.Request) (compat.ChatCompletionRequest, error) {
	if strings.TrimSpace(input.UpstreamModel) == "" {
		return compat.ChatCompletionRequest{}, fmt.Errorf("upstream model is required")
	}
	request := compat.ChatCompletionRequest{
		Model: input.UpstreamModel, Stream: input.Conversation.Stream,
		Messages: make([]compat.ChatMessage, 0, len(input.Conversation.Turn.Items)),
		Extra:    make(map[string]json.RawMessage),
	}
	if err := applyAllowedFields(&request, input.Conversation.Extra); err != nil {
		return compat.ChatCompletionRequest{}, err
	}
	request.Extra["thinking"] = json.RawMessage(`{"type":"enabled"}`)
	if effort := input.Conversation.ReasoningEffort; effort != "" {
		mapped, err := mapReasoningEffort(effort)
		if err != nil {
			return compat.ChatCompletionRequest{}, err
		}
		request.Extra["reasoning_effort"], _ = json.Marshal(mapped)
	}
	if len(input.Conversation.Tools) > 0 {
		tools := make([]map[string]any, 0, len(input.Conversation.Tools))
		for _, tool := range input.Conversation.Tools {
			tools = append(tools, map[string]any{"type": "function", "function": map[string]any{
				"name": tool.Name, "description": tool.Description, "parameters": json.RawMessage(tool.Parameters), "strict": tool.Strict,
			}})
		}
		request.Extra["tools"], _ = json.Marshal(tools)
	}
	items := input.Conversation.Turn.Items
	for index := 0; index < len(items); index++ {
		switch item := items[index].(type) {
		case conversation.Message:
			content, _ := json.Marshal(item.Text)
			request.Messages = append(request.Messages, compat.ChatMessage{Role: item.Role, Content: content})
		case conversation.Reasoning:
			if len(item.CallIDs) > 0 && item.Route != input.Route {
				return compat.ChatCompletionRequest{}, ErrReasoningRouteMismatch
			}
			callStart := index + 1
			if callStart < len(items) {
				if message, ok := items[callStart].(conversation.Message); ok && message.Role == "assistant" && message.Text == item.AssistantContent && message.Text != "" {
					callStart++
				}
			}
			calls := make([]chatToolCall, 0, len(item.CallIDs))
			for offset, callID := range item.CallIDs {
				position := callStart + offset
				if position >= len(items) {
					return compat.ChatCompletionRequest{}, errors.New("reasoning item does not match function calls")
				}
				call, ok := items[position].(conversation.FunctionCall)
				if !ok || call.CallID != callID || call.ReasoningEnvelopeID != item.EnvelopeID {
					return compat.ChatCompletionRequest{}, errors.New("reasoning item does not match function calls")
				}
				calls = append(calls, chatToolCall{ID: call.CallID, Type: "function", Function: chatToolFunction{Name: call.Name, Arguments: call.Arguments}})
			}
			content, _ := json.Marshal(item.AssistantContent)
			reasoning, _ := json.Marshal(item.Content)
			extra := map[string]json.RawMessage{"reasoning_content": reasoning}
			if len(calls) > 0 {
				extra["tool_calls"], _ = json.Marshal(calls)
			}
			request.Messages = append(request.Messages, compat.ChatMessage{Role: "assistant", Content: content, Extra: extra})
			index = callStart + len(calls) - 1
		case conversation.FunctionCall:
			return compat.ChatCompletionRequest{}, ErrReasoningRequired
		case conversation.FunctionOutput:
			callID, _ := json.Marshal(item.CallID)
			request.Messages = append(request.Messages, compat.ChatMessage{Role: "tool", Content: cloneRaw(item.Output), Extra: map[string]json.RawMessage{"tool_call_id": callID}})
		case conversation.Opaque:
			return compat.ChatCompletionRequest{}, fmt.Errorf("deepseek dialect does not support opaque item %q", item.Kind)
		default:
			return compat.ChatCompletionRequest{}, errors.New("unsupported conversation item")
		}
	}
	return request, nil
}

func (d *deepseekDialect) ParseChatResponse(response compat.ChatCompletionResponse) (providerdialect.Response, error) {
	if len(response.Choices) != 1 || response.Choices[0].Index != 0 || response.Choices[0].Message.Role != "assistant" {
		return providerdialect.Response{}, errors.New("provider returned unsupported response choices")
	}
	message := response.Choices[0].Message
	text, err := assistantText(message.Content)
	if err != nil {
		return providerdialect.Response{}, err
	}
	reasoning, err := rawString(message.Extra["reasoning_content"])
	if err != nil {
		return providerdialect.Response{}, errors.New("provider returned unsupported reasoning content")
	}
	calls, err := parseToolCalls(message.Extra["tool_calls"])
	if err != nil {
		return providerdialect.Response{}, err
	}
	if len(calls) > 0 && reasoning == "" {
		return providerdialect.Response{}, ErrIncompleteReasoningToolCall
	}
	items := make([]conversation.Item, 0, 2+len(calls))
	envelopeID := ""
	if reasoning != "" {
		envelopeID, err = d.newID()
		if err != nil || strings.TrimSpace(envelopeID) == "" {
			return providerdialect.Response{}, errors.New("failed to generate reasoning envelope ID")
		}
		callIDs := make([]string, len(calls))
		for index, call := range calls {
			callIDs[index] = call.ID
		}
		items = append(items, conversation.Reasoning{EnvelopeID: envelopeID, Content: reasoning, AssistantContent: text, CallIDs: callIDs})
	}
	if text != "" {
		items = append(items, conversation.Message{Role: "assistant", Text: text})
	}
	for _, call := range calls {
		items = append(items, conversation.FunctionCall{CallID: call.ID, Name: call.Function.Name, Arguments: call.Function.Arguments, ReasoningEnvelopeID: envelopeID})
	}
	if len(items) == 0 {
		return providerdialect.Response{}, errors.New("provider returned unsupported response content")
	}
	return providerdialect.Response{Turn: conversation.Turn{Items: items}, Usage: cloneUsage(response.Usage)}, nil
}

func (d *deepseekDialect) NewStreamDecoder() providerdialect.StreamDecoder {
	return &streamDecoder{dialect: d, tools: make(map[int]*streamToolCall)}
}

type chatToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function chatToolFunction `json:"function"`
}

type chatToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func parseToolCalls(raw json.RawMessage) ([]chatToolCall, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var calls []chatToolCall
	if json.Unmarshal(raw, &calls) != nil || len(calls) == 0 {
		return nil, errors.New("provider returned unsupported function calls")
	}
	seen := make(map[string]struct{}, len(calls))
	for _, call := range calls {
		if strings.TrimSpace(call.ID) == "" || call.Type != "function" || strings.TrimSpace(call.Function.Name) == "" || !json.Valid([]byte(call.Function.Arguments)) {
			return nil, errors.New("provider returned unsupported function calls")
		}
		if _, exists := seen[call.ID]; exists {
			return nil, errors.New("provider returned unsupported function calls")
		}
		seen[call.ID] = struct{}{}
	}
	return calls, nil
}

func assistantText(raw json.RawMessage) (string, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return "", nil
	}
	var text string
	if json.Unmarshal(raw, &text) != nil {
		return "", errors.New("provider returned unsupported assistant content")
	}
	return text, nil
}

func rawString(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return "", errors.New("value must be a string")
	}
	return value, nil
}

func mapReasoningEffort(value string) (string, error) {
	switch value {
	case "low", "medium", "high":
		return "high", nil
	case "xhigh":
		return "max", nil
	default:
		return "", fmt.Errorf("unsupported reasoning effort %q", value)
	}
}

func applyAllowedFields(request *compat.ChatCompletionRequest, extra map[string]json.RawMessage) error {
	for key, raw := range extra {
		switch key {
		case "temperature":
			var value float64
			if json.Unmarshal(raw, &value) != nil {
				return errors.New("temperature must be a number")
			}
			request.Temperature = &value
		case "top_p":
			var value float64
			if json.Unmarshal(raw, &value) != nil {
				return errors.New("top_p must be a number")
			}
			request.TopP = &value
		case "max_tokens":
			var value int
			if json.Unmarshal(raw, &value) != nil {
				return errors.New("max_tokens must be an integer")
			}
			request.MaxTokens = &value
		case "stop":
			request.Stop = cloneRaw(raw)
		case "user":
			if json.Unmarshal(raw, &request.User) != nil {
				return errors.New("user must be a string")
			}
		case "frequency_penalty", "presence_penalty", "response_format", "seed", "stream_options":
			request.Extra[key] = cloneRaw(raw)
		}
	}
	return nil
}

func cloneRaw(value json.RawMessage) json.RawMessage {
	return append(json.RawMessage(nil), value...)
}

func cloneUsage(usage *compat.Usage) *compat.Usage {
	if usage == nil {
		return nil
	}
	copy := *usage
	return &copy
}

func randomEnvelopeID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return "env_" + hex.EncodeToString(value[:]), nil
}
