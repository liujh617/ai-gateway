# Task 099 - Release doc default config check

## Status

Done.

## Context

`make release-check` now includes default config validation through
`make check-config`. Release documentation and the original release process task
should describe the current gate.

## Changes

- Updated `docs/release.md` to include `make check-config`.
- Updated `tasks/019-release-process.md` to mention the default config check.

## Verification

- `make verify`
