/*
Package ops provides a simple API for tracking operations across the call stack.

These operations will be translated into log groups, spans, etc. depending on what handlers
are configured.
*/
package ops

import "context"

type contextKey string

const (
	parentContextKey contextKey = "go-wide.ops.parent_context"
	operationKey     contextKey = "go-wide.ops.operation"
	errorKey         contextKey = "go-wide.ops.error"
)

type Subscriber interface {
	NotifyStartOperation(ctx context.Context, op *Operation) context.Context
	NotifyEndOperation(ctx context.Context, op *Operation) context.Context
}

type Operation struct {
	Op          string
	Description string
	parent      *Operation
	parentOps   []string
	subscribers []Subscriber
}

func (o *Operation) Subscribe(s Subscriber) {
	o.subscribers = append(o.subscribers, s)
}

type OperationAction string

const (
	StartOperation OperationAction = "start"
	EndOperation   OperationAction = "end"
)

func (o *Operation) NotifyAllSubscribers(ctx context.Context, action OperationAction) context.Context {
	var notify func(s Subscriber) context.Context
	switch action {
	case StartOperation:
		notify = func(s Subscriber) context.Context {
			return s.NotifyStartOperation(ctx, o)
		}
	case EndOperation:
		notify = func(s Subscriber) context.Context {
			return s.NotifyEndOperation(ctx, o)
		}
	default:
		return ctx
	}
	for _, s := range o.subscribers {
		ctx = notify(s)
	}
	return ctx
}

func (o *Operation) Parent() *Operation {
	return o.parent
}

func (o *Operation) Ops() []string {
	ops := append(o.parentOps, o.Op)
	return ops
}
