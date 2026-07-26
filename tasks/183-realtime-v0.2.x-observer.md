# Task 183 - Realtime API v0.2.x Protocol Observer

## 背景

v0.2.0 实现了连接级审计（connect/disconnect + 字节数）。v0.2.1 实现了浏览器 token 和配额。
v0.2.x 增加消息级可观测性：解析 Realtime 协议消息，记录关键事件和指标。

## 设计

### 非侵入式 Observer 模式

Observer 在 Relay 的两个 goroutine 中并行观察消息流，不修改、不阻断数据流：

```
client → [Observer: 审计 + metrics] → upstream
client ← [Observer: 审计 + metrics] ← upstream
```

Observer 失败（解析错误、审计写入失败等）不影响数据中继。

### Observer 接口

```go
type Observer struct {
    OnClientMessage func(msg []byte, direction string)
    OnServerEvent   func(eventType string, fields map[string]any)
    OnError         func(err error)
}
```

### 协议事件

Realtime 协议使用 JSON 事件流。关键事件类型：

| 事件 | 方向 | 审计 | Metrics |
|---|---|---|---|
| `session.created` | server→client | ✅ session_id, model | — |
| `session.updated` | server→client | ✅ | — |
| `response.created` | server→client | ✅ response_id | — |
| `response.done` | server→client | ✅ usage, status | token usage |
| `response.audio.delta` | server→client | — | audio duration |
| `response.text.delta` | server→client | — | — |
| `input_audio_buffer.append` | client→server | — | audio duration |
| `error` | server→client | ✅ error message | — |

### 防阻塞设计

```go
func NewObserver(auditRecorder audit.Recorder) *Observer {
    return &Observer{
        audit: auditRecorder,
        events: make(chan auditEvent, 100), // 缓冲通道
    }
}

func (o *Observer) start() {
    for event := range o.events {
        o.audit.Record(ctx, event)
    }
}
```

缓冲区满时丢弃新事件（不阻塞中继），记录丢弃计数。

## 实施

### 修改文件

```
internal/wsproxy/proxy.go   # Relay 新增 observer 参数
internal/wsproxy/observer.go # Observer 实现
internal/wsproxy/observer_test.go # 测试
internal/api/realtime.go    # 创建 Observer 并传给 Relay
```

### 验证

- `go test ./...`
- Observer 不阻塞 Relay 的测试
