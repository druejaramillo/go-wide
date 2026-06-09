package ops

import "context"

type LifecycleObserver interface {
	OnStart(ctx context.Context, op *Operation) context.Context
	OnEnd(ctx context.Context, op *Operation) context.Context
}

func WithLifecycleObserver(o LifecycleObserver) Option {
	return func(rc *RootConfig) {
		rc.LifecycleObservers = append(rc.LifecycleObservers, o)
	}
}

type ErrorObserver interface {
	OnError(ctx context.Context, op *Operation, err error)
}

func WithErrorObserver(o ErrorObserver) Option {
	return func(rc *RootConfig) {
		rc.ErrorObservers = append(rc.ErrorObservers, o)
	}
}
