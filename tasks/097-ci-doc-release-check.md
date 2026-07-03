# Task 097 - CI release check docs

## Status

Done.

## Context

CI already runs `make release-check`, and that target now includes default
config validation plus fake-provider smoke checks. The CI documentation should
describe the current gate instead of the older partial command list.

## Changes

- Updated `docs/ci.md` to use `make release-check` as the local equivalent.
- Documented the release-check sub-steps, including `check-config` and `smoke`.
- Clarified that smoke uses the fake provider and does not require real API
  keys.

## Verification

- `make verify`
