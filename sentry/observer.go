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
	return &Observer{hub: hub}
}

func (o *Observer) RootOption() ops.Option {
	return func(rc *ops.RootConfig) {
		rc.LifecycleObservers = append(rc.LifecycleObservers, o)
		rc.ErrorObservers = append(rc.ErrorObservers, o)
	}
}

func (o *Observer) OnStart(ctx context.Context, op *ops.Operation) context.Context {
	return ctx
}

func (o *Observer) OnEnd(ctx context.Context, op *ops.Operation) context.Context {
	return ctx
}

func (o *Observer) OnError(ctx context.Context, op *ops.Operation, err error) {
	_ = ctx
	_ = op
	_ = err
	return
}
