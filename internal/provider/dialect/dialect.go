package dialect

import (
	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/conversation"
)

type Capabilities struct {
	ReasoningReplay bool
}

type Request struct {
	Conversation  conversation.Request
	UpstreamModel string
}

type Response struct {
	Turn  conversation.Turn
	Usage *compat.Usage
}

type StreamEvent struct {
	TextDelta string
	Usage     *compat.Usage
}

type StreamDecoder interface {
	Push(compat.ChatCompletionChunk) ([]StreamEvent, error)
	Finish() (Response, error)
}

type Dialect interface {
	Name() string
	Capabilities() Capabilities
	BuildChatRequest(Request) (compat.ChatCompletionRequest, error)
	ParseChatResponse(compat.ChatCompletionResponse) (Response, error)
	NewStreamDecoder() StreamDecoder
}
