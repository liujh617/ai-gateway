# Task 096 - Release check default config

## Status

Done.

## Context

`make check-config` validates the default runtime configuration path, while
`make check-config-examples` validates example files. The release gate should
cover both.

## Changes

- Added `check-config` to `make release-check`.

## Verification

- `make check-config`
- `make release-check`
