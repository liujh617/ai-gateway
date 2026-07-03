# Task 119 - OpenAI list models upstream error

## Status

Done.

## Context

Chat and embeddings provider calls already cover upstream OpenAI-compatible
error mapping. List models uses the same mapping path and should have explicit
coverage too.

## Changes

- Added `TestListModelsMapsUpstreamError`.

## Verification

- `go test ./internal/provider/openai`
- `make verify`
