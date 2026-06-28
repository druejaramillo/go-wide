package sentry

import (
	"context"

	sentrysdk "github.com/getsentry/sentry-go"
	"github.com/druejaramillo/go-wide/ops"
)

var (
	_ ops.LifecycleObserver = (*Observer)(nil)
	_ ops.ErrorObserver     = (*Observer)(nil)
)

type Observer struct {
	hub *sentrysdk.Hub
}

func NewObserver(hub *sentrysdk.Hub) *Observer {
	// TODO(phase-4 done): Return an observer whose RootOption registers the same
	// instance as both a lifecycle observer and an error observer.
	return &Observer{hub: hub}
}

func (o *Observer) RootOption() ops.Option {
	return func(rc *ops.RootConfig) {
		rc.LifecycleObservers = append(rc.LifecycleObservers, o)
		rc.ErrorObservers = append(rc.ErrorObservers, o)
	}
}

func (o *Observer) OnStart(ctx context.Context, op *ops.Operation) context.Context {
	// TODO: Root start derives a context with a Sentry hub and root transaction.
	// TODO: Child start derives a context with a child span under the active trace.
	if o == nil || o.hub == nil || op == nil {
		return ctx
	}

	if len(op.Ops()) != 1 {
		if sentrysdk.SpanFromContext(ctx) == nil {
			return ctx
		}

		childSpan := sentrysdk.StartSpan(ctx, op.Op)
		return childSpan.Context()
	}

	hub := o.hub.Clone()
	ctx = sentrysdk.SetHubOnContext(ctx, hub)
	transaction := sentrysdk.StartTransaction(ctx, op.Op)
	return sentrysdk.SetHubOnContext(transaction.Context(), hub)
}

func (o *Observer) OnEnd(ctx context.Context, op *ops.Operation) context.Context {
	// TODO: Finish child spans on child end without breaking parent operation context.
	// TODO: Finish the root transaction on root end.
	if op == nil {
		return ctx
	}

	span := sentrysdk.SpanFromContext(ctx)
	if span != nil {
		span.Finish()
	}

	return ctx
}

func (o *Observer) OnError(ctx context.Context, op *ops.Operation, err error) {
	// TODO: Capture each non-nil ops.Error call to Sentry and mark the active span failed.
	// TODO: Preserve repeated capture behavior across repeated ops.Error calls.
	_ = ctx
	_ = op
	_ = err
	return
}
