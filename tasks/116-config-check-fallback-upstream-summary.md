# Task 116 - Config check fallback upstream summary

## Status

Done.

## Context

Model summaries already show the effective upstream model when
`models.<name>.upstream_model` is omitted. Fallback summaries should do the same
so `check-config` output reflects the actual routing target.

## Changes

- Added `ModelFallbackSummary` for config check output.
- Normalized omitted fallback `upstream_model` values to the external model
  name in config check summaries.
- Extended config check tests for the fallback summary JSON and Go report.

## Verification

- `make check-config`
- `make verify`
