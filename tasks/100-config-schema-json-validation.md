# Task 100 - Config schema JSON validation

## Status

Done.

## Context

`schema/config.schema.json` is part of the configuration contract. The example
config check should also catch an invalid schema JSON file, not only invalid
example runtime configs.

## Changes

- Added a config package test that verifies `schema/config.schema.json` is
  valid JSON.
- The existing `make check-config-examples` path now runs this validation
  through `go test ./internal/config`.

## Verification

- `make check-config-examples`
- `make verify`
