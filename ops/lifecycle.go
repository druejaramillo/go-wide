package ops

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrNoActiveOperation   = errors.New("no active operation")
	ErrNestedRootOperation = errors.New("cannot create a nested root operation")
	ErrRootEndEmission     = errors.New("root end emission failed")
)

func StartRoot(ctx context.Context, op string, opts ...Option) (context.Context, error) {
	if getActiveOperationFromContext(ctx) != nil {
		return ctx, ErrNestedRootOperation
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
	parent := getActiveOperationFromContext(ctx)
	if parent == nil {
		return ctx, ErrNoActiveOperation
	}

	rc := parent.rootConfig
	operation := &Operation{
		Op:          op,
		Description: "",
		parent:      parent,
		parentOps:   parent.Ops(),
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
	operation := getActiveOperationFromContext(ctx)
	if operation == nil {
		return ctx, ErrNoActiveOperation
	}

	for _, o := range operation.rootConfig.LifecycleObservers {
		ctx = o.OnEnd(ctx, operation)
	}

	operation.markEnded()

	parentCtx, ok := ctx.Value(parentContextKey).(context.Context)
	if !ok {
		return ctx, errors.New("no parent context found")
	}

	if endErr := getEndErrorFromContext(ctx); endErr != nil {
		return parentCtx, fmt.Errorf("%w: %w", ErrRootEndEmission, endErr)
	}

	return parentCtx, nil
}
