package ops

import "context"

func GetOperationFromContext(ctx context.Context) *Operation {
	op, ok := ctx.Value(operationKey).(*Operation)
	if !ok {
		return nil
	}
	return op
}

func GetErrorFromContext(ctx context.Context) error {
	err, ok := ctx.Value(ErrorKey).(error)
	if !ok {
		return nil
	}
	return err
}

func Start(ctx context.Context, op string, description string) context.Context {
	parent := GetOperationFromContext(ctx)
	subscribers := []Subscriber{}
	var parentOps []string
	if parent != nil {
		subscribers = append(subscribers, parent.subscribers...)
		parentOps = parent.Ops()
	} else {
		parentOps = []string{}
	}
	operation := &Operation{
		Op:          op,
		Description: description,
		parent:      parent,
		parentOps:   parentOps,
		subscribers: subscribers,
	}
	operation.NotifyAllSubscribers(ctx, StartOperation)
	return context.WithValue(ctx, operationKey, operation)
}

func End(ctx context.Context) {
	operation := GetOperationFromContext(ctx)
	if operation == nil {
		return
	}
	operation.NotifyAllSubscribers(ctx, EndOperation)
}

func Error(ctx context.Context, err error) context.Context {
	if err != nil {
		return context.WithValue(ctx, ErrorKey, err)
	}
	return ctx
}
