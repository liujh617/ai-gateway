package anthropic

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

// MessageStream reads Anthropic streaming message events from a reader.
type MessageStream struct {
	scanner *bufio.Scanner
}

// NewMessageStream creates a stream reader for Anthropic SSE-like events.
func NewMessageStream(reader io.Reader) *MessageStream {
	s := &MessageStream{scanner: bufio.NewScanner(reader)}
	s.scanner.Buffer(nil, 1<<20) // 1MB
	return s
}

// Next returns the next stream event, or io.EOF when the stream ends.
func (s *MessageStream) Next() (*MessageStreamEvent, error) {
	for s.scanner.Scan() {
		line := s.scanner.Text()
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "event: ") {
			continue
		}
		if strings.HasPrefix(line, "data: ") {
			payload := strings.TrimPrefix(line, "data: ")
			var event MessageStreamEvent
			if err := json.Unmarshal([]byte(payload), &event); err != nil {
				return nil, err
			}
			return &event, nil
		}
	}
	if err := s.scanner.Err(); err != nil {
		return nil, err
	}
	return nil, io.EOF
}
