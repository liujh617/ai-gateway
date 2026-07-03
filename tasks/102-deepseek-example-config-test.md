# Task 102 - DeepSeek example config test

## Status

Done.

## Context

`config.deepseek.example.json` is checked by the shell-based config example
script, but it should also be part of the direct Go example config load test so
all repository example configs share the same coverage path.

## Changes

- Added `config.deepseek.example.json` to `TestExampleConfigsLoad`.

## Verification

- `make check-config-examples`
- `make verify`
