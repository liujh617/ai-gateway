package dialect

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/conversation"
)

type openAICompatible struct{}

func NewOpenAICompatible() Dialect {
	return openAICompatible{}
}

func (openAICompatible) Name() string { return "openai-compatible" }

func (openAICompatible) Capabilities() Capabilities { return Capabilities{} }

func (openAICompatible) BuildChatRequest(input Request) (compat.ChatCompletionRequest, error) {
	if strings.TrimSpace(input.UpstreamModel) == "" {
		return compat.ChatCompletionRequest{}, fmt.Errorf("upstream model is required")
	}
	request := compat.ChatCompletionRequest{
		Model:    input.UpstreamModel,
		Stream:   input.Conversation.Stream,
		Messages: make([]compat.ChatMessage, 0, len(input.Conversation.Turn.Items)),
		Extra:    cloneRawMap(input.Conversation.Extra),
	}
	if request.Extra == nil {
		request.Extra = make(map[string]json.RawMessage)
	}
	applyKnownRequestFields(&request)
	if input.Conversation.ReasoningEffort != "" {
		request.Extra["reasoning_effort"], _ = json.Marshal(input.Conversation.ReasoningEffort)
	}
	if len(input.Conversation.ToolChoice) > 0 {
		request.Extra["tool_choice"] = cloneRaw(input.Conversation.ToolChoice)
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
			return compat.ChatCompletionRequest{}, fmt.Errorf("openai-compatible dialect does not support reasoning replay")
		case conversation.FunctionCall:
			calls := make([]chatToolCall, 0, 1)
			for index < len(items) {
				call, ok := items[index].(conversation.FunctionCall)
				if !ok {
					break
				}
				if call.ReasoningEnvelopeID != "" {
					return compat.ChatCompletionRequest{}, fmt.Errorf("openai-compatible dialect does not support reasoning replay")
				}
				calls = append(calls, chatToolCall{ID: call.CallID, Type: "function", Function: chatToolFunction{Name: call.Name, Arguments: call.Arguments}})
				index++
			}
			index--
			raw, _ := json.Marshal(calls)
			request.Messages = append(request.Messages, compat.ChatMessage{Role: "assistant", Content: json.RawMessage("null"), Extra: map[string]json.RawMessage{"tool_calls": raw}})
		case conversation.FunctionOutput:
			callID, _ := json.Marshal(item.CallID)
			request.Messages = append(request.Messages, compat.ChatMessage{Role: "tool", Content: cloneRaw(item.Output), Extra: map[string]json.RawMessage{"tool_call_id": callID}})
		case conversation.Opaque:
			return compat.ChatCompletionRequest{}, fmt.Errorf("openai-compatible dialect does not support opaque item %q", item.Kind)
		default:
			return compat.ChatCompletionRequest{}, fmt.Errorf("unsupported conversation item")
		}
	}
	return request, nil
}

func (openAICompatible) ParseChatResponse(response compat.ChatCompletionResponse) (Response, error) {
	if len(response.Choices) != 1 || response.Choices[0].Index != 0 || response.Choices[0].Message.Role != "assistant" {
		return Response{}, fmt.Errorf("provider returned unsupported response choices")
	}
	turn, err := parseAssistant(response.Choices[0].Message)
	if err != nil {
		return Response{}, err
	}
	return Response{Turn: turn, Usage: cloneUsage(response.Usage)}, nil
}

