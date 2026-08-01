package conversation

import "encoding/json"

type Request struct {
	Model           string
	Stream          bool
	Turn            Turn
	Tools           []FunctionTool
	ToolChoice      json.RawMessage
	ReasoningEffort string
	Extra           map[string]json.RawMessage
}

type Turn struct {
	Items []Item
}

type Item interface {
	isConversationItem()
}

type RouteBinding struct {
	Dialect       string
	Provider      string
	UpstreamModel string
}

type Message struct {
	Role string
	Text string
}

func (Message) isConversationItem() {}

type Reasoning struct {
	EnvelopeID       string
	Content          string
	AssistantContent string
	CallIDs          []string
	Route            RouteBinding
}

func (Reasoning) isConversationItem() {}

type FunctionCall struct {
	CallID              string
	Name                string
	Arguments           string
	ReasoningEnvelopeID string
}

func (FunctionCall) isConversationItem() {}

type FunctionOutput struct {
	CallID string
	Output json.RawMessage
}

func (FunctionOutput) isConversationItem() {}

type Opaque struct {
	Kind string
	Data json.RawMessage
}

func (Opaque) isConversationItem() {}

type FunctionTool struct {
	Name        string
	Description string
	Parameters  json.RawMessage
	Strict      bool
}

type Limits struct {
	MaxItems            int
	MaxToolCallsPerTurn int
	MaxReasoningBytes   int
}

func CloneRequest(request Request) Request {
	clone := request
	clone.ToolChoice = cloneRaw(request.ToolChoice)
	clone.Extra = cloneRawMap(request.Extra)
	clone.Tools = make([]FunctionTool, len(request.Tools))
	for index, tool := range request.Tools {
		clone.Tools[index] = tool
		clone.Tools[index].Parameters = cloneRaw(tool.Parameters)
	}
	clone.Turn.Items = make([]Item, len(request.Turn.Items))
	for index, raw := range request.Turn.Items {
		switch item := raw.(type) {
		case Message:
			clone.Turn.Items[index] = item
		case Reasoning:
			item.CallIDs = append([]string(nil), item.CallIDs...)
			clone.Turn.Items[index] = item
		case FunctionCall:
			clone.Turn.Items[index] = item
		case FunctionOutput:
			item.Output = cloneRaw(item.Output)
			clone.Turn.Items[index] = item
		case Opaque:
			item.Data = cloneRaw(item.Data)
			clone.Turn.Items[index] = item
		default:
			clone.Turn.Items[index] = raw
		}
	}
	return clone
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
