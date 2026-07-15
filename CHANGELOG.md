# Changelog

All notable changes to this project are documented here.

## Unreleased

### Added

- Added authenticated retrieval of completed, locally stored Responses through `GET` and `HEAD /v1/responses/{response_id}` with client isolation and normalized observability paths.
- Added authenticated single-model metadata retrieval through `GET` and `HEAD /v1/models/{model}`, including client allowlist enforcement and normalized observability paths.
- Added authenticated deletion of locally stored Responses through `DELETE /v1/responses/{response_id}`, with client isolation, non-cascading semantics, and correct store accounting.

### Changed

- Reduced `/v1/responses` in-memory state store lock hold time by cloning stored transcripts after releasing the store mutex.
- Reduced provider-health hot-path lock contention by using a read lock for non-mutating health checks.

## 0.1.3 - 2026-07-13

### Added

- Stateless text-only `/v1/responses` compatibility with string/message inputs, `instructions`, non-streaming output Items, and typed SSE text streaming.
- Credential-free Responses API smoke coverage included in release checks.
- Responses function tools with strict schemas, common tool choices, parallel calls, stateless function outputs, and typed arguments streaming.
- In-memory Responses `previous_response_id` continuation with bounded TTL/LRU storage, client/model isolation, completed-stream persistence, safe metrics, and offline smoke coverage.

## 0.1.2 - 2026-07-12

### Added

- Credential-free local Azure OpenAI smoke coverage for chat completions, streaming chat completions, and embeddings, included in release checks.
- Repository-enforced LF line endings and automated LF/CRLF checks for shell scripts.

### Fixed

- Stabilized stream read timeout race coverage by replacing timing-dependent duplicate tests with deterministic shared HTTP stream tests.

## 0.1.1 - 2026-07-11

### Added

- Azure OpenAI provider with deployment endpoints for chat completions, streaming chat completions, and embeddings.
- Azure OpenAI example configuration and JSON Schema support.

### Changed

- Extracted shared provider HTTP response, error, timeout, and SSE parsing helpers.

## 0.1.0 - 2026-07-11

### Added

- OpenAI-compatible `/v1/chat/completions` with streaming support.
- OpenAI-compatible `/v1/embeddings`.
- Static model routing with model capability constraints.
- OpenAI-compatible upstream provider.
- Fake provider for local development and tests.
- Bearer token authentication.
- Request timeouts, stream timeouts, rate limiting, structured logs, metrics, readiness, health, and version endpoints.
- JSON configuration, configuration self-check, JSON Schema, local verification scripts, CI workflows, Dockerfile, and deployment docs.
