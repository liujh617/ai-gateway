# Task 179 - Responses API Conversations

## 背景

OpenAI Responses API 的核心差异化功能是 Conversations——自动管理对话上下文，无需客户端手动拼接消息历史。当前 Responses API 只支持基础文本、函数工具和流式，缺少 Conversations 支持。

## 端点复用

复用现有 `/v1/responses` 端点，通过 `conversation` 字段启用：

```json
{"model": "gpt-4o", "input": "hello", "conversation": "conv_abc"}
```

或创建新 conversation：
```json
{"model": "gpt-4o", "input": "hello", "conversation": {}}
```

## 类型

- `Conversation`：id, object, created_at, metadata
- `ConversationItem`：id, object, conversation_id, input, output

## 变更

- `internal/compat/responses.go`：新增 conversation 类型、扩展 ResponseRequest
- `internal/responsestore/`：扩展存储支持 conversations
- `internal/api/responses.go`：conversation 创建和延续逻辑
- `internal/api/responses_test.go`：conversation 测试
- `openai-compatible-proxy-spec.md`：更新规范

## 验证

- `go test ./...`
- `make responses-smoke`