func (openAICompatible) NewStreamDecoder() StreamDecoder {
	return &openAIStreamDecoder{tools: make(map[int]*streamToolCall)}
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

func parseAssistant(message compat.ChatMessage) (conversation.Turn, error) {
	items := make([]conversation.Item, 0, 2)
	trimmed := strings.TrimSpace(string(message.Content))
	if trimmed != "" && trimmed != "null" {
		var text string
		if json.Unmarshal(message.Content, &text) != nil {
			return conversation.Turn{}, fmt.Errorf("provider returned unsupported assistant content")
		}
		if text != "" {
			items = append(items, conversation.Message{Role: "assistant", Text: text})
		}
	}
	if raw := message.Extra["tool_calls"]; len(raw) > 0 {
		var calls []chatToolCall
		if json.Unmarshal(raw, &calls) != nil || len(calls) == 0 {
			return conversation.Turn{}, fmt.Errorf("provider returned unsupported function calls")
		}
		seen := make(map[string]struct{}, len(calls))
		for _, call := range calls {
			if strings.TrimSpace(call.ID) == "" || call.Type != "function" || strings.TrimSpace(call.Function.Name) == "" || !json.Valid([]byte(call.Function.Arguments)) {
				return conversation.Turn{}, fmt.Errorf("provider returned unsupported function calls")
			}
			if _, exists := seen[call.ID]; exists {
				return conversation.Turn{}, fmt.Errorf("provider returned unsupported function calls")
			}
			seen[call.ID] = struct{}{}
			items = append(items, conversation.FunctionCall{CallID: call.ID, Name: call.Function.Name, Arguments: call.Function.Arguments})
		}
	}
	if len(items) == 0 {
		return conversation.Turn{}, fmt.Errorf("provider returned unsupported response content")
	}
	return conversation.Turn{Items: items}, nil
}

type streamToolCall struct {
	id        string
	typeName  string
	name      strings.Builder
	arguments strings.Builder
}

type openAIStreamDecoder struct {
	text       strings.Builder
	tools      map[int]*streamToolCall
	usage      *compat.Usage
	seenFinish bool
}

func (d *openAIStreamDecoder) Push(chunk compat.ChatCompletionChunk) ([]StreamEvent, error) {
	if len(chunk.Choices) > 1 || (len(chunk.Choices) == 1 && chunk.Choices[0].Index != 0) {
		return nil, fmt.Errorf("provider returned unsupported stream choices")
	}
	events := make([]StreamEvent, 0, 2)
	if len(chunk.Choices) == 1 {
		choice := chunk.Choices[0]
		if d.seenFinish {
			return nil, fmt.Errorf("provider sent content after stream finish")
		}
		if choice.Delta.Content != "" {
			d.text.WriteString(choice.Delta.Content)
			events = append(events, StreamEvent{TextDelta: choice.Delta.Content})
		}
		if raw := choice.Delta.Extra["tool_calls"]; len(raw) > 0 {
			toolEvents, err := d.pushToolDeltas(raw)
			if err != nil {
				return nil, err
			}
			events = append(events, toolEvents...)
		}
		if choice.FinishReason != nil {
			d.seenFinish = true
		}
	}
	if chunk.Usage != nil {
		d.usage = cloneUsage(chunk.Usage)
		events = append(events, StreamEvent{Usage: cloneUsage(chunk.Usage)})
	}
	return events, nil
}

func (d *openAIStreamDecoder) pushToolDeltas(raw json.RawMessage) ([]StreamEvent, error) {
	var deltas []struct {
		Index    int    `json:"index"`
		ID       string `json:"id"`
		Type     string `json:"type"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}
	if json.Unmarshal(raw, &deltas) != nil || len(deltas) == 0 {
		return nil, fmt.Errorf("provider returned malformed tool call delta")
	}
	events := make([]StreamEvent, 0, len(deltas))
	for _, delta := range deltas {
		if delta.Index < 0 {
			return nil, fmt.Errorf("provider returned malformed tool call delta")
		}
		state := d.tools[delta.Index]
		if state == nil {
			state = &streamToolCall{}
			d.tools[delta.Index] = state
		}
		if delta.ID != "" {
			if state.id != "" && state.id != delta.ID {
				return nil, fmt.Errorf("provider changed tool call ID")
			}
			state.id = delta.ID
		}
		if delta.Type != "" {
			if state.typeName != "" && state.typeName != delta.Type {
				return nil, fmt.Errorf("provider changed tool call type")
			}
			state.typeName = delta.Type
		}
		state.name.WriteString(delta.Function.Name)
		state.arguments.WriteString(delta.Function.Arguments)
		events = append(events, StreamEvent{FunctionCallDelta: &FunctionCallDelta{Index: delta.Index, CallID: delta.ID, Name: delta.Function.Name, Arguments: delta.Function.Arguments}})
	}
	return events, nil
}

func (d *openAIStreamDecoder) Finish() (Response, error) {
	if !d.seenFinish {
		return Response{}, fmt.Errorf("provider stream ended before finish")
	}
	items := make([]conversation.Item, 0, 1+len(d.tools))
	if d.text.Len() > 0 {
		items = append(items, conversation.Message{Role: "assistant", Text: d.text.String()})
	}
	indexes := make([]int, 0, len(d.tools))
	for index := range d.tools {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	seen := make(map[string]struct{}, len(indexes))
	for _, index := range indexes {
		state := d.tools[index]
		arguments := state.arguments.String()
		if strings.TrimSpace(state.id) == "" || (state.typeName != "" && state.typeName != "function") || strings.TrimSpace(state.name.String()) == "" || !json.Valid([]byte(arguments)) {
			return Response{}, fmt.Errorf("provider returned incomplete function call stream")
		}
		if _, exists := seen[state.id]; exists {
			return Response{}, fmt.Errorf("provider returned duplicate function call ID")
		}
		seen[state.id] = struct{}{}
		items = append(items, conversation.FunctionCall{CallID: state.id, Name: state.name.String(), Arguments: arguments})
	}
	if len(items) == 0 {
		return Response{}, fmt.Errorf("provider stream returned no content")
	}
	return Response{Turn: conversation.Turn{Items: items}, Usage: cloneUsage(d.usage)}, nil
}

func applyKnownRequestFields(request *compat.ChatCompletionRequest) {
	if raw := request.Extra["temperature"]; len(raw) > 0 {
		var value float64
		if json.Unmarshal(raw, &value) == nil {
			request.Temperature = &value
			delete(request.Extra, "temperature")
		}
	}
	if raw := request.Extra["top_p"]; len(raw) > 0 {
		var value float64
		if json.Unmarshal(raw, &value) == nil {
			request.TopP = &value
			delete(request.Extra, "top_p")
		}
	}
	if raw := request.Extra["max_tokens"]; len(raw) > 0 {
		var value int
		if json.Unmarshal(raw, &value) == nil {
			request.MaxTokens = &value
			delete(request.Extra, "max_tokens")
		}
	}
	if raw := request.Extra["stop"]; len(raw) > 0 {
		request.Stop = cloneRaw(raw)
		delete(request.Extra, "stop")
	}
	if raw := request.Extra["user"]; len(raw) > 0 {
		if json.Unmarshal(raw, &request.User) == nil {
			delete(request.Extra, "user")
		}
	}
}

func cloneRaw(value json.RawMessage) json.RawMessage {
	return append(json.RawMessage(nil), value...)
}

func cloneRawMap(values map[string]json.RawMessage) map[string]json.RawMessage {
	if len(values) == 0 {
		return nil
	}
	clone := make(map[string]json.RawMessage, len(values))
	for key, value := range values {
		clone[key] = cloneRaw(value)
	}
	return clone
}

func cloneUsage(usage *compat.Usage) *compat.Usage {
	if usage == nil {
		return nil
	}
	copy := *usage
	return &copy
}
