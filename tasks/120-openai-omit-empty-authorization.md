# Task 120 - OpenAI omit empty authorization

## Status

Done.

## Context

OpenAI-compatible providers may be used with local upstreams that do not require
an API key. In that case the adapter should omit `Authorization` instead of
sending an empty bearer token.

## Changes

- Added `TestListModelsOmitsAuthorizationWithoutAPIKey`.

## Verification

- `go test ./internal/provider/openai`
- `make verify`
