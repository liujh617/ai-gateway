# Task 105 - Local verification smoke docs

## Status

Done.

## Context

The local verification guide should list the smoke targets that are now part of
the release gate, including the rate limit smoke and the safe DeepSeek skip
path.

## Changes

- Added `make smoke-rate-limit` to local verification docs.
- Added `make smoke-deepseek-skip` to local verification docs.
- Updated the original release process task to describe the current
  release-check smoke set.

## Verification

- `make verify`
