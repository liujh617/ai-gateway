# Task 092 - Smoke client metrics label

## Status

Done.

## Context

Client-aware observability should be validated through the same local smoke flow
used for release checks, not only through unit tests.

## Changes

- Added a smoke assertion that request metrics include the configured
  `client="default"` label after authenticated API requests.

## Verification

- `make smoke`
- `make verify`
