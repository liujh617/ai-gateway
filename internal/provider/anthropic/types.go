package anthropic

import "encoding/json"

type MessageRequest struct {
	Model         string         `json:"model"`
	Messages      []Message      `json:"messages"`
	System        string         `json:"system,omitempty"`
	Stream        bool           `json:"stream,omitempty"`
	MaxTokens     int            `json:"max_tokens"`
	Temperature   *float64       `json:"temperature,omitempty"`
	TopP          *float64       `json:"top_p,omitempty"`
	StopSequences []string       `json:"stop_sequences,omitempty"`
	Tools         []Tool         `json:"tools,omitempty"`
	ToolChoice    map[string]any `json:"tool_choice,omitempty"`
}

type Message struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

type ContentBlock struct {
	Type         string          `json:"type"`
	Text         string          `json:"text,omitempty"`
	ID           string          `json:"id,omitempty"`
	Name         string          `json:"name,omitempty"`
	Input        json.RawMessage `json:"input,omitempty"`
	ToolUseID    string          `json:"tool_use_id,omitempty"`
	Content      string          `json:"content,omitempty"`
	IsError      bool            `json:"is_error,omitempty"`
}

type Tool struct {
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	InputSchema *ToolSchema `json:"input_schema"`
}

type ToolSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]any         `json:"properties,omitempty"`
	Required   []string               `json:"required,omitempty"`
	Extra      map[string]json.RawMessage `json:"-"`
}

type MessageResponse struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Role       string         `json:"role"`
	Model      string         `json:"model"`
	Content    []ContentBlock `json:"content"`
	StopReason string         `json:"stop_reason"`
	Usage      *Usage         `json:"usage"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type MessageStreamEvent struct {
	Type         string         `json:"type"`
	Index        int            `json:"index"`
	Message      *StreamMessage `json:"message,omitempty"`
	ContentBlock *ContentBlock  `json:"content_block,omitempty"`
	Delta        *StreamDelta   `json:"delta,omitempty"`
	Usage        *Usage         `json:"usage,omitempty"`
}

type StreamMessage struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Role  string `json:"role"`
	Model string `json:"model"`
}

type StreamDelta struct {
	Type         string `json:"type"`
	Text         string `json:"text,omitempty"`
	PartialJSON  string `json:"partial_json,omitempty"`
	StopReason   string `json:"stop_reason,omitempty"`
	StopSequence string `json:"stop_sequence,omitempty"`
}

type ErrorResponse struct {
	Type  string       `json:"type"`
	Error ErrorDetail  `json:"error"`
}

type ErrorDetail struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
