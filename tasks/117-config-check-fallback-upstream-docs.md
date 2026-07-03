# Task 117 - Config check fallback upstream docs

## Status

Done.

## Context

Config check output now normalizes fallback `upstream_model` values in the same
way as primary model summaries. Public docs should mention that the summary
shows effective upstream model names.

## Changes

- Updated the API contract config-check section.
- Updated the README config-check description.

## Verification

- `make verify`
