package ops

import "context"

func Error(ctx context.Context, err error) (context.Context, error) {
	currentErr := GetErrorFromContext(ctx)
	if currentErr != nil {
		return ctx, nil
	}
	return context.WithValue(ctx, errorKey, err), nil
}
