# Task 181 - Realtime API WebSocket 架构

## 版本路线

| 版本 | 内容 |
|---|---|
| v0.2.0 | 方案 A+：消息级透明代理 |
| v0.2.1 | 浏览器 client_secret + 连接配额/限流 |
| v0.2.x | 协议 Observer：关键事件审计 + 指标 |

---

## v0.2.0：方案 A+ 消息级透明代理

### 技术选型

- **WebSocket 库**：`github.com/coder/websocket`（高性能、低分配、支持 context）
- **鉴权**：标准 HTTP Authorization Header（与 HTTP 端点一致）
- **路由**：`model` query param + `ResolveByCapability("realtime")`
- **Fallback**：上游握手阶段重试（连接建立前可切换 provider）
- **审计**：连接级事件（connect/disconnect + 字节数 + 时长）
- **Metrics**：并发连接数、消息数、带宽
- **安全**：消息大小上限、连接时长上限、并发上限、Origin 白名单

### 新建文件

```
internal/wsproxy/
  proxy.go        # WebSocket 中继核心
  limiter.go      # 并发/带宽/消息大小控制
  proxy_test.go   # 测试
internal/api/
  realtime.go     # HTTP Upgrade handler + 鉴权 + 路由 + fallback
internal/api/
  realtime_test.go # API 层测试
```

### 修改文件

```
internal/provider/provider.go   # RealtimeProvider 接口
internal/provider/openai/openai.go    # openai RealtimeEndpoint
internal/provider/azureopenai/azureopenai.go  # azure RealtimeEndpoint
internal/provider/fake/fake.go         # fake 桩（echo server）
internal/routes/routes.go    # RealtimePath
internal/api/server.go       # 注册 handler
internal/router/model_router.go  # ResolveByCapability（已存在）
internal/config/config.go    # realtime 能力
schema/config.schema.json    # schema
cmd/gateway/main.go          # 可能调整 server 配置
go.mod                       # 新增 coder/websocket 依赖
```

### 核心实现

#### `internal/wsproxy/proxy.go`

```go
type Config struct {
    MaxMessageSize    int64         // 单消息上限，默认 1MB
    MaxDuration       time.Duration // 连接最长时长，默认 30min
    MaxConcurrency    int           // 全局并发连接上限
    AllowedOrigins    []string      // Origin 白名单
}

type Stats struct {
    ClientSentBytes   int64
    UpstreamSentBytes int64
    ClientMessages    int64
    UpstreamMessages  int64
    Duration          time.Duration
    ConnectedAt       time.Time
}

// Relay 在 client 和 upstream 之间全双工中继 WebSocket 消息
// 返回连接统计（字节数、消息数、时长）
func Relay(ctx context.Context, client, upstream *websocket.Conn, cfg Config) Stats
```

实现要点：
- 两个 goroutine：client→upstream 和 upstream→client
- `ReadMessage` 替代 `io.Copy`，实现消息大小控制
- 任一方向读失败时，用状态码关闭另一方向
- 超时或消息超限时优雅关闭
- 返回完整 Stats

#### `internal/wsproxy/limiter.go`

```go
type ConnectionLimiter struct { ... }
func NewConnectionLimiter(max int) *ConnectionLimiter
func (l *ConnectionLimiter) Acquire() bool
func (l *ConnectionLimiter) Release()
```

#### `internal/provider/provider.go`

```go
type RealtimeProvider interface {
    RealtimeEndpoint(model string) string // wss://... 或 https://...（gateway 负责升级）
}
```

fake 实现：启动本地 `net/http` server，接受 WebSocket 升级并 echo 消息。

#### `internal/api/realtime.go`

```go
func (s *Server) handleRealtime(w http.ResponseWriter, r *http.Request)
```

流程：

1. **鉴权**：`r.Header.Get("Authorization")` → gateway API key 校验
2. **Origin 检查**：`r.Header.Get("Origin")` 对白名单
3. **路由**：`r.URL.Query().Get("model")` → `ResolveByCapability("realtime")`
4. **并发控制**：`limiter.Acquire()` → 超出上限返回 503
5. **上游握手**：从 `RealtimeProvider.RealtimeEndpoint()` 获取上游 URL
6. **Fallback**：握手失败时尝试下一个具备 realtime 能力的 provider
7. **HTTP Upgrade**：升级客户端连接为 WebSocket
8. **中继**：`wsproxy.Relay(ctx, clientConn, upstreamConn, cfg)`
9. **审计**：记录 connect/disconnect 事件 + Stats（字节数、时长）
10. **Metrics**：并发连接计数、带宽

