# Task 185：Codex reasoning dialect bridge

## 目标

让 Codex 经 `/v1/responses` 使用 DeepSeek thinking + function tools，支持 `store:false`、无 `previous_response_id`、非流式/流式和跨实例续接，并为后续 Kimi/GLM dialect 保留清晰扩展点。

## 设计与计划

- 设计：`docs/superpowers/specs/2026-08-01-codex-reasoning-dialect-design.md`
- 实施计划：`docs/superpowers/plans/2026-08-01-codex-reasoning-dialect.md`
- 决策：`docs/adr/0002-codex-reasoning-dialect-bridge.md`

## 验收矩阵

- encrypted envelope：round trip、tamper、TTL、client/model/audience binding、active/previous key。
- conversation/compat：reasoning/call/output 顺序、重复和未知 call ID、稳定错误。
- DeepSeek dialect：thinking 参数、reasoning replay、非空 assistant content、stream 聚合和不完整响应。
- API：无状态两轮工具调用、stream `[DONE]`、route pinning、store、跨 Server、取消和无明文泄漏。
- 启动：dialect registry、能力预检、key env 解析和配置错误拒绝启动。
- 回归：`make verify`、`make smoke-deepseek`。

## 回滚边界

回滚可移除 reasoning route 配置并恢复 `openai-compatible` dialect；已有 encrypted reasoning item 无法在不具备对应 key/dialect 的旧版本继续，客户端需要新建 task。不得降级为忽略或伪造 reasoning。
