# Go Wide

`go-wide` is a Go library for modeling operation lifecycles and, when you want it, emitting a single structured `wide-event` that summarizes an entire operation tree.

It is split on purpose:

- `ops` handles operation lifecycle, operation-metadata, context propagation, and observers.
- `wide` wraps `log/slog` and turns logs produced inside an operation tree into either ordinary passthrough records or one final aggregated `wide-event`.

The result is a small core that is useful even without log aggregation, while still supporting rich structured summaries when you attach a `wide` handler.

## Packages

| Package | Purpose |
| --- | --- |
| `ops` | Start and end operations, attach operation-metadata, record explicit operation errors, and register root-owned observers. |
| `wide` | `slog.Handler` wrapper that can pass logs through immediately, aggregate them into a root-owned tree, or delegate to a custom strategy. |
| `handlers` | Thin operation-aware `slog.Handler` helpers. |
| `err` | Tiny integration surface for error capture via `ErrorReporter`. |

## Installation

```bash
go get github.com/druejaramillo/go-wide
```

## Core Idea

An `operation` is a named lifecycle boundary. Operations are nested explicitly with `ops.StartRoot`, `ops.Start`, and `ops.End`.

- Root operations own the observer set, emission policy, and wide collection state for the whole tree.
- Child operations inherit that root-owned behavior.
- Operation-metadata is attached with `ops.WithAttrs(...)`.
- Logger-local `With(...)` and `WithGroup(...)` state stays local to the logger and is only materialized onto log records.

If you attach `wide`, the root can emit one final `wide-event` when it ends. If you do not, logging stays ordinary `slog` passthrough.

## Quick Start

### `ops` Only

Use `ops` when you want explicit operation boundaries, metadata, and observer propagation without changing your logging setup.

```go
package main

import (
	"context"
	"log/slog"

	"github.com/druejaramillo/go-wide/ops"
)

func main() {
	rootCtx, err := ops.StartRoot(context.Background(), "checkout")
	if err != nil {
		panic(err)
	}

	if err := ops.WithAttrs(rootCtx,
		slog.String("request_id", "req-123"),
		slog.String("tenant", "acme"),
	); err != nil {
		panic(err)
	}

	chargeCtx, err := ops.Start(rootCtx, "charge")
	if err != nil {
		panic(err)
	}

	if err := ops.WithAttrs(chargeCtx, slog.String("provider", "stripe")); err != nil {
		panic(err)
	}

	chargeCtx, err = ops.Error(chargeCtx, context.DeadlineExceeded)
	if err != nil {
		panic(err)
	}

	rootCtx, err = ops.End(chargeCtx)
	if err != nil {
		panic(err)
	}

	if _, err := ops.End(rootCtx); err != nil {
		panic(err)
	}
}
```

Useful `ops` helpers:

- `ops.Run(ctx, op, fn)` wraps `Start`, callback execution, `Error` on callback failure, and deferred `End` for a child operation.
- `ops.GetOperationFromContext(ctx)` returns the current operation.
- `ops.GetErrorFromContext(ctx)` returns the first explicit operation error stored on that context.

### Aggregate a `wide-event`

Attach a `wide.Handler` at root creation time to collect logs for the whole operation tree and emit one final summary when the root ends.

```go
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/druejaramillo/go-wide/ops"
	"github.com/druejaramillo/go-wide/wide"
)

func main() {
	handler := wide.NewHandler(
		slog.NewJSONHandler(os.Stdout, nil),
		wide.WithAggregate(),
	)
	logger := slog.New(handler)

	rootCtx, err := ops.StartRoot(context.Background(), "checkout", handler.RootOption())
	if err != nil {
		panic(err)
	}

	if err := ops.WithAttrs(rootCtx, slog.String("request_id", "req-123")); err != nil {
		panic(err)
	}

	chargeCtx, err := ops.Start(rootCtx, "charge")
	if err != nil {
		panic(err)
	}

	if err := ops.WithAttrs(chargeCtx, slog.String("provider", "stripe")); err != nil {
		panic(err)
	}

	logger.WithGroup("payment").InfoContext(chargeCtx, "retrying charge", slog.Int("attempt", 1))

	chargeCtx, err = ops.Error(chargeCtx, context.DeadlineExceeded)
	if err != nil {
		panic(err)
	}

	rootCtx, err = ops.End(chargeCtx)
	if err != nil {
		panic(err)
	}

	if _, err := ops.End(rootCtx); err != nil {
		panic(err)
	}
}
```

The final JSON record will look roughly like this:

```json
{
  "msg": "checkout",
  "request_id": "req-123",
  "version": 1,
  "status": "error",
  "start": "2026-06-27T12:00:00Z",
  "end": "2026-06-27T12:00:01Z",
  "charge": {
    "provider": "stripe",
    "status": "error",
    "error": "context deadline exceeded",
    "logs": {
      "retrying charge": {
        "message": "retrying charge",
        "level": "INFO",
        "count": 1,
        "payment": {
          "attempt": 1
        }
      }
    }
  }
}
```

