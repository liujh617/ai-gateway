# Task 159 - Responses 进程内会话状态

## 状态

Done.

## 目标

- 支持 `/v1/responses` 的 `previous_response_id` 和默认 `store: true`。
- 使用有 TTL、条目数和字节上限的进程内 store 保存规范化 Chat transcript。
- 保持客户端和外部模型隔离，并支持文本及 function call/output 续接。
- 仅保存完整成功的非流式响应和正常 `response.completed` 流。
- 增加低基数 metrics、安全日志和离线 `make smoke-responses-state`。

## 验收

- 客户端只发送新增 input 即可续接已存储响应。
- `store: false` 可以读取已有 previous response，但不保存当前响应。
- 顶层 instructions、tools 和采样控制不跨轮继承。
- 不存在、过期、淘汰或属于其他客户端的 ID 返回统一 404。
- 模型不一致、store 禁用和上下文超限返回稳定 400。
- 重启会丢失状态；容量满时先清理过期项，再按 LRU 淘汰。
- WSL `make verify`、`make smoke-responses-state` 和 `make release-check` 通过。
