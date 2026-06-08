package ops

import (
	"context"
	"errors"
)

func Error(ctx context.Context, err error) (context.Context, error) {
	if GetOperationFromContext(ctx) == nil {
		return ctx, errors.New("expected an active operation")
	}
	currentErr := GetErrorFromContext(ctx)
	if currentErr != nil {
		return ctx, nil
	}
	return context.WithValue(ctx, errorKey, err), nil
}