Field ordering may vary, but the shape is stable:

- Root operation-metadata stays at the top level.
- Child operations become direct groups.
- Aggregated logs live under `logs`.
- Diverging values are summarized under `variants`.
- The aggregate is structural, not chronological.

## Handler Modes

| Mode | How to enable | Behavior |
| --- | --- | --- |
| Passthrough | Default `wide.NewHandler(...)` | Emits each `slog.Record` immediately. |
| Attached passthrough | Pass `handler.RootOption()` to `ops.StartRoot(...)` without aggregate options | Keeps normal passthrough behavior while attaching root-owned state. |
| Aggregate | `wide.WithAggregate()` and `handler.RootOption()` | Buffers logs under the active root and emits one final `wide-event` on root `End`. |
| Custom | `wide.WithStrategy(strategy)` and `handler.RootOption()` | Feeds public `LogEntry` values into your strategy and flushes an `OperationSnapshot` on root `End`. |

Important: building an aggregate handler is not enough by itself. You must attach `handler.RootOption()` when the root operation is created, or logs will remain ordinary passthrough records.

## Lifecycle Rules

- Call `ops.StartRoot(...)` before `ops.Start(...)`.
- Root operations cannot be nested.
- Always keep the returned context from `StartRoot`, `Start`, `Error`, and `End`.
- `ops.End(childCtx)` returns the parent context.
- `ops.End(rootCtx)` restores a context with no active operation.
- `ops.End(rootCtx)` can return `ops.ErrRootEndEmission` if final root emission fails.
- `ops.WithAttrs(...)` adds operation-metadata only to the active operation.
- `ops.Error(...)` stores only the first explicit operation error in context.
- Root-owned error observers still see every non-nil `ops.Error(...)` call.

## Operation-Metadata vs Logger Attrs

`go-wide` distinguishes between persistent operation-metadata and logger-local attrs:

- Use `ops.WithAttrs(ctx, ...)` for metadata that belongs to the operation node itself.
- Use `logger.With(...)` or `logger.WithGroup(...)` for normal `slog` scoping.

In aggregate mode, logger-local attrs are captured on the specific log entry that emitted them. They are not written back into the operation node.

## Aggregate Semantics

The built-in aggregate strategy emits a single `slog.Record` when the root operation ends.

- The final message is the root operation name.
- Root attrs include `version`, `status`, `start`, and `end`.
- Child operations with the same name are merged structurally.
- Repeated log messages are bucketed by message and level.
- Shared values stay in place; differing values move under `variants`.
- Final emission bypasses ordinary downstream level filtering.

If you need a chronological stream, use passthrough instead of aggregate mode.

## Overflow Behavior

Use `wide.WithAggregateLimit(n)` to cap how many log records a root-owned aggregate collector will buffer.

When the limit is exceeded:

- One warning record is emitted with message `wide aggregate overflow`.
- The warning includes `reason=limit_exceeded` and the configured `limit`.
- Buffered aggregate state is discarded.
- The rest of that root lifetime falls back to passthrough logging.
- No final aggregate record is emitted for that root.

Invalid limits fail early. A negative aggregate limit causes `ops.StartRoot(..., handler.RootOption())` to return `ops.ErrInvalidOptionUsage`.

## Observers And Integrations

`ops` supports root-owned observers that propagate to every descendant operation:

- `ops.WithLifecycleObserver(...)` for start and end hooks.
- `ops.WithErrorObserver(...)` for explicit operation errors.

The `err` package exposes:

```go
type ErrorReporter interface {
	Capture(ctx context.Context, err error)
}
```

That keeps vendor-specific reporting integrations outside the core module, which matches the design in the ADRs.

This repository also includes a nested Sentry integration module at `./sentry`:

- module path: `github.com/druejaramillo/go-wide/sentry`
- provides `sentry.Observer` for `ops` tracing and explicit operation error capture
- provides `sentry.Reporter` for the `err.ErrorReporter` seam

For installation, wiring, and usage examples, see [`sentry/README.md`](./sentry/README.md).

## Custom Strategies

Custom wide strategies only need the public `wide` API:

```go
type Strategy interface {
	Collect(entry LogEntry)
	Flush(snapshot OperationSnapshot) slog.Record
}
```

Useful helpers:

- `wide.NormalizeLogs(...)` converts raw `[]LogEntry` into the same normalized log bucket shape used by the built-in aggregate strategy.
- `wide.NormalizeSnapshot(...)` converts a raw `OperationSnapshot` into the same merged structural shape used by the built-in aggregate strategy.

`Flush` receives a raw snapshot. Same-named children are still separate at that point; call `wide.NormalizeSnapshot(...)` if you want the built-in merged view.

## Development

Run the test suite with:

```bash
go test ./...
```

The repository also keeps its design history in:

- `CONTEXT.md`
- `docs/adr/0001-separate-ops-and-wide.md`
- `docs/adr/0002-root-owned-wide-collection.md`
- `docs/adr/0003-wide-event-schema.md`
