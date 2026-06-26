package ops

import (
	"context"
	"log/slog"
)

func WithAttrs(ctx context.Context, attrs ...slog.Attr) error {
	op := getActiveOperationFromContext(ctx)
	if op == nil {
		return ErrNoActiveOperation
	}
	op.attrs = append(op.attrs, attrs...)
	return nil
}
