package sentry

import (
	"context"

	sentrysdk "github.com/getsentry/sentry-go"
	goerr "github.com/druejaramillo/go-wide/err"
)

var _ goerr.ErrorReporter = (*Reporter)(nil)

type Reporter struct {
	hub *sentrysdk.Hub
}

func NewReporter(hub *sentrysdk.Hub) *Reporter {
	return &Reporter{hub: hub}
}

func (r *Reporter) Capture(ctx context.Context, err error) {
	_ = ctx
	_ = err
	return
}
