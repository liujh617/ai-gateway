# Task 104 - Smoke rate limit

## Status

Done.

## Context

Gateway rate limiting has unit and API coverage, but the release smoke flow did
not verify the behavior through a running service. A separate fake-provider
smoke keeps the main happy-path smoke uncluttered while checking the external
HTTP contract.

## Changes

- Added `scripts/smoke-rate-limit.sh`.
- Added `make smoke-rate-limit`.
- Added the rate limit smoke to `make release-check`.
- Updated CI and release docs.

## Verification

- `make smoke-rate-limit`
- `make release-check`
