package ops

import (
	"context"
	"errors"
)

func StartRoot(ctx context.Context, op string) (context.Context, error) {
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
	if parent != nil {
		parentOps = parent.Ops()
	}
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
	ctx, ok := ctx.Value(parentContextKey).(context.Context)
	if !ok {
		return ctx, errors.New("no parent context found")
	}
	return ctx, nil
}
