# Task 103 - Release check DeepSeek skip

## Status

Done.

## Context

`make smoke-deepseek` supports a no-key skip path so CI and local release checks
can verify the script without calling a real external provider. The release
gate should exercise that safe path explicitly.

## Changes

- Added `make smoke-deepseek-skip`, which clears `DEEPSEEK_API_KEY` before
  running the DeepSeek smoke script.
- Added `smoke-deepseek-skip` to `make release-check`.
- Updated CI and release docs to describe the new safe smoke check.

## Verification

- `make smoke-deepseek-skip`
- `make release-check`
