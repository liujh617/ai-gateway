# Changelog

All notable changes to this project are documented here.

## Unreleased

### Added

### Changed

## 0.1.7 - 2026-07-23

### Added

- Added `POST /v1/images/generations` endpoint with JSON request/response, reusing existing model router, provider fallback, circuit breaker, and metrics infrastructure.
- Added `POST /v1/moderations` endpoint with JSON request/response.

## 0.1.6 - 2026-07-21

### Added

- Populated `duration_ms` in audit events (response, stream_done, error) by capturing request start time in the RequestID middleware and computing elapsed milliseconds in `auditBaseEvent`.

### Fixed

- Fixed completions streaming `[DONE]` sentinel being JSON-encoded with quotes (`data: "[DONE]"`) instead of the OpenAI-compatible plain form (`data: [DONE]`).
- Fixed `countingCompletionProvider` test helper field/method name conflict (`calls` field vs `calls()` method) that prevented compilation.
- Fixed completions 503 tests that incorrectly expected 200 from always-failing providers before the circuit breaker opened.
- Added missing `CreateCompletion`/`StreamCompletion` stubs to `responseFunctionStateProvider` and `functionStreamProvider` test mocks.

## 0.1.5 - 2026-07-20

### Added

- Added `POST /v1/completions` endpoint with JSON and SSE streaming support, using existing model router, provider fallback, circuit breaker, and metrics infrastructure.

## 0.1.4 - 2026-07-16

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
