# 0002 - Root-Owned Wide Collection

**Date:** 2026-06-08
**Status:** Accepted

## Context

Wide events need to summarize nested work across a request or other operation tree. The system must support:

- explicit operation boundaries via `ops.Start` / `ops.End`
- persistent operation metadata
- `slog.Logger.With` / `WithGroup` semantics
- root-scoped aggregation
- passthrough fallback when no aggregation is active
- safe concurrent use across goroutines

The design also needs to preserve normal `slog` expectations: logger-derived state applies only to the returned logger, not globally to the operation.

## Decision

A root operation owns the wide collector tree for the whole operation hierarchy.

The context carries immutable operation scope. Child operations reference nodes inside the root-owned tree instead of owning independent buffers.

`logger.With(...)` and `logger.WithGroup(...)` remain handler-local immutable fragments. They are materialized into normalized log entries at `Handle` time and are never written back into persistent operation metadata.

Persistent operation metadata is attached through `ops.WithAttrs(ctx, attrs ...slog.Attr) error`.

Wide collection is optional and is attached explicitly at root creation through a wide-specific `ops.Option` produced by the wide handler.

Emission strategy is root-owned and inherited by descendants. The public strategy interface exposes:

- `Collect`
- `Flush`

The built-in strategies are passthrough and aggregate.

`ops.End(ctx)` restores the parent context and flushes the root when the ending operation is the root.

If aggregate collection exceeds configured limits, the root emits one synthetic overflow diagnostic, discards buffered aggregate state, and continues in passthrough mode for the rest of the root lifetime.

## Consequences

Aggregation boundaries are explicit and align with operation lifecycle.

Concurrent goroutines can write into the same root-owned tree safely.

Mixed strategy behavior inside one tree is avoided because the root owns the effective strategy.

The collector remains optional rather than becoming an inherent part of every operation.

Overflow behavior is predictable and avoids emitting misleading partial summaries.
