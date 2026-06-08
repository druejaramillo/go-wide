/*
Package ops provides a simple API for tracking operations across the call stack.

These operations will be translated into log groups, spans, etc. depending on what handlers
are configured.
*/
package ops

type contextKey string

const (
	parentContextKey contextKey = "go-wide.ops.parent_context"
	operationKey     contextKey = "go-wide.ops.operation"
	errorKey         contextKey = "go-wide.ops.error"
)

type Operation struct {
	Op          string
	Description string
	parent      *Operation
	parentOps   []string
	rootConfig  *rootConfig
}

func (o *Operation) Ops() []string {
	ops := append(o.parentOps, o.Op)
	return ops
}

type rootConfig struct {
	lifecycleObservers []LifecycleObserver
}

type Option func(*rootConfig)
