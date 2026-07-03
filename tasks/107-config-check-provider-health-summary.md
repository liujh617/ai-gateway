# Task 107 - Config check provider health summary

## Status

Done.

## Context

Provider health settings control the circuit breaker behavior. They are
non-sensitive runtime settings and should be visible in `check-config` output
so operators can confirm the active thresholds before release or deployment.

## Changes

- Added `provider_health_failure_threshold` to config check output.
- Added `provider_health_cooldown_seconds` to config check output.
- Extended config check tests to verify snake_case JSON output and no unstable
  Go field names.
- Updated local verification docs.

## Verification

- `make check-config`
- `make verify`
