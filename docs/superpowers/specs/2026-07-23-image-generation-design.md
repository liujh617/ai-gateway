# `/v1/images/generations` API 设计

## 背景

网关当前支持 Chat Completions、Completions、Embeddings、Responses 和 Models 端点。图像生成 (`POST /v1/images/generations`) 是 OpenAI API 的常用端点，允许通过文本 prompt 生成图像。

本任务新增 JSON 请求/响应的图像生成端点，复用现有 model router、provider fallback、circuit breaker 和 metrics 基础设施。不涉及 multipart/image edit/variations 端点。

## API 契约

新增受保护路由：

- `POST /v1/images/generations`

### Request

JSON body，`Content-Type: application/json`：

```json
{
  "model": "dall-e-3",
  "prompt": "a white siamese cat",
  "n": 1,
  "size": "1024x1024",
  "quality": "standard",
  "response_format": "url",
  "style": "vivid",
  "user": "user-123"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `model` | string | 是 | 模型名称 |
| `prompt` | string | 是 | 图像描述 |
| `n` | integer | 否 | 生成数量，默认 1 |
| `size` | string | 否 | 图像尺寸，如 `1024x1024` |
| `quality` | string | 否 | 图像质量 |
| `response_format` | string | 否 | `url` 或 `b64_json` |
| `style` | string | 否 | 风格 |
| `user` | string | 否 | 终端用户标识 |

路由使用 `images` capability。`model` 缺失或 `prompt` 为空返回 `400 invalid_request_error`。

### Response

返回 `200 application/json`：

```json
{
  "created": 1699000000,
  "data": [
    {
      "url": "https://...",
      "b64_json": null,
      "revised_prompt": "a white siamese cat sitting on a windowsill"
    }
  ]
}
```

不强制覆盖上游 `created` 字段；其他未列举字段通过 Extra 透传到上游。

## 错误语义

- 缺失 model 或空 prompt → `400 invalid_request_error`。
- 模型无 `images` capability → `404 model_not_found`。
- 鉴权失败 → `401 authentication_error`。
- 不支持的方法 → `405 invalid_request_error` + `Allow: POST`。
- 上游错误按现有 `providerError` 映射。

## 设计

### Compat 类型

新增 `compat.ImageGenerationRequest` 和 `compat.ImageGenerationResponse`，遵循现有 known-fields + Extra 透传模式。

### Provider 接口

`Provider` 接口新增 `CreateImage(ctx, req) (*ImageGenerationResponse, error)`。

### Provider 实现

- **fake**: 返回固定 image URL，支持可选 `Err` 错误注入。
- **openai**: `POST {baseURL}/images/generations`，JSON body，JSON response。
- **azureopenai**: `POST {baseURL}/openai/deployments/{deployment}/images/generations?api-version=...`。

### API Handler

复用现有模式：
1. JSON Content-Type 校验
2. JSON body 解码
3. req.Validate()
4. model 白名单校验
5. router.ResolveFor("images")
6. audit request event
7. 创建 request timeout context
8. createImageWithFallback（主 provider + fallback + circuit breaker）
9. audit response event
10. 返回 JSON 响应

不支持流式（图像生成无 SSE）。

## 测试策略

### Compat

- `ImageGenerationRequest` 序列化/反序列化保持未知字段。
- `Validate()` 拒绝空 model 和空 prompt。
- `ImageGenerationResponse` JSON 输出符合 OpenAI 格式。

### Routes

- `/v1/images/generations` path normalization。
- POST allowed，GET/HEAD/DELETE 拒绝。
- Allow header 返回 `POST`。
- 非公开路由。

### API

- 成功非流式响应的 status、content type 和 JSON 字段。
- 缺失 model、空 prompt 返回 400。
- 鉴权失败返回 401。
- 模型无 images capability 返回 404。
- 不支持的方法返回 405 + `Allow: POST`。
- provider 错误正确转换。
- provider circuit open 返回 503。
- provider fallback 可切换。
- audit 记录 request/response event。

### 集成

- fake provider smoke test。

## 文档同步

- `openai-compatible-proxy-spec.md`
- `README.md`
- `CHANGELOG.md`
- `tasks/172-image-generation.md`

## 非目标

- 不支持 `/v1/images/edits` 和 `/v1/images/variations`（需要 multipart form-data）。
- 不支持流式响应。
- 不支持 `dall-e-2` 特有的 `n > 1` 验证（由上游 provider 处理）。
