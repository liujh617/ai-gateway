# 单模型查询 API 设计

## 背景

网关已经提供 `GET /v1/models`，可列出当前 gateway client 有权访问的模型，但尚未提供 OpenAI Models API 的单模型查询端点。部分 SDK 和调用方会通过模型 ID 查询模型元数据，因此需要补充 `GET /v1/models/{model}`，同时沿用项目对 GET 路由支持 HEAD 的约定。

本任务只查询网关静态模型路由，不向上游 provider 发起请求，也不扩展模型元数据字段。

## API 契约

新增受保护路由：

- `GET /v1/models/{model}`
- `HEAD /v1/models/{model}`

`{model}` 必须是一个非空路径段。`/v1/models/`、包含额外路径段的地址以及其他不匹配模板的地址继续按未知路由处理。

成功的 GET 请求返回与 `GET /v1/models` 列表元素相同的 `compat.Model`：

```json
{
  "id": "test-model",
  "object": "model",
  "created": 0,
  "owned_by": "open-ai-gateway"
}
```

成功的 HEAD 请求返回与 GET 相同的状态码和 `Content-Type: application/json`，但不返回 body。

## 鉴权和可见性

该路由不是 public route，必须通过现有 Bearer token 鉴权。

handler 先检查当前 client 的 model allowlist，再查询模型路由。模型不存在和模型对当前 client 不可见时，都返回现有稳定的 `404 invalid_request_error` model-not-found 响应。两种情况使用相同响应，避免向无权访问的 client 泄露隐藏模型是否存在。

查询只读取本地路由元数据，不检查 provider health，也不调用 provider。

## 路由和观测

使用 Go 1.22 `net/http` ServeMux 的 `GET /v1/models/{model}` 模式注册路由，并通过 `r.PathValue("model")` 读取模型 ID。Go 的 GET 模式同时匹配 HEAD；handler 负责省略 HEAD body。

`internal/routes` 将 `/v1/models/{model}` 作为规范化路径。以下能力必须识别具体实例路径，例如 `/v1/models/test-model`：

- `NormalizePath` 返回 `/v1/models/{model}`，避免日志和 metrics 使用真实模型 ID 形成高基数标签。
- `AllowedMethods`、`MethodAllowed` 和 `AllowHeader` 返回 GET、HEAD 契约。
- `IsPublicPath` 仍返回 false。

动态匹配仅接受 `/v1/models/` 后的单个非空路径段，不使用通用 catch-all，也不改变其他路由的精确匹配语义。

## 组件变更

### Router

`ModelRouter` 增加按外部模型 ID 读取 `compat.Model` 元数据的方法。该方法只判断路由是否存在并构造与 `Models()` 一致的模型对象，不要求 provider 可用，也不暴露 provider 或 upstream model 信息。

### API

`Server` 注册单模型 handler。handler 的处理顺序为：

1. 从路径读取模型 ID。
2. 检查当前 client allowlist。
3. 从 `ModelRouter` 读取模型元数据。
4. 不可见或不存在时写入统一 model-not-found 错误。
5. 设置 JSON content type；HEAD 到此结束，GET 编码模型对象。

### 文档

实现时同步更新：

- `openai-compatible-proxy-spec.md`
- `architecture/overview.md`
- `README.md`
- `CHANGELOG.md`
- `tasks/163-retrieve-model.md`

## 错误和方法语义

- 缺少或错误 Bearer token：`401 authentication_error`。
- 模型不存在：`404 invalid_request_error`。
- 模型不在 client allowlist：与不存在相同的 `404 invalid_request_error`。
- 已知动态路由使用其他方法：`405 invalid_request_error`，`Allow: GET, HEAD`。
- 不匹配动态模板的路径：现有 JSON `404 invalid_request_error` route-not-found 响应。

## 测试策略

按 TDD 增加以下覆盖：

- route metadata 能识别并规范化单模型具体路径。
- 空模型段和额外路径段不匹配。
- GET 返回正确模型对象。
- HEAD 返回正确响应头且 body 为空。
- 未鉴权请求返回 401。
- 未知模型返回 404。
- allowlist 内模型可查询，allowlist 外模型返回与未知模型一致的 404。
- POST 等不支持的方法返回 JSON 405 和 `Allow: GET, HEAD`。
- 现有模型列表、其他已知路由和未知路由行为不回归。

最终在 WSL `Ubuntu-24.04` 中执行 `make verify`。

## 非目标

- 不代理上游 `GET /models/{model}`。
- 不查询 provider health 或模型实时可用性。
- 不增加删除模型或修改模型端点。
- 不扩展 `created`、`owned_by` 等模型元数据来源。
- 不实现 Responses 查询或删除端点。
