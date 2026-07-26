package wsproxy

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"
)

// ObserverMetrics tracks realtime session metrics without blocking the relay.
type ObserverMetrics struct {
	SessionCreated    int64
	ResponseCompleted int64
	AudioInputBytes   int64
	AudioOutputBytes  int64
	TokenInput        int64
	TokenOutput       int64
	Errors            int64
	DroppedEvents     int64
}

// ObserverCallbacks receives parsed realtime protocol events.
// All callbacks are called from the relay goroutines and must not block.
type ObserverCallbacks struct {
	OnSessionCreated func(sessionID, model string)
	OnResponseDone   func(responseID string, usage map[string]any)
	OnAudioDelta     func(direction string, bytes int)
	OnError          func(errorType, message string)
}

// Observer non-intrusively observes WebSocket message traffic.
type Observer struct {
	callbacks ObserverCallbacks
	metrics   ObserverMetrics
	mu        sync.Mutex
}

// NewObserver creates a protocol observer with the given callbacks.
func NewObserver(callbacks ObserverCallbacks) *Observer {
	return &Observer{callbacks: callbacks}
}

// Metrics returns a snapshot of the current metrics.
func (o *Observer) Metrics() ObserverMetrics {
	return ObserverMetrics{
		SessionCreated:    atomic.LoadInt64(&o.metrics.SessionCreated),
		ResponseCompleted: atomic.LoadInt64(&o.metrics.ResponseCompleted),
		AudioInputBytes:   atomic.LoadInt64(&o.metrics.AudioInputBytes),
		AudioOutputBytes:  atomic.LoadInt64(&o.metrics.AudioOutputBytes),
		TokenInput:        atomic.LoadInt64(&o.metrics.TokenInput),
		TokenOutput:       atomic.LoadInt64(&o.metrics.TokenOutput),
		Errors:            atomic.LoadInt64(&o.metrics.Errors),
		DroppedEvents:     atomic.LoadInt64(&o.metrics.DroppedEvents),
	}
}

// ObserveClientMessage parses a message from client to server.
func (o *Observer) ObserveClientMessage(msg []byte) {
	if o == nil || o.callbacks.OnAudioDelta == nil {
		return
	}
	var event struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(msg, &event) != nil {
		return
	}
	switch event.Type {
	case "input_audio_buffer.append":
		var audio struct {
			Audio string `json:"audio"`
		}
		if json.Unmarshal(msg, &audio) == nil {
			atomic.AddInt64(&o.metrics.AudioInputBytes, int64(len(audio.Audio)))
			o.callbacks.OnAudioDelta("client", len(audio.Audio))
		}
	}
}

// ObserveServerEvent parses a message from server to client.
func (o *Observer) ObserveServerEvent(msg []byte) {
	if o == nil {
		return
	}
	var event struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(msg, &event) != nil {
		return
	}
	switch event.Type {
	case "session.created":
		var session struct {
			Session struct {
				ID    string `json:"id"`
				Model string `json:"model"`
			} `json:"session"`
		}
		if json.Unmarshal(msg, &session) == nil {
			atomic.AddInt64(&o.metrics.SessionCreated, 1)
			if o.callbacks.OnSessionCreated != nil {
				o.callbacks.OnSessionCreated(session.Session.ID, session.Session.Model)
			}
		}
	case "response.done":
		var resp struct {
			Response struct {
				ID    string         `json:"id"`
				Usage map[string]any `json:"usage"`
			} `json:"response"`
		}
		if json.Unmarshal(msg, &resp) == nil {
			atomic.AddInt64(&o.metrics.ResponseCompleted, 1)
			if u := resp.Response.Usage; u != nil {
				if input, ok := u["input_tokens"].(float64); ok {
					atomic.AddInt64(&o.metrics.TokenInput, int64(input))
				}
				if output, ok := u["output_tokens"].(float64); ok {
					atomic.AddInt64(&o.metrics.TokenOutput, int64(output))
				}
			}
			if o.callbacks.OnResponseDone != nil {
				o.callbacks.OnResponseDone(resp.Response.ID, resp.Response.Usage)
			}
		}
	case "response.audio.delta":
		var audio struct {
			Delta string `json:"delta"`
		}
		if json.Unmarshal(msg, &audio) == nil {
			atomic.AddInt64(&o.metrics.AudioOutputBytes, int64(len(audio.Delta)))
			if o.callbacks.OnAudioDelta != nil {
				o.callbacks.OnAudioDelta("server", len(audio.Delta))
			}
		}
	case "error":
		var errEvent struct {
			Error struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(msg, &errEvent) == nil {
			atomic.AddInt64(&o.metrics.Errors, 1)
			if o.callbacks.OnError != nil {
				o.callbacks.OnError(errEvent.Error.Type, errEvent.Error.Message)
			}
		}
	}
}

// Now is a time function (used for timestamping).
var Now = time.Now
