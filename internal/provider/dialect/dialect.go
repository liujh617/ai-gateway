package dialect

import (
	"errors"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/conversation"
)

var (
	ErrReasoningRequired           = errors.New("reasoning item is required for this tool call")
	ErrReasoningRouteMismatch      = errors.New("reasoning item route does not match provider route")
	ErrIncompleteReasoningToolCall = errors.New("provider returned an incomplete reasoning tool call")
)

type Capabilities struct {
	ReasoningReplay         bool
	ProducesReasoning       bool
	BufferStreamUntilFinish bool
}

type Request struct {
	Conversation  conversation.Request
	UpstreamModel string
	Route         conversation.RouteBinding
}

type Response struct {
	Turn  conversation.Turn
	Usage *compat.Usage
}

type StreamEvent struct {
	TextDelta         string
	FunctionCallDelta *FunctionCallDelta
	Usage             *compat.Usage
}

type FunctionCallDelta struct {
	Index     int
	CallID    string
	Name      string
	Arguments string
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
