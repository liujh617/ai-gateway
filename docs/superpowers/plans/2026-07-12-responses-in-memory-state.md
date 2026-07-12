# Responses In-Memory State Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 `/v1/responses` 增加受容量约束、按客户端隔离的进程内 `previous_response_id` continuation。

**Architecture:** 新建线程安全的 `internal/responsestore`，保存规范化 Chat transcript 快照；API 层在 provider 调用前恢复历史，在完整成功后写入新快照。配置、指标和文档保持 provider 无状态，并明确单进程、重启丢失的边界。

**Tech Stack:** Go 标准库、`net/http`、`encoding/json`、现有 compat/router/provider/middleware、Shell smoke tests、WSL Ubuntu-24.04。

## Global Constraints

- 默认 TTL `3600` 秒、最多 `1000` 条、单 transcript `4194304` 字节、总计 `67108864` 字节。
- 任一 response-store 限制显式为 `0` 时整体禁用；负数必须使配置校验失败。
- 客户端、模型、缺失、过期和淘汰状态不得泄露 response ID 或其他客户端信息。
- 只有非流式完整成功和正常 `response.completed` 流才可写入；失败、超时、取消和 incomplete 不写入。
- 不新增第三方依赖；最终验证必须在 WSL `Ubuntu-24.04` 中运行 `make release-check`。
- 外部契约变更必须同步 spec、架构、Task 159、README、配置、验证文档和 changelog。

---

### Task 1: Response Store 核心

**Files:**
- Create: `internal/responsestore/store.go`
- Create: `internal/responsestore/store_test.go`

**Interfaces:**
- Produces: `Config{TTL time.Duration, MaxEntries int, MaxContextBytes int64, MaxTotalBytes int64}`、`Record{ID, Client, Model string; Transcript []compat.ChatMessage}`、`New(Config, Clock) *Store`、`Get(id, client, model string) (Record, MissReason, bool)`、`Put(Record) error`、`Snapshot() Stats`。
- Produces: miss reasons `not_found|expired|client|model`，eviction reasons `expired|capacity`，以及可供 API 映射为 400 的 `ErrContextTooLarge`、`ErrDisabled`。

- [ ] **Step 1: 写失败测试**

在 `store_test.go` 用可控 clock 和小容量表驱动测试：深拷贝、成功 Get 刷新 LRU 但不延长绝对 TTL、过期、客户端/模型隔离、entry/byte LRU 淘汰、4 MiB 类单项拒绝、分支读取、ID 冲突不覆盖及 50 goroutine 并发 Get/Put。

- [ ] **Step 2: 验证测试先失败**

Run: `go test ./internal/responsestore`
Expected: FAIL，因为 package/API 尚不存在。

- [ ] **Step 3: 实现最小 store**

使用 `map[string]*entry` 加 `container/list` LRU；JSON 编码和深拷贝在锁外完成，过期清理、容量淘汰和计数在锁内完成。`Snapshot` 先惰性清理并返回 entries、bytes、按 reason 累计的 eviction/miss。

- [ ] **Step 4: 验证单元测试和竞态**

Run: `go test -race ./internal/responsestore`
Expected: PASS，且无 race report。

- [ ] **Step 5: 提交**

```bash
git add internal/responsestore
git commit -m "feat: add in-memory response store"
```

