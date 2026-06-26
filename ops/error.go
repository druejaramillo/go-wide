package ops

import (
	"context"
)

func Error(ctx context.Context, err error) (context.Context, error) {
	operation := GetOperationFromContext(ctx)
	if operation == nil {
		return ctx, ErrNoActiveOperation
	}
	if err == nil {
		return ctx, nil
	}
	for _, o := range operation.rootConfig.ErrorObservers {
		o.OnError(ctx, operation, err)
	}
	currentErr := GetErrorFromContext(ctx)
	if currentErr != nil {
		return ctx, nil
	}
	ctx = context.WithValue(ctx, errorKey, err)
	return ctx, nil
}
