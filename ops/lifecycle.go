package ops

import (
	"context"
	"errors"
)

func StartRoot(ctx context.Context, op string) (context.Context, error) {
	if GetOperationFromContext(ctx) != nil {
		return ctx, errors.New("cannot create a nested root operation")
	}
	ctx = context.WithValue(ctx, parentContextKey, ctx)
	return context.WithValue(ctx, operationKey, &Operation{
		Op:          op,
		Description: "",
		parent:      nil,
		parentOps:   []string{},
		subscribers: []Subscriber{},
	}), nil
}

func Start(ctx context.Context, op string) (context.Context, error) {
	parent := GetOperationFromContext(ctx)
	var parentOps []string
	if parent == nil {
		return ctx, errors.New("expected a parent operation")
	}
	parentOps = parent.Ops()
	operation := &Operation{
		Op:          op,
		Description: "",
		parent:      parent,
		parentOps:   parentOps,
		subscribers: []Subscriber{},
	}
	ctx = context.WithValue(ctx, parentContextKey, ctx)
	return context.WithValue(ctx, operationKey, operation), nil
}

func End(ctx context.Context) (context.Context, error) {
	if GetOperationFromContext(ctx) == nil {
		return ctx, errors.New("expected an active operation")
	}
	ctx, ok := ctx.Value(parentContextKey).(context.Context)
	if !ok {
		return ctx, errors.New("no parent context found")
	}
	return ctx, nil
}
