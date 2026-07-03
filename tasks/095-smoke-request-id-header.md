# Task 095 - Smoke request id header

## Status

Done.

## Context

The gateway contract says a valid client-supplied `X-Request-Id` is reused and
returned in the response. Unit tests cover the middleware, and the smoke flow
should verify the behavior through the running HTTP service.

## Changes

- Added a smoke assertion that `HEAD /healthz` echoes a valid
  `X-Request-Id` header.

## Verification

- `make smoke`
- `make verify`
