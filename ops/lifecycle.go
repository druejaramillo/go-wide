package ops

import (
	"context"
	"errors"
)

func StartRoot(ctx context.Context, op string, opts ...Option) (context.Context, error) {
	if GetOperationFromContext(ctx) != nil {
		return ctx, errors.New("cannot create a nested root operation")
	}
	ctx = context.WithValue(ctx, parentContextKey, ctx)
	rc := &RootConfig{}
	for _, opt := range opts {
		opt(rc)
	}
	operation := &Operation{
		Op:          op,
		Description: "",
		parent:      nil,
		parentOps:   []string{},
		rootConfig:  rc,
	}
	ctx = context.WithValue(ctx, operationKey, operation)
	for _, o := range rc.LifecycleObservers {
		ctx = o.OnStart(ctx, operation)
	}
	return ctx, nil
}

func Start(ctx context.Context, op string) (context.Context, error) {
	parent := GetOperationFromContext(ctx)
	var parentOps []string
	if parent == nil {
		return ctx, errors.New("expected a parent operation")
	}
	parentOps = parent.Ops()
	rc := parent.rootConfig
	operation := &Operation{
		Op:          op,
		Description: "",
		parent:      parent,
		parentOps:   parentOps,
		rootConfig:  rc,
	}
	ctx = context.WithValue(ctx, parentContextKey, ctx)
	ctx = context.WithValue(ctx, operationKey, operation)
	for _, o := range rc.LifecycleObservers {
		ctx = o.OnStart(ctx, operation)
	}
	return ctx, nil
}

func End(ctx context.Context) (context.Context, error) {
	operation := GetOperationFromContext(ctx)
	if operation == nil {
		return ctx, errors.New("expected an active operation")
	}
	for _, o := range operation.rootConfig.LifecycleObservers {
		_ = o.OnEnd(ctx, operation)
	}
	ctx, ok := ctx.Value(parentContextKey).(context.Context)
	if !ok {
		return ctx, errors.New("no parent context found")
	}
	return ctx, nil
}
