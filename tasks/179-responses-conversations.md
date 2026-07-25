# Task 179 - Responses API Conversations ✅

## 背景

OpenAI Responses API 需要 conversation 支持来自动管理对话上下文。

## 变更

- `internal/compat/responses.go`：ResponseRequest.Conversation 字段 (json.RawMessage)、ConversationID() helper、Response.ConversationID 字段
- `internal/responsestore/store.go`：Record.ConversationID 字段、conversations 索引、ConversationResponses() 查询、DeleteByID 清除
- `internal/api/responses.go`：conversation-aware responseHistory（从对话构建历史）、store.Put/Get 传递 ConversationID
- `openai-compatible-proxy-spec.md`：更新 conversation 支持文档

## 验证

- `go test ./...` ✅
