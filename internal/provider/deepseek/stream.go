package deepseek

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"open-ai-gateway/internal/compat"
	providerdialect "open-ai-gateway/internal/provider/dialect"
)

type streamToolCall struct {
	id        string
	typeName  string
	name      strings.Builder
	arguments strings.Builder
}

type streamDecoder struct {
	dialect    *deepseekDialect
	reasoning  strings.Builder
	text       strings.Builder
	tools      map[int]*streamToolCall
	usage      *compat.Usage
	seenFinish bool
}

func (d *streamDecoder) Push(chunk compat.ChatCompletionChunk) ([]providerdialect.StreamEvent, error) {
	if len(chunk.Choices) > 1 || (len(chunk.Choices) == 1 && chunk.Choices[0].Index != 0) {
		return nil, errors.New("provider returned unsupported stream choices")
	}
	events := make([]providerdialect.StreamEvent, 0, 2)
	if len(chunk.Choices) == 1 {
		if d.seenFinish {
			return nil, errors.New("provider sent content after stream finish")
		}
		choice := chunk.Choices[0]
		if raw := choice.Delta.Extra["reasoning_content"]; len(raw) > 0 {
			value, err := rawString(raw)
			if err != nil {
				return nil, errors.New("provider returned malformed reasoning delta")
			}
			d.reasoning.WriteString(value)
		}
		if choice.Delta.Content != "" {
			d.text.WriteString(choice.Delta.Content)
			events = append(events, providerdialect.StreamEvent{TextDelta: choice.Delta.Content})
		}
		if raw := choice.Delta.Extra["tool_calls"]; len(raw) > 0 {
			if err := d.pushToolDeltas(raw); err != nil {
				return nil, err
			}
		}
		if choice.FinishReason != nil {
			d.seenFinish = true
		}
	}
	if chunk.Usage != nil {
		d.usage = cloneUsage(chunk.Usage)
		events = append(events, providerdialect.StreamEvent{Usage: cloneUsage(chunk.Usage)})
	}
	return events, nil
}

func (d *streamDecoder) pushToolDeltas(raw json.RawMessage) error {
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
		return errors.New("provider returned malformed tool call delta")
	}
	for _, delta := range deltas {
		if delta.Index < 0 {
			return errors.New("provider returned malformed tool call delta")
		}
		state := d.tools[delta.Index]
		if state == nil {
			state = &streamToolCall{}
			d.tools[delta.Index] = state
		}
		if delta.ID != "" {
			if state.id != "" && state.id != delta.ID {
				return errors.New("provider changed tool call ID")
			}
			state.id = delta.ID
		}
		if delta.Type != "" {
			if state.typeName != "" && state.typeName != delta.Type {
				return errors.New("provider changed tool call type")
			}
			state.typeName = delta.Type
		}
		state.name.WriteString(delta.Function.Name)
		state.arguments.WriteString(delta.Function.Arguments)
	}
	return nil
}

func (d *streamDecoder) Finish() (providerdialect.Response, error) {
	if !d.seenFinish {
		return providerdialect.Response{}, errors.New("provider stream ended before finish")
	}
	message := compat.ChatMessage{Role: "assistant"}
	message.Content, _ = json.Marshal(d.text.String())
	message.Extra = make(map[string]json.RawMessage)
	if d.reasoning.Len() > 0 {
		message.Extra["reasoning_content"], _ = json.Marshal(d.reasoning.String())
	}
	indexes := make([]int, 0, len(d.tools))
	for index := range d.tools {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	calls := make([]chatToolCall, 0, len(indexes))
	for _, index := range indexes {
		state := d.tools[index]
		arguments := state.arguments.String()
		if strings.TrimSpace(state.id) == "" || (state.typeName != "" && state.typeName != "function") || strings.TrimSpace(state.name.String()) == "" || !json.Valid([]byte(arguments)) {
			return providerdialect.Response{}, errors.New("provider returned incomplete function call stream")
		}
		calls = append(calls, chatToolCall{ID: state.id, Type: "function", Function: chatToolFunction{Name: state.name.String(), Arguments: arguments}})
	}
	if len(calls) > 0 {
		message.Extra["tool_calls"], _ = json.Marshal(calls)
	}
	response, err := d.dialect.ParseChatResponse(compat.ChatCompletionResponse{
		Choices: []compat.ChatCompletionChoice{{Index: 0, Message: message}}, Usage: cloneUsage(d.usage),
	})
	if err != nil {
		return providerdialect.Response{}, err
	}
	return response, nil
}
