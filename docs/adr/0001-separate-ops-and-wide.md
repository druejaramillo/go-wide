# 0001 - Separate Ops And Wide

**Date:** 2026-06-08
**Status:** Accepted

## Context

The library needs to support two distinct concerns:

1. operation lifecycle, metadata, and observer propagation
2. `slog` log capture, aggregation, and wide-event emission

An earlier direction wrapped `slog` with a custom logging abstraction, but that duplicated a standard interface and mixed concerns that should remain separate. The repo also included vendor-specific code alongside the core library, which would force unnecessary dependencies into the base module.

## Decision

The core module is split into two primary packages:

- `ops` owns generic operation lifecycle, metadata, explicit operation error state, and observer registration/propagation
- `wide` owns `slog` handling, wide-event collection, normalization, and emission strategies

`wide` remains inside the `slog` universe by implementing a `slog.Handler` that wraps a downstream handler.

Vendor-specific integrations are not part of the core module. They live in separate modules that import `go-wide` and implement its extension points.

## Consequences

The core library keeps a clean dependency direction and no vendor lock-in.

`ops` remains useful even when wide-event collection is not attached.

`wide` can evolve as a logging-focused package without turning every operation into a logging artifact.

External integrations such as Sentry can implement observer interfaces without becoming required dependencies of the core module.
