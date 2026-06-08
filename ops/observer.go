package ops

import "context"

type LifecycleObserver interface {
	OnStart(ctx context.Context, op *Operation) context.Context
	OnEnd(ctx context.Context, op *Operation) context.Context
}

func WithLifecycleObserver(o LifecycleObserver) Option {
	return func(rc *rootConfig) {
		rc.lifecycleObservers = append(rc.lifecycleObservers, o)
	}
}
