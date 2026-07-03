# Task 114 - Config check provider timeout summary

## Status

Done.

## Context

Provider timeout is a non-sensitive upstream runtime setting. It should appear
in `check-config` provider summaries so operators can confirm the effective
timeout before starting the gateway.

## Changes

- Added `timeout_seconds` to provider summaries in config check output.
- Extended config check tests for snake_case JSON and Go report values.

## Verification

- `make check-config`
- `make verify`