### Task 2: 配置契约和启动装配

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/config/examples_test.go`
- Modify: `schema/config.schema.json`
- Modify: `config.example.json`
- Modify: `config.local.example.json`
- Modify: `config.azure-openai.example.json`
- Modify: `config.deepseek.example.json`
- Modify: `cmd/open-ai-gateway/main.go`
- Modify: `internal/api/server.go`

**Interfaces:**
- Consumes: `responsestore.Config`、`responsestore.New`。
- Produces: `config.ResponseStoreConfig`、默认值应用、`Enabled() bool` 和 check-config 中的生效摘要；`api.Options.ResponseStore *responsestore.Store`。

- [ ] **Step 1: 写失败配置测试**

增加缺省值、四个字段逐一为零禁用、逐一为负拒绝、schema/example 严格解析和 check-config 摘要断言。

- [ ] **Step 2: 验证失败**

Run: `go test ./internal/config ./cmd/open-ai-gateway`
Expected: FAIL，缺少 `response_store` 字段和摘要。

- [ ] **Step 3: 实现配置与装配**

为显式零与缺省配置段保留可区分状态；缺省段填充四项默认值，负数返回字段名明确的错误，任一零创建 disabled store 并记录一次安全 warning。将 store 注入 `api.Server`，不得注入 provider。

- [ ] **Step 4: 更新 schema 和所有示例**

schema 为四个 integer 字段设置 `minimum: 0` 且拒绝额外字段；示例写入默认配置，测试确保 JSON/schema/check-config 一致。

- [ ] **Step 5: 验证并提交**

Run: `go test ./internal/config ./cmd/open-ai-gateway`
Expected: PASS。

```bash
git add internal/config schema config*.json cmd/open-ai-gateway internal/api/server.go
git commit -m "feat: configure response state store"
```

### Task 3: 非流式 Continuation

**Files:**
- Modify: `internal/compat/responses.go`
- Modify: `internal/compat/responses_test.go`
- Modify: `internal/api/responses.go`
- Modify: `internal/api/responses_test.go`

**Interfaces:**
- Consumes: `Store.Get`、`Store.Put`、客户端 context、现有 Responses-to-Chat 转换。
- Produces: request 字段 `PreviousResponseID *string`、`Store *bool`；规范化 transcript 合并和成功响应快照。

- [ ] **Step 1: 写失败测试**

覆盖两轮文本、function call/output 回放、同 previous 分支、默认存储、`store:false` 可读不可写、instructions/tools 不继承、客户端 404、模型 400、disabled 400、context-too-large 400，以及上游错误/超时不写入。

- [ ] **Step 2: 验证失败**

Run: `go test ./internal/compat ./internal/api -run 'Responses|ResponseStore'`
Expected: FAIL，request 尚不接受状态字段。

- [ ] **Step 3: 实现读取和稳定错误映射**

在 handler 调用 provider 前读取 previous，合并深拷贝的历史消息和当前 input；将 not-found/expired/client 统一映射为 404 `previous response not found`，模型、disabled、超限映射为稳定 400，`param` 均为 `previous_response_id`。

- [ ] **Step 4: 实现成功快照写入**

非流式完整响应后，把历史、本轮输入和统一模型输出组成 transcript；只有 `store` 缺省/true 才以本次 response ID 写入。保存外部模型名和 context 中客户端名，不保存 instructions/tools/control fields。

- [ ] **Step 5: 验证并提交**

Run: `go test -race ./internal/compat ./internal/api`
Expected: PASS。

```bash
git add internal/compat/responses.go internal/compat/responses_test.go internal/api/responses.go internal/api/responses_test.go
git commit -m "feat: continue stored Responses conversations"
```

### Task 4: 流式完成边界

**Files:**
- Modify: `internal/api/responses.go`
- Modify: `internal/api/responses_test.go`

**Interfaces:**
- Consumes: Task 3 transcript builder 和 `Store.Put`。
- Produces: 仅在正常输出 `response.completed` 后写入的 stream transcript。

- [ ] **Step 1: 写失败流测试**

分别断言正常文本流、正常 function 流可作为下一轮 previous；provider stream error、缺失终止、read timeout 和客户端 cancel 的 response ID 均返回 404。

- [ ] **Step 2: 验证失败**

Run: `go test ./internal/api -run 'Responses.*(Stream|Previous)'`
Expected: FAIL，流式路径尚未提交快照。

- [ ] **Step 3: 实现流式累积与原子提交**

复用现有 chunk 聚合构造 assistant 文本/function call；只有上游正常结束并成功写出 `response.completed` 后调用 Put。任何提前 return、context cancellation 或写错误都丢弃局部 transcript。

- [ ] **Step 4: 验证并提交**

Run: `go test -race ./internal/api -run Responses`
Expected: PASS。

```bash
git add internal/api/responses.go internal/api/responses_test.go
git commit -m "feat: store completed Responses streams"
```

### Task 5: 指标、日志与审计安全

**Files:**
- Modify: `internal/middleware/metrics.go`
- Modify: `internal/middleware/metrics_test.go`
- Modify: `internal/api/server.go`
- Modify: `internal/api/responses.go`
- Modify: `internal/api/responses_test.go`
- Modify: `internal/audit/event.go`
- Modify: `internal/audit/audit_test.go`

**Interfaces:**
- Consumes: `Store.Snapshot()`。
- Produces: 四组 `open_ai_gateway_response_store_*` 指标；访问日志布尔值 `previous_response`；审计事件可选 previous response ID。

- [ ] **Step 1: 写失败可观测性测试**

断言 entries/bytes gauge、expired/capacity eviction、四种 miss reason；普通日志只出现 `previous_response=true` 且不出现 ID/transcript，审计按现有策略包含 ID；readiness 在 store disabled/full 时仍成功。

- [ ] **Step 2: 验证失败**

Run: `go test ./internal/middleware ./internal/audit ./internal/api`
Expected: FAIL，指标和字段尚不存在。

- [ ] **Step 3: 实现指标与安全字段**

Prometheus 输出从 store snapshot 获取无高基数数据；只允许固定 reason 标签。request audit event 设置 previous ID，访问日志 context 只传播 boolean。

- [ ] **Step 4: 验证并提交**

Run: `go test -race ./internal/middleware ./internal/audit ./internal/api`
Expected: PASS 且敏感 ID 不出现在普通日志 fixture。

```bash
git add internal/middleware internal/audit internal/api
git commit -m "feat: observe response state safely"
```

### Task 6: Smoke、兼容文档和发布验证

**Files:**
- Create: `scripts/smoke-responses-state.sh`
- Create: `tasks/159-responses-in-memory-state.md`
- Modify: `Makefile`
- Modify: `openai-compatible-proxy-spec.md`
- Modify: `architecture/overview.md`
- Modify: `README.md`
- Modify: `docs/local-verification.md`
- Modify: `docs/release.md`
- Modify: `CHANGELOG.md`

**Interfaces:**
- Consumes: 完整状态 API、配置和指标。
- Produces: `make smoke-responses-state`，并纳入 `make release-check`。

- [ ] **Step 1: 写离线 smoke**

脚本启动本地 fixture/provider，先验证文本两轮只提交新增 input，再验证 function call/output continuation；同时检查 store:false 和未知 previous 的稳定错误，不依赖外网或真实 API key。

- [ ] **Step 2: 接入 Makefile 并验证 smoke**

Run: `wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "make smoke-responses-state"`
Expected: PASS，输出文本和 function continuation 检查成功。

- [ ] **Step 3: 同步全部外部文档**

明确默认存储、非继承字段、404/400 契约、单进程和重启丢失、安全边界、容量配置、指标及 smoke 命令；Task 159 逐项记录验收结果。

- [ ] **Step 4: 执行完整 WSL 验证**

Run: `wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "make release-check"`
Expected: 所有 Go tests、race、schema、smoke 和文档检查 PASS。

- [ ] **Step 5: 检查变更并提交**

Run: `git diff --check && git status --short`
Expected: 无 whitespace error，只有本 Task 的预期文件。

```bash
git add scripts/smoke-responses-state.sh tasks/159-responses-in-memory-state.md Makefile openai-compatible-proxy-spec.md architecture/overview.md README.md docs/local-verification.md docs/release.md CHANGELOG.md
git commit -m "docs: document Responses conversation state"
```

### Task 7: 最终回归和分支交付

**Files:**
- Verify only: entire repository

**Interfaces:**
- Consumes: Tasks 1–6 的所有提交。
- Produces: 可审阅、可推送的完整 Task 159 分支。

- [ ] **Step 1: 运行最终验证**

Run: `wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "make verify && make release-check"`
Expected: 两个目标均 PASS。

- [ ] **Step 2: 核对历史和工作树**

Run: `git status --short; git log --oneline master..HEAD`
Expected: 工作树 clean，提交按 store、config、nonstream、stream、observability、docs 顺序排列。

- [ ] **Step 3: 请求代码审查**

使用 `superpowers:requesting-code-review` 对照本计划和设计文档审查；发现问题时先补失败测试，再修复并重跑对应测试和完整 release-check。
