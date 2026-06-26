package ops

import "context"

func GetOperationFromContext(ctx context.Context) *Operation {
	op, ok := ctx.Value(operationKey).(*Operation)
	if !ok {
		return nil
	}
	return op
}

func getActiveOperationFromContext(ctx context.Context) *Operation {
	op := GetOperationFromContext(ctx)
	if op == nil || op.IsEnded() {
		return nil
	}
	return op
}

func GetErrorFromContext(ctx context.Context) error {
	err, ok := ctx.Value(errorKey).(error)
	if !ok {
		return nil
	}
	return err
}

const endErrorKey contextKey = "go-wide.ops.end_error"

func WithEndError(ctx context.Context, err error) context.Context {
	if err == nil {
		return ctx
	}

	if existing, ok := ctx.Value(endErrorKey).(error); ok && existing != nil {
		return ctx
	}

	return context.WithValue(ctx, endErrorKey, err)
}

func getEndErrorFromContext(ctx context.Context) error {
	err, ok := ctx.Value(endErrorKey).(error)
	if !ok {
		return nil
	}
	return err
}
