/*
Package ops provides a simple API for tracking operations across the call stack.

These operations will be translated into log groups, spans, etc. depending on what handlers
are configured.
*/
package ops

import (
	"log/slog"
	"sync/atomic"
)

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
	rootConfig  *RootConfig
	attrs       []slog.Attr
	ended       atomic.Bool
}

func (o *Operation) Ops() []string {
	ops := append(o.parentOps, o.Op)
	return ops
}

func (o *Operation) Attrs() []slog.Attr {
	return o.attrs
}

func (o *Operation) IsEnded() bool {
	return o != nil && o.ended.Load()
}

func (o *Operation) markEnded() {
	if o != nil {
		o.ended.Store(true)
	}
}

type RootConfig struct {
	LifecycleObservers []LifecycleObserver
	ErrorObservers     []ErrorObserver
}

type Option func(*RootConfig)
