# Task 093 - Rate limit unconfigured client coverage

## Status

Done.

## Context

When gateway authentication is disabled, protected API routes still pass through
rate limiting. The rate limiter should use the synthetic `unconfigured` client
label instead of silently falling back to remote address buckets.

## Changes

- Added middleware coverage for the no-credentials path.
- Verified that different remote addresses share the `unconfigured` client
  bucket.
- Verified that rate limit rejection observation uses client label
  `unconfigured`.

## Verification

- `go test ./internal/middleware`
- `make verify`
