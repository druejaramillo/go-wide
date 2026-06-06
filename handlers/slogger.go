/*
Package handlers provides a set of handlers for logging and tracing.
*/
package handlers

import (
	"context"
	"log/slog"

	"github.com/druejaramillo/go-wide/ops"
)

type Slogger struct {
	handler slog.Handler
	attrs   []slog.Attr
	groups  []string
}

func NewSlogger(handler slog.Handler) *Slogger {
	return &Slogger{
		handler: handler,
	}
}

func (h *Slogger) Enabled(_ context.Context, level slog.Level) bool {
	return h.handler.Enabled(context.Background(), level)
}

func (h *Slogger) Handle(ctx context.Context, rec slog.Record) error {
	op := ops.GetOperationFromContext(ctx)
	groups := op.Ops()
	groups = append(groups, h.groups...)
	handler := h.handler
	for _, group := range groups {
		handler = handler.WithGroup(group)
	}
	rec.AddAttrs(h.attrs...)
	return h.handler.Handle(ctx, rec)
}

func (h *Slogger) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Slogger{
		handler: h.handler,
		attrs:   append(h.attrs, attrs...),
		groups:  h.groups,
	}
}

func (h *Slogger) WithGroup(name string) slog.Handler {
	return &Slogger{
		handler: h.handler,
		attrs:   h.attrs,
		groups:  append(h.groups, name),
	}
}
