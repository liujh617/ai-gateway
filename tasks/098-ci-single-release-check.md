# Task 098 - CI single release check

## Status

Done.

## Context

`make release-check` already runs verification, config checks, build, and fake
smoke tests. Running `make verify`, `make check-config-examples`, and
`make build` as separate CI steps before `make release-check` duplicates work
and makes the workflow easier to drift.

## Changes

- Simplified GitHub Actions CI to run `make release-check` as the single
  verification gate.
- Kept the Gitee workflow in sync.
- Updated CI documentation to match the workflow.

## Verification

- `make release-check`
