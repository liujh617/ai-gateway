# Task 161 - Response Store Lock Hold Time

## Status

Done.

## Context

`/v1/responses` conversation state is stored in an in-memory TTL/LRU store. The store uses one mutex to protect the entry map, LRU list, byte accounting, and miss/eviction counters.

Before this task, `Get` returned a deep copy while still holding the store mutex. This preserved caller isolation, but large stored transcripts could make unrelated response-state requests wait on clone work that does not need exclusive access to the map or LRU metadata.

## Changes

- Keep `Get` locking around lookup, expiry checks, client/model isolation, miss counters, and LRU refresh.
- Move the returned record deep clone after unlocking the store mutex.
- Preserve the existing deep-copy behavior so callers cannot mutate stored transcript memory.

## Acceptance

- Existing response store immutability and concurrency tests pass.
- WSL `go test ./internal/responsestore -count=1` passes.
- WSL `make verify` passes.
