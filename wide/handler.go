/*
Package wide provides a handler for the slog.Handler interface that translates
slog groups into log messages.
*/
package wide

import (
	"context"
	"log/slog"

	"github.com/druejaramillo/go-wide/ops"
)

type Handler struct {
	handler slog.Handler
}

func NewHandler(h slog.Handler) *Handler {
	return &Handler{handler: h}
}

func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	return h.handler.Handle(ctx, r)
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h.handler.WithAttrs(attrs)
}

func (h *Handler) WithGroup(name string) slog.Handler {
	return h.handler.WithGroup(name)
}

func (h *Handler) RootOption() ops.Option {
	return func(rc *ops.RootConfig) {
	}
}
