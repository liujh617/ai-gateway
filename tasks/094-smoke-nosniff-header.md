# Task 094 - Smoke nosniff header

## Status

Done.

## Context

`X-Content-Type-Options: nosniff` is part of the gateway HTTP contract and is
already covered by API tests. The local smoke flow should also verify the
header on the actual running service.

## Changes

- Added smoke assertions for `X-Content-Type-Options: nosniff` on:
  - `HEAD /healthz`
  - `HEAD /metrics`
  - unauthenticated `GET /v1/models`

## Verification

- `make smoke`
- `make verify`
