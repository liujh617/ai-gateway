package wsproxy

import (
	"context"
	"time"

	"github.com/coder/websocket"
)

// Config controls WebSocket relay behavior.
type Config struct {
	MaxMessageBytes int64
	MaxDuration     time.Duration
	MaxConcurrency  int
	AllowedOrigins  []string
}

// Stats holds connection statistics after relay completes.
type Stats struct {
	ClientSentBytes   int64
	UpstreamSentBytes int64
	ClientMessages    int64
	UpstreamMessages  int64
	Duration          time.Duration
}

// Relay performs a full-duplex relay between client and upstream WebSocket connections.
// If observer is non-nil, messages are passed to it for protocol-level observability.
func Relay(ctx context.Context, client, upstream *websocket.Conn, cfg Config, obs *Observer) Stats {
	start := time.Now()
	stats := &relayStats{}
	done := make(chan struct{}, 2)

	readConn := func(dst, src *websocket.Conn, sentBytes, msgCount *int64, isClient bool) {
		defer func() { done <- struct{}{} }()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			msg, err := readMessage(src, cfg.MaxMessageBytes)
			if err != nil {
				return
			}
			if err := writeMessage(dst, msg); err != nil {
				return
			}
			*sentBytes += int64(len(msg))
			*msgCount++
			if obs != nil {
				if isClient {
					obs.ObserveClientMessage(msg)
				} else {
					obs.ObserveServerEvent(msg)
				}
			}
		}
	}

	var deadlineCancel context.CancelFunc
	if cfg.MaxDuration > 0 {
		ctx, deadlineCancel = context.WithTimeout(ctx, cfg.MaxDuration)
		defer deadlineCancel()
	}

	go readConn(client, upstream, &stats.clientSent, &stats.clientMsgs, true)
	go readConn(upstream, client, &stats.upstreamSent, &stats.upstreamMsgs, false)

	<-done
	if deadlineCancel != nil {
		deadlineCancel()
	}
	<-done

	client.Close(websocket.StatusNormalClosure, "")
	upstream.Close(websocket.StatusNormalClosure, "")

	return Stats{
		ClientSentBytes:   stats.clientSent,
		UpstreamSentBytes: stats.upstreamSent,
		ClientMessages:    stats.clientMsgs,
		UpstreamMessages:  stats.upstreamMsgs,
		Duration:          time.Since(start),
	}
}

type relayStats struct {
	clientSent   int64
	upstreamSent int64
	clientMsgs   int64
	upstreamMsgs int64
}

func readMessage(conn *websocket.Conn, maxBytes int64) ([]byte, error) {
	_, msg, err := conn.Read(context.Background())
	if err != nil {
		return nil, err
	}
	if maxBytes > 0 && int64(len(msg)) > maxBytes {
		return nil, websocket.CloseError{Code: websocket.StatusMessageTooBig}
	}
	return msg, nil
}

func writeMessage(conn *websocket.Conn, msg []byte) error {
	return conn.Write(context.Background(), websocket.MessageBinary, msg)
}