#### Handshake Fallback 逻辑

```go
func (s *Server) dialRealtimeWithFallback(model string) (*websocket.Conn, string, error) {
    for each route attempt with realtime capability:
        url := provider.RealtimeEndpoint(model)
        conn, err := websocket.Dial(ctx, url, opts)
        if err == nil:
            return conn, providerName, nil
        if non-retryable error (e.g. 401):
            return nil, "", err
        // retry next provider
    }
    return nil, "", lastErr
}
```

### 鉴权流程

```
客户端                          网关
  |                               |
  |-- GET /v1/realtime?model=xxx -|
  |   Authorization: Bearer xxx   |
  |   Origin: https://example.com |
  |                               |-- 鉴权 ✅
  |                               |-- Upgrade: 101 Switching Protocols
  |<======== WebSocket ==========>|
```

WebSocket 握手使用标准 HTTP 请求，因此 Authorization Header 直接可用。
客户端无需在 URL 中传 API key。

### 配置

```json
{
  "realtime": {
    "enabled": true,
    "max_message_bytes": 1048576,
    "max_duration_seconds": 1800,
    "max_concurrency": 100,
    "allowed_origins": []
  }
}
```

Env overrides：
- `GATEWAY_REALTIME_MAX_MESSAGE_BYTES`
- `GATEWAY_REALTIME_MAX_DURATION_SECONDS`
- `GATEWAY_REALTIME_MAX_CONCURRENCY`
- `GATEWAY_REALTIME_ALLOWED_ORIGINS`（逗号分隔）

### 测试策略

1. **wsproxy 单元测试**：`TestRelay` 双向中继、`TestRelayMessageSizeLimit`、`TestRelayTimeout`
2. **fake provider**：本地 WebSocket echo server，支持 `RealtimeEndpoint`
3. **API 层测试**：
   - 鉴权拒绝 → 401
   - 无 realtime 能力 → 404
   - Origin 不符合白名单 → 403
   - 并发超限 → 503
   - 正常中继 → 200 + Stats
4. **Handshake fallback**：第一个 provider 失败时切换到第二个

---

## v0.2.1：浏览器支持 + 限流

### 浏览器短期 client_secret

浏览器 WebSocket 不支持自定义 Authorization Header。提供短期 token 机制：

```json
// POST /v1/realtime/tokens
{"model": "gpt-4o-realtime", "client_secret": "temp-token-xxx", "expires_in": 300}
```

浏览器端使用 `wss://gateway/v1/realtime?token=temp-token-xxx`。

### 连接配额和限流

- 每个 client 的并发连接上限
- 连接速率限制（connects/minute/client）
- 带宽配额（bytes/minute/client）

---

## v0.2.x：协议 Observer

在不修改中继逻辑的前提下，通过 Observer 模式解析消息流：

```go
type Observer struct {
    OnSessionCreated  func(event)
    OnResponseDone    func(event)
    OnAudioDelta      func(duration)
    OnError           func(event)
}

// Relay 新增 observer 参数
func Relay(ctx context.Context, client, upstream *websocket.Conn, cfg Config, obs *Observer) Stats
```

Observer 非侵入式读取消息流，记录：

- **审计**：session.created、response.done、error 事件
- **Metrics**：token 用量、音频时长、首字节延迟
- **不阻断中继**：Observer 失败不影响数据流

---

## v0.2.0 实施步骤

1. 新增 `github.com/coder/websocket` 依赖（go get）
2. 创建 `internal/wsproxy/proxy.go` + `limiter.go`
3. 创建 `internal/wsproxy/proxy_test.go`
4. 新增 `RealtimeProvider` 接口 + fake/openai/azureopenai 实现
5. 创建 `internal/api/realtime.go`
6. 注册路由 + 配置 + schema
7. 创建 `internal/api/realtime_test.go`
8. 全量测试 + make verify
