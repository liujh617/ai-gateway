# Task 101 - Config schema core contract

## Status

Done.

## Context

The config schema should remain strict and keep the core sections used by the
gateway configuration contract. Valid JSON alone would not catch accidental
removal of required top-level fields or key definitions.

## Changes

- Added schema structure coverage for:
  - top-level `additionalProperties: false`
  - required `providers` and `models`
  - key `$defs` used by current config fields

## Verification

- `make check-config-examples`
- `make verify`
