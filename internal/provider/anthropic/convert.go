package anthropic

import (
	"encoding/json"
	"strings"

	"open-ai-gateway/internal/compat"
)

const roleAssistant = "assistant"
const roleUser = "user"

func openaiToAnthropic(req compat.ChatCompletionRequest) MessageRequest {
	ar := MessageRequest{
		Model:     req.Model,
		Stream:    req.Stream,
		MaxTokens: 4096,
	}
	if req.Temperature != nil {
		ar.Temperature = req.Temperature
	}
	if req.TopP != nil {
		ar.TopP = req.TopP
	}
	if req.MaxTokens != nil {
		ar.MaxTokens = *req.MaxTokens
	}

	var stopStrings []string
	if err := json.Unmarshal(req.Stop, &stopStrings); err == nil && len(stopStrings) > 0 {
		ar.StopSequences = stopStrings
	}

	var msgs []Message
	for _, msg := range req.Messages {
		if msg.Role == "system" || msg.Role == "developer" {
			var text string
			if err := json.Unmarshal(msg.Content, &text); err == nil {
				if ar.System == "" {
					ar.System = text
				} else {
					ar.System += "\n\n" + text
				}
			}
			continue
		}
		content := openaiMsgToContent(msg)
		msgs = append(msgs, Message{Role: msg.Role, Content: content})
	}
	ar.Messages = msgs

	if raw, ok := req.Extra["tools"]; ok && len(raw) > 0 {
		json.Unmarshal(raw, &ar.Tools)
	}
	if raw, ok := req.Extra["tool_choice"]; ok && len(raw) > 0 {
		json.Unmarshal(raw, &ar.ToolChoice)
	}
	return ar
}

func openaiMsgToContent(msg compat.ChatMessage) []ContentBlock {
	var blocks []ContentBlock

	if msg.Role == roleUser {
		var text string
		if json.Unmarshal(msg.Content, &text) == nil && strings.TrimSpace(text) != "" {
			blocks = append(blocks, ContentBlock{Type: "text", Text: text})
		}
		return blocks
	}

	if msg.Role == roleAssistant {
		raw := msg.Extra["tool_calls"]
		if len(raw) > 0 {
			var calls []struct {
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			}
			if json.Unmarshal(raw, &calls) == nil {
				for _, c := range calls {
					blocks = append(blocks, ContentBlock{
						Type:  "tool_use",
						ID:    c.ID,
						Name:  c.Function.Name,
						Input: json.RawMessage(c.Function.Arguments),
					})
				}
			}
		}
		var text string
		if json.Unmarshal(msg.Content, &text) == nil && strings.TrimSpace(text) != "" {
			blocks = append(blocks, ContentBlock{Type: "text", Text: text})
		}
		return blocks
	}

	if msg.Role == "tool" {
		var output string
		json.Unmarshal(msg.Content, &output)
		var callID string
		if raw := msg.Extra["tool_call_id"]; len(raw) > 0 {
			json.Unmarshal(raw, &callID)
		}
		blocks = append(blocks, ContentBlock{
			Type:      "tool_result",
			ToolUseID: callID,
			Content:   output,
		})
		return blocks
	}

	return blocks
}

func anthropicToOpenAI(resp MessageResponse, model string) *compat.ChatCompletionResponse {
	content, toolCalls := contentToOpenAI(resp.Content)
	finish := anthropicFinishReason(resp.StopReason)

	openaiResp := &compat.ChatCompletionResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: 0,
		Model:   model,
		Choices: []compat.ChatCompletionChoice{{
			Index: 0,
			Message: compat.ChatMessage{
				Role:    "assistant",
				Content: content,
			},
			FinishReason: finish,
		}},
	}
	if resp.Usage != nil {
		openaiResp.Usage = &compat.Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		}
	}
	if len(toolCalls) > 0 {
		calls, _ := json.Marshal(toolCalls)
		openaiResp.Choices[0].Message.Extra = map[string]json.RawMessage{
			"tool_calls": calls,
		}
	}
	return openaiResp
}

func contentToOpenAI(blocks []ContentBlock) (json.RawMessage, []map[string]any) {
	var textParts []string
	var toolCalls []map[string]any
	var toolCallIndex int

	for _, block := range blocks {
		switch block.Type {
		case "text":
			textParts = append(textParts, block.Text)
		case "tool_use":
			parsed, err := parseAnthropicJSON(block.Input)
			args := string(block.Input)
			if err == nil {
				args = parsed
			}
			toolCalls = append(toolCalls, map[string]any{
				"id":   block.ID,
				"type": "function",
				"function": map[string]any{
					"name":      block.Name,
					"arguments": args,
				},
			})
			toolCallIndex++
		}
	}

	if len(toolCalls) > 0 && len(textParts) == 0 {
		textParts = append(textParts, "")
	}
	text := strings.Join(textParts, "")
	if text == "" && len(toolCalls) == 0 {
		return json.RawMessage("null"), nil
	}
	content, _ := json.Marshal(text)
	return content, toolCalls
}

func parseAnthropicJSON(raw json.RawMessage) (string, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", err
	}
	pretty, err := json.Marshal(v)
	return string(pretty), err
}

func anthropicFinishReason(reason string) string {
	switch reason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	case "stop_sequence":
		return "stop"
	default:
		return "stop"
	}
}

func anthropicErrorToOpenAI(body []byte, statusCode int) *compat.Error {
	var ar ErrorResponse
	if json.Unmarshal(body, &ar) == nil && ar.Error.Type != "" {
		return compat.NewError(statusCode, ar.Error.Type, ar.Error.Message, nil)
	}
	return compat.NewError(statusCode, "api_error", string(body), nil)
}
