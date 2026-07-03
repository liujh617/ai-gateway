# Task 118 - OpenAI list models trailing JSON

## Status

Done.

## Context

OpenAI-compatible provider responses are decoded as a single JSON value. Chat
and embeddings already had explicit trailing-response coverage; list models
should have the same boundary test.

## Changes

- Added `TestListModelsRejectsTrailingJSONResponse`.

## Verification

- `go test ./internal/provider/openai`
- `make verify`
