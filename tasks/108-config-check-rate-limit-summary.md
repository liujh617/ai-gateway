# Task 108 - Config check rate limit summary

## Status

Done.

## Context

Per-client rate limit overrides already appear in config check output, but the
global default `rate_limit.requests_per_minute` did not. Operators should be
able to confirm the active default limit from the same non-sensitive summary.

## Changes

- Added top-level `rate_limit_requests_per_minute` to config check output.
- Extended config check tests to cover the snake_case JSON field and Go report
  value.
- Updated local verification docs.

## Verification

- `make check-config`
- `make verify`
