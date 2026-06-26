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
