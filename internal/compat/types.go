package compat

import (
	"encoding/json"
	"strings"
)

type ChatCompletionRequest struct {
	Model       string                     `json:"model"`
	Messages    []ChatMessage              `json:"messages"`
	Stream      bool                       `json:"stream,omitempty"`
	Temperature *float64                   `json:"temperature,omitempty"`
	TopP        *float64                   `json:"top_p,omitempty"`
	MaxTokens   *int                       `json:"max_tokens,omitempty"`
	Stop        json.RawMessage            `json:"stop,omitempty"`
	User        string                     `json:"user,omitempty"`
	Extra       map[string]json.RawMessage `json:"-"`
}

type ChatMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type chatCompletionRequestJSON struct {
	Model       string          `json:"model"`
	Messages    []ChatMessage   `json:"messages"`
	Stream      bool            `json:"stream,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
	MaxTokens   *int            `json:"max_tokens,omitempty"`
	Stop        json.RawMessage `json:"stop,omitempty"`
	User        string          `json:"user,omitempty"`
}

var chatCompletionRequestKnownFields = []string{
	"model",
	"messages",
	"stream",
	"temperature",
	"top_p",
	"max_tokens",
	"stop",
	"user",
}

func (r *ChatCompletionRequest) UnmarshalJSON(data []byte) error {
	var known chatCompletionRequestJSON
	if err := json.Unmarshal(data, &known); err != nil {
		return err
	}
	extra, err := decodeExtraFields(data, chatCompletionRequestKnownFields)
	if err != nil {
		return err
	}
	*r = ChatCompletionRequest{
		Model:       known.Model,
		Messages:    known.Messages,
		Stream:      known.Stream,
		Temperature: known.Temperature,
		TopP:        known.TopP,
		MaxTokens:   known.MaxTokens,
		Stop:        known.Stop,
		User:        known.User,
		Extra:       extra,
	}
	return nil
}

func (r ChatCompletionRequest) MarshalJSON() ([]byte, error) {
	fields := copyRawFields(r.Extra, chatCompletionRequestKnownFields)
	if err := putJSONField(fields, "model", r.Model); err != nil {
		return nil, err
	}
	if r.Messages != nil {
		if err := putJSONField(fields, "messages", r.Messages); err != nil {
			return nil, err
		}
	}
	if r.Stream {
		if err := putJSONField(fields, "stream", r.Stream); err != nil {
			return nil, err
		}
	}
	if r.Temperature != nil {
		if err := putJSONField(fields, "temperature", r.Temperature); err != nil {
			return nil, err
		}
	}
	if r.TopP != nil {
		if err := putJSONField(fields, "top_p", r.TopP); err != nil {
			return nil, err
		}
	}
	if r.MaxTokens != nil {
		if err := putJSONField(fields, "max_tokens", r.MaxTokens); err != nil {
			return nil, err
		}
	}
	if len(r.Stop) > 0 {
		fields["stop"] = cloneRawMessage(r.Stop)
	}
	if r.User != "" {
		if err := putJSONField(fields, "user", r.User); err != nil {
			return nil, err
		}
	}
	return json.Marshal(fields)
}

func (r ChatCompletionRequest) Validate() *Error {
	if strings.TrimSpace(r.Model) == "" {
		return InvalidRequest("missing required field: model", "model")
	}
	if len(r.Messages) == 0 {
		return InvalidRequest("missing required field: messages", "messages")
	}
	for i, msg := range r.Messages {
		if strings.TrimSpace(msg.Role) == "" {
			return InvalidRequest("message role is required", "messages")
		}
		if !hasProcessableContent(msg.Content) {
			return InvalidRequest("message content is required", "messages")
		}
		if i == 0 {
			continue
		}
	}
	return nil
}

func hasProcessableContent(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s) != ""
	}
	return true
}

type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   *Usage                 `json:"usage,omitempty"`
}

type ChatCompletionChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatCompletionChunk struct {
	ID      string                      `json:"id"`
	Object  string                      `json:"object"`
	Created int64                       `json:"created"`
	Model   string                      `json:"model"`
	Choices []ChatCompletionChunkChoice `json:"choices"`
	Usage   *Usage                      `json:"usage,omitempty"`
}

type ChatCompletionChunkChoice struct {
	Index        int              `json:"index"`
	Delta        ChatMessageDelta `json:"delta"`
	FinishReason *string          `json:"finish_reason"`
}

type ChatMessageDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type ModelListResponse struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

type EmbeddingRequest struct {
	Model          string                     `json:"model"`
	Input          json.RawMessage            `json:"input"`
	EncodingFormat string                     `json:"encoding_format,omitempty"`
	User           string                     `json:"user,omitempty"`
	Extra          map[string]json.RawMessage `json:"-"`
}

type embeddingRequestJSON struct {
	Model          string          `json:"model"`
	Input          json.RawMessage `json:"input"`
	EncodingFormat string          `json:"encoding_format,omitempty"`
	User           string          `json:"user,omitempty"`
}

var embeddingRequestKnownFields = []string{
	"model",
	"input",
	"encoding_format",
	"user",
}

func (r *EmbeddingRequest) UnmarshalJSON(data []byte) error {
	var known embeddingRequestJSON
	if err := json.Unmarshal(data, &known); err != nil {
		return err
	}
	extra, err := decodeExtraFields(data, embeddingRequestKnownFields)
	if err != nil {
		return err
	}
	*r = EmbeddingRequest{
		Model:          known.Model,
		Input:          known.Input,
		EncodingFormat: known.EncodingFormat,
		User:           known.User,
		Extra:          extra,
	}
	return nil
}

func (r EmbeddingRequest) MarshalJSON() ([]byte, error) {
	fields := copyRawFields(r.Extra, embeddingRequestKnownFields)
	if err := putJSONField(fields, "model", r.Model); err != nil {
		return nil, err
	}
	if len(r.Input) > 0 {
		fields["input"] = cloneRawMessage(r.Input)
	}
	if r.EncodingFormat != "" {
		if err := putJSONField(fields, "encoding_format", r.EncodingFormat); err != nil {
			return nil, err
		}
	}
	if r.User != "" {
		if err := putJSONField(fields, "user", r.User); err != nil {
			return nil, err
		}
	}
	return json.Marshal(fields)
}

func (r EmbeddingRequest) Validate() *Error {
	if strings.TrimSpace(r.Model) == "" {
		return InvalidRequest("missing required field: model", "model")
	}
	if !hasProcessableEmbeddingInput(r.Input) {
		return InvalidRequest("missing required field: input", "input")
	}
	return nil
}

func hasProcessableEmbeddingInput(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s) != ""
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err == nil {
		return len(items) > 0
	}
	return true
}

type EmbeddingResponse struct {
	Object string          `json:"object"`
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  *Usage          `json:"usage,omitempty"`
}

type EmbeddingData struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

func decodeExtraFields(data []byte, knownFields []string) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	for _, field := range knownFields {
		delete(fields, field)
	}
	if len(fields) == 0 {
		return nil, nil
	}
	return fields, nil
}

func copyRawFields(src map[string]json.RawMessage, knownFields []string) map[string]json.RawMessage {
	fields := make(map[string]json.RawMessage, len(src)+len(knownFields))
	for key, value := range src {
		fields[key] = cloneRawMessage(value)
	}
	for _, field := range knownFields {
		delete(fields, field)
	}
	return fields
}

func putJSONField(fields map[string]json.RawMessage, key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	fields[key] = data
	return nil
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return nil
	}
	cloned := make(json.RawMessage, len(raw))
	copy(cloned, raw)
	return cloned
}
