# Task 162 - Provider Health Read Lock

## Status

Done.

## Context

Provider health is checked on hot request paths before routing attempts. The state object is process-local and protected by one mutex.

The common path is a registered, currently healthy provider. Before this task, even that read-only case acquired the write lock.

## Changes

- Changed provider health locking to `sync.RWMutex`.
- Let `Healthy` return healthy/unhealthy fast-path decisions under a read lock when no mutation is needed.
- Keep write locking for provider registration, failure/success transitions, missing-provider state creation, and cooldown recovery mutation.
- Added direct unit coverage for provider registration through `Healthy`, cooldown recovery, and concurrent access.

## Acceptance

- WSL `go test ./internal/api -run ProviderHealth -count=1` passes.
- WSL `make verify` passes.
