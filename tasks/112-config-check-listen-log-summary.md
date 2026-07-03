# Task 112 - Config check listen and log summary

## Status

Done.

## Context

The config check summary already includes server runtime values, but listen
address and log mode are also non-sensitive startup settings that operators
often verify before deployment.

## Changes

- Added `addr` to config check output.
- Added `log_format` and `log_level` to config check output.
- Extended config check tests for snake_case JSON and Go report values.
- Updated local verification docs.

## Verification

- `make check-config`
- `make verify`
