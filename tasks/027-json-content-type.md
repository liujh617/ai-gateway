# Task 027: JSON Content-Type Validation

## 状态

Done

## 背景

网关的 `POST` JSON 接口已经只接受单个 JSON 请求体，但此前没有校验 `Content-Type`。生产环境中显式拒绝缺失或错误的媒体类型，可以更早发现客户端接入问题，并让 HTTP 契约更清晰。

## 范围

实现：

- chat completions 校验 `Content-Type`。
- embeddings 校验 `Content-Type`。
- 允许 `application/json`。
- 允许 `application/json` 携带标准参数，例如 `charset=utf-8`。
- 缺失或非 JSON 媒体类型返回 `415 invalid_request_error`。
- 同步 API spec。

暂不实现：

- 对 `GET`/`HEAD` 接口做 Content-Type 校验。
- 对响应 `Accept` header 做协商。
- 要求请求必须显式声明 charset。

## 验收标准

- chat completions 缺失或错误 `Content-Type` 返回 `415`。
- embeddings 缺失或错误 `Content-Type` 返回 `415`。
- `application/json; charset=utf-8` 正常通过。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
