package ops

import (
	"context"
	"errors"
	"log/slog"
)

func WithAttrs(ctx context.Context, attrs ...slog.Attr) error {
	op := GetOperationFromContext(ctx)
	if op == nil {
		return errors.New("no active operation")
	}
	op.attrs = append(op.attrs, attrs...)
	return nil
}
