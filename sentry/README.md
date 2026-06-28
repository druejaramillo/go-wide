# Go Wide Sentry

`github.com/druejaramillo/go-wide/sentry` is the Sentry integration module for `go-wide`.

It provides two small adapters:

- `sentry.Observer` implements `ops.LifecycleObserver` and `ops.ErrorObserver`
- `sentry.Reporter` implements `err.ErrorReporter`

Use `Observer` when you want `ops.StartRoot` / `ops.Start` / `ops.End` to create and finish Sentry transactions and spans, and when you want `ops.Error(...)` to capture exceptions in Sentry.

Use `Reporter` when you want a plain `err.ErrorReporter` that captures exceptions through Sentry.

## Installation

```bash
go get github.com/druejaramillo/go-wide/sentry
```

## Tracing And Operation Error Capture

Create a Sentry client and hub, then attach the observer at root creation time.

```go
package main

import (
	"context"
	"time"

	gowidesentry "github.com/druejaramillo/go-wide/sentry"
	"github.com/druejaramillo/go-wide/ops"
	sentrysdk "github.com/getsentry/sentry-go"
)

func main() {
	if err := sentrysdk.Init(sentrysdk.ClientOptions{
		Dsn:              "https://public@example.com/1",
		EnableTracing:    true,
		TracesSampleRate: 1.0,
	}); err != nil {
		panic(err)
	}
	defer sentrysdk.Flush(2 * time.Second)

	observer := gowidesentry.NewObserver(sentrysdk.CurrentHub().Clone())

	rootCtx, err := ops.StartRoot(context.Background(), "checkout", observer.RootOption())
	if err != nil {
		panic(err)
	}

	chargeCtx, err := ops.Start(rootCtx, "charge")
	if err != nil {
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

Behavior:

- the root-operation starts a Sentry transaction
- child operations start child spans
- `ops.End(...)` finishes spans and the root transaction
- `ops.Error(ctx, err)` marks the active span with `internal_error` and captures the exception
- repeated non-nil `ops.Error(...)` calls produce repeated Sentry captures, matching `ops.ErrorObserver` semantics

## Generic Error Reporting

If you only need the `err.ErrorReporter` seam, use `Reporter`.

```go
package main

import (
	"context"
	"errors"
	"time"

	goerr "github.com/druejaramillo/go-wide/err"
	gowidesentry "github.com/druejaramillo/go-wide/sentry"
	sentrysdk "github.com/getsentry/sentry-go"
)

func main() {
	if err := sentrysdk.Init(sentrysdk.ClientOptions{Dsn: "https://public@example.com/1"}); err != nil {
		panic(err)
	}
	defer sentrysdk.Flush(2 * time.Second)

	var reporter goerr.ErrorReporter = gowidesentry.NewReporter(sentrysdk.CurrentHub().Clone())
	reporter.Capture(context.Background(), errors.New("checkout failed"))
}
```

`Reporter.Capture(ctx, err)` uses the Sentry hub stored on `ctx` when present and falls back to the reporter's base hub otherwise.

## Notes

- This module depends on `github.com/getsentry/sentry-go`, not the core `go-wide` module.
- The core `go-wide` packages remain vendor-neutral; this module is the Sentry-specific adapter layer.
