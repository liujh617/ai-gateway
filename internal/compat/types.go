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
	Role    string                     `json:"role"`
	Content json.RawMessage            `json:"content"`
	Extra   map[string]json.RawMessage `json:"-"`
}

func (m *ChatMessage) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if v, ok := raw["role"]; ok {
		if err := json.Unmarshal(v, &m.Role); err != nil {
			return err
		}
		delete(raw, "role")
	}
	if v, ok := raw["content"]; ok {
		m.Content = v
		delete(raw, "content")
	}
	if len(raw) > 0 {
		m.Extra = raw
	}
	return nil
}

func (m ChatMessage) MarshalJSON() ([]byte, error) {
	fields := make(map[string]json.RawMessage, len(m.Extra)+2)
	for k, v := range m.Extra {
		fields[k] = cloneRawMessage(v)
	}
	roleBytes, err := json.Marshal(m.Role)
	if err != nil {
		return nil, err
	}
	fields["role"] = roleBytes
	fields["content"] = m.Content
	return json.Marshal(fields)
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
	// Reject missing or empty content; accept null (valid for tool messages / assistant with tool_calls).
	if trimmed == "" {
		return false
	}
	if trimmed == "null" {
		return true
	}
	// Accept array-type content (multimodal / content parts).
	if strings.HasPrefix(trimmed, "[") {
		return true
	}
	// Accept string content, even if empty (upstream validates).
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
	Role    string                     `json:"role,omitempty"`
	Content string                     `json:"content,omitempty"`
	Extra   map[string]json.RawMessage `json:"-"`
}

func (d *ChatMessageDelta) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if v, ok := raw["role"]; ok {
		if err := json.Unmarshal(v, &d.Role); err != nil {
			return err
		}
		delete(raw, "role")
	}
	if v, ok := raw["content"]; ok {
		if err := json.Unmarshal(v, &d.Content); err != nil {
			return err
		}
		delete(raw, "content")
	}
	if len(raw) > 0 {
		d.Extra = raw
	}
	return nil
}

func (d ChatMessageDelta) MarshalJSON() ([]byte, error) {
	fields := make(map[string]json.RawMessage, len(d.Extra)+2)
	for k, v := range d.Extra {
		fields[k] = cloneRawMessage(v)
	}
	if d.Role != "" {
		roleBytes, err := json.Marshal(d.Role)
		if err != nil {
			return nil, err
		}
		fields["role"] = roleBytes
	}
	if d.Content != "" {
		contentBytes, err := json.Marshal(d.Content)
		if err != nil {
			return nil, err
		}
		fields["content"] = contentBytes
	}
	return json.Marshal(fields)
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

// Completions

type CompletionsRequest struct {
	Model       string                     `json:"model"`
	Prompt      json.RawMessage            `json:"prompt"`
	Stream      bool                       `json:"stream,omitempty"`
	Temperature *float64                   `json:"temperature,omitempty"`
	TopP        *float64                   `json:"top_p,omitempty"`
	MaxTokens   *int                       `json:"max_tokens,omitempty"`
	Stop        json.RawMessage            `json:"stop,omitempty"`
	User        string                     `json:"user,omitempty"`
	Extra       map[string]json.RawMessage `json:"-"`
}

type completionsRequestJSON struct {
	Model       string          `json:"model"`
	Prompt      json.RawMessage `json:"prompt"`
	Stream      bool            `json:"stream,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
	MaxTokens   *int            `json:"max_tokens,omitempty"`
	Stop        json.RawMessage `json:"stop,omitempty"`
	User        string          `json:"user,omitempty"`
}

var completionsRequestKnownFields = []string{
	"model",
	"prompt",
	"stream",
	"temperature",
	"top_p",
	"max_tokens",
	"stop",
	"user",
}

func (r *CompletionsRequest) UnmarshalJSON(data []byte) error {
	var known completionsRequestJSON
	if err := json.Unmarshal(data, &known); err != nil {
		return err
	}
	extra, err := decodeExtraFields(data, completionsRequestKnownFields)
	if err != nil {
		return err
	}
	*r = CompletionsRequest{
		Model:       known.Model,
		Prompt:      known.Prompt,
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

func (r CompletionsRequest) MarshalJSON() ([]byte, error) {
	fields := copyRawFields(r.Extra, completionsRequestKnownFields)
	if err := putJSONField(fields, "model", r.Model); err != nil {
		return nil, err
	}
	if len(r.Prompt) > 0 {
		fields["prompt"] = cloneRawMessage(r.Prompt)
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

func (r CompletionsRequest) Validate() *Error {
	if strings.TrimSpace(r.Model) == "" {
		return InvalidRequest("missing required field: model", "model")
	}
	if !hasProcessableCompletionsPrompt(r.Prompt) {
		return InvalidRequest("missing required field: prompt", "prompt")
	}
	return nil
}

func hasProcessableCompletionsPrompt(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" || trimmed == "[]" {
		return false
	}
	return true
}

type CompletionsResponse struct {
	ID      string              `json:"id"`
	Object  string              `json:"object"`
	Created int64               `json:"created"`
	Model   string              `json:"model"`
	Choices []CompletionsChoice `json:"choices"`
	Usage   *Usage              `json:"usage,omitempty"`
}

type CompletionsChoice struct {
	Text         string `json:"text"`
	Index        int    `json:"index"`
	Logprobs     any    `json:"logprobs"`
	FinishReason string `json:"finish_reason"`
}

type CompletionsChunk struct {
	ID      string                   `json:"id"`
	Object  string                   `json:"object"`
	Created int64                    `json:"created"`
	Model   string                   `json:"model"`
	Choices []CompletionsChunkChoice `json:"choices"`
	Usage   *Usage                   `json:"usage,omitempty"`
}

type CompletionsChunkChoice struct {
	Text         string  `json:"text"`
	Index        int     `json:"index"`
	Logprobs     any     `json:"logprobs"`
	FinishReason *string `json:"finish_reason"`
}
