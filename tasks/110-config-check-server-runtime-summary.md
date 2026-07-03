# Task 110 - Config check server runtime summary

## Status

Done.

## Context

Server timeout and request body settings are non-sensitive runtime values that
operators often need to confirm before deployment. They should be visible in
the same `check-config` summary as rate limit and provider health settings.

## Changes

- Added request, stream, HTTP server timeout, shutdown timeout, and max request
  body settings to config check output.
- Extended config check tests for snake_case JSON and Go report values.
- Updated local verification docs.

## Verification

- `make check-config`
- `make verify`
