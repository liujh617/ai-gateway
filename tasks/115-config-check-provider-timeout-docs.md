# Task 115 - Config check provider timeout docs

## Status

Done.

## Context

Provider summaries in config check output now include `timeout_seconds`. The
public docs should mention this non-sensitive provider summary field.

## Changes

- Updated the config-check section in the API contract.
- Updated the README config-check description.

## Verification

- `make verify`
