# Task 182 - Realtime API v0.2.1（浏览器 + 限流）

## 背景

v0.2.0 实现了基于 Authorization Header 的 WebSocket 鉴权。但浏览器的 `WebSocket` API 不支持自定义 Header，需要基于 URL token 的鉴权方案。同时需要连接配额和速率限制来防止滥用。

## 端点

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | /v1/realtime/tokens | 创建短期访问 token |
| GET | /v1/realtime | 支持 `?token=xxx` 参数鉴权（已有，新增） |

## 短期 Token 方案

### 流程

1. 客户端通过标准 HTTP `POST /v1/realtime/tokens` 请求短期 token
2. 请求使用标准 `Authorization: Bearer <api-key>` Header
3. 网关返回 `client_secret`（短期随机 token，默认 300 秒过期）
4. 浏览器使用 `wss://gateway/v1/realtime?model=xxx&token=<client_secret>` 连接
5. 网关验证 token 有效性（存在、未过期、client 匹配）

### 数据结构

```go
type RealtimeToken struct {
    ClientSecret string `json:"client_secret"`
    ExpiresAt    int64  `json:"expires_at"`
    TTL          int    `json:"ttl_seconds"`
}

type RealtimeTokenRequest struct {
    Model    string `json:"model"`
    TTL      int    `json:"ttl_seconds,omitempty"` // 默认 300，最大 3600
}
```

### Token Store

```
internal/realtimetoken/token.go
  - 进程内 map[token]TokenRecord
  - TTL 过期清理
  - client 隔离
```

### 鉴权流程更新

```go
func (s *Server) handleRealtime(w, r) {
    // 1. 先检查 Authorization Header（v0.2.0 方式）
    // 2. 若无 Header，检查 ?token= 参数
    // 3. 验证 token 有效性
    // 4. 继续原有路由/中继逻辑
}
```

## 连接配额

### 数据结构

```go
type RealtimeConfig struct {
    Enabled         bool
    MaxMessageBytes int64
    MaxDurationSec  int
    MaxConcurrency  int       // 全局并发上限（已有）
    MaxPerClient    int       // 每个 client 的并发上限
    MaxConnPerMin   int       // 每个 client 每分钟连接数上限
    AllowedOrigins  []string
}
```

### ClientQuota

```go
type ClientQuota struct {
    mu           sync.Mutex
    connections  int
    windowStart  time.Time
    windowCount  int
    maxConcurrent int
    maxPerMinute int
}

func (q *ClientQuota) Allow() bool  // 检查并发和速率
func (q *ClientQuota) Acquire()     // 占用
func (q *ClientQuota) Release()     // 释放
```

### 集成

- `internal/api/realtime.go`：`handleRealtime` 在 Acquire 全局 limiter 后，再检查 client 配额
- client 标识：`clientFromContext(r.Context())` 或 token 绑定的 client

## 新建文件

```
internal/realtimetoken/
  token.go        # Token 生成/验证/过期
  token_test.go   # 测试
internal/api/
  realtime_token.go       # POST /v1/realtime/tokens handler
  realtime_token_test.go  # 测试
```

## 修改文件

```
internal/api/realtime.go    # 新增 token 鉴权路径 + client 配额检查
internal/api/server.go      # 注册 /v1/realtime/tokens 路由
internal/routes/routes.go   # RealtimeTokensPath
internal/wsproxy/limiter.go # ClientQuota（或新建 quota.go）
internal/config/config.go   # RealtimeConfig 新增 MaxPerClient、MaxConnPerMin
schema/config.schema.json   # Schema 更新
```

## 测试

1. `TestRealtimeTokenCreate`：创建 token → 200 + client_secret
2. `TestRealtimeTokenAuth`：创建 token → WebSocket 连接 + `?token=xxx` → 成功
3. `TestRealtimeTokenExpired`：过期 token → 401
4. `TestRealtimeTokenWrongClient`：其他 client 使用 → 401
5. `TestRealtimeClientQuota`：超过 MaxPerClient → 429
6. `TestRealtimeRateLimit`：超过 MaxConnPerMin → 429

## 配置示例

```json
{
  "realtime": {
    "enabled": true,
    "max_message_bytes": 1048576,
    "max_duration_seconds": 1800,
    "max_concurrency": 100,
    "max_per_client": 5,
    "max_conn_per_minute": 10,
    "allowed_origins": ["https://example.com"]
  }
}
```

## 验证

- `go test ./...`
