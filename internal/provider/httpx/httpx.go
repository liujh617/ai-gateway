package httpx

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"strings"

	"open-ai-gateway/internal/compat"
)

const MaxResponseBodyBytes = 10 << 20

// ChatCompletionStream parses SSE events for /v1/chat/completions.
type ChatCompletionStream struct {
	body          io.ReadCloser
	reader        *bufio.Reader
	seenFirstLine bool
	skipNextLF    bool
}

func NewChatCompletionStream(body io.ReadCloser) *ChatCompletionStream {
	return &ChatCompletionStream{
		body:   body,
		reader: bufio.NewReader(body),
	}
}

func (s *ChatCompletionStream) Next(ctx context.Context) (*compat.ChatCompletionChunk, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		payload, err := s.nextPayload()
		if err != nil {
			return nil, err
		}
		if payload == "" {
			continue
		}
		if payload == "[DONE]" {
			return nil, io.EOF
		}
		var chunk compat.ChatCompletionChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return nil, err
		}
		return &chunk, nil
	}
}

func (s *ChatCompletionStream) Close() error {
	return s.body.Close()
}

// CompletionStream parses SSE events for /v1/completions.
type CompletionStream struct {
	body          io.ReadCloser
	reader        *bufio.Reader
	seenFirstLine bool
	skipNextLF    bool
}

func NewCompletionStream(body io.ReadCloser) *CompletionStream {
	return &CompletionStream{
		body:   body,
		reader: bufio.NewReader(body),
	}
}

func (s *CompletionStream) Next(ctx context.Context) (*compat.CompletionsChunk, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		payload, err := s.nextPayload()
		if err != nil {
			return nil, err
		}
		if payload == "" {
			continue
		}
		if payload == "[DONE]" {
			return nil, io.EOF
		}
		var chunk compat.CompletionsChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return nil, err
		}
		return &chunk, nil
	}
}

func (s *CompletionStream) Close() error {
	return s.body.Close()
}

// Internal helpers for ChatCompletionStream

func (s *ChatCompletionStream) nextPayload() (string, error) {
	return readSSEPayload(s.reader, &s.seenFirstLine, &s.skipNextLF)
}

func (s *ChatCompletionStream) readSSELine() (string, error) {
	var line strings.Builder
	return readRawSSELine(s.reader, &s.skipNextLF, &line)
}

// Internal helpers for CompletionStream

func (s *CompletionStream) nextPayload() (string, error) {
	return readSSEPayload(s.reader, &s.seenFirstLine, &s.skipNextLF)
}

func (s *CompletionStream) readSSELine() (string, error) {
	var line strings.Builder
	return readRawSSELine(s.reader, &s.skipNextLF, &line)
}

func readSSEPayload(reader *bufio.Reader, seenFirstLine *bool, skipNextLF *bool) (string, error) {
	var data []string
	eventBytes := 0
	for {
		var b strings.Builder
		line, err := readRawSSELine(reader, skipNextLF, &b)
		if err != nil {
			if errors.Is(err, io.EOF) {
				if line == "" {
					if len(data) == 0 {
						return "", io.EOF
					}
				}
				return "", errors.New("upstream SSE event ended without blank line")
			}
			return "", err
		}

		line = strings.TrimRight(line, "\r\n")
		if !*seenFirstLine {
			*seenFirstLine = true
			line = strings.TrimPrefix(line, "\ufeff")
		}
		if line == "" {
			eventBytes = 0
			if len(data) == 0 {
				continue
			}
			return strings.Join(data, "\n"), nil
		}
		eventBytes += len(line) + 1
		if eventBytes > MaxResponseBodyBytes {
			return "", fmt.Errorf("upstream SSE event exceeds %d bytes", MaxResponseBodyBytes)
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, _ := strings.Cut(line, ":")
		if strings.HasPrefix(value, " ") {
			value = strings.TrimPrefix(value, " ")
		}
		switch field {
		case "data":
			data = append(data, value)
		case "event", "id", "retry":
			continue
		default:
			continue
		}
	}
}

func readRawSSELine(reader *bufio.Reader, skipNextLF *bool, line *strings.Builder) (string, error) {
	for {
		b, err := reader.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				if line.Len() == 0 {
					return "", io.EOF
				}
				return line.String(), io.EOF
			}
			return "", TransportError(err)
		}
		if *skipNextLF {
			*skipNextLF = false
			if b == '\n' {
				continue
			}
		}
		if line.Len()+1 > MaxResponseBodyBytes+1 {
			return "", fmt.Errorf("upstream SSE line exceeds %d bytes", MaxResponseBodyBytes)
		}
		line.WriteByte(b)
		if b == '\n' {
			return line.String(), nil
		}
		if b == '\r' {
			*skipNextLF = true
			return line.String(), nil
		}
	}
}

func DecodeLimited(r io.Reader, out any) error {
	limited := &io.LimitedReader{R: r, N: MaxResponseBodyBytes + 1}
	decoder := json.NewDecoder(limited)
	if err := decoder.Decode(out); err != nil {
		return err
	}
	if limited.N <= 0 {
		return fmt.Errorf("upstream response body exceeds %d bytes", MaxResponseBodyBytes)
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("upstream response body must contain a single JSON value")
		}
		return err
	}
	if limited.N <= 0 {
		return fmt.Errorf("upstream response body exceeds %d bytes", MaxResponseBodyBytes)
	}
	return nil
}

func RequireJSONResponse(resp *http.Response) error {
	if !ResponseContentTypeIs(resp, "application/json") {
		return fmt.Errorf("upstream response Content-Type must be application/json")
	}
	return nil
}

func RequireEventStreamResponse(resp *http.Response) error {
	if !ResponseContentTypeIs(resp, "text/event-stream") {
		return fmt.Errorf("upstream response Content-Type must be text/event-stream")
	}
	return nil
}

func ResponseContentTypeIs(resp *http.Response, want string) bool {
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && strings.EqualFold(mediaType, want)
}

func TransportError(err error) error {
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return fmt.Errorf("upstream request timeout: %w", context.DeadlineExceeded)
	}
	return err
}

func UpstreamError(resp *http.Response) error {
	var upstream compat.ErrorResponse
	if ResponseContentTypeIs(resp, "application/json") {
		var decoded compat.ErrorResponse
		if err := DecodeLimited(resp.Body, &decoded); err == nil {
			upstream = decoded
		}
	}

	message := http.StatusText(resp.StatusCode)
	if upstream.Error.Message != "" {
		message = upstream.Error.Message
	}

	errorType := upstream.Error.Type
	if errorType == "" {
		errorType = defaultErrorType(resp.StatusCode)
	}
	status := resp.StatusCode
	if resp.StatusCode >= 500 && resp.StatusCode != http.StatusGatewayTimeout {
		status = http.StatusBadGateway
	}
	return &compat.Error{
		Status:  status,
		Message: message,
		Type:    errorType,
		Param:   upstream.Error.Param,
		Code:    upstream.Error.Code,
	}
}

func defaultErrorType(status int) string {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return "authentication_error"
	case http.StatusTooManyRequests:
		return "rate_limit_error"
	case http.StatusBadRequest, http.StatusNotFound:
		return "invalid_request_error"
	default:
		return "server_error"
	}
}
