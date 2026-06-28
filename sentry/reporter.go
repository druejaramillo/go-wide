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
	// TODO: Capture non-nil errors with the hub from ctx when present.
	// TODO: Fall back to the reporter base hub when ctx has no Sentry hub.
	// TODO: Treat nil errors as a no-op.
	if err == nil {
		return
	}

	hub := sentrysdk.GetHubFromContext(ctx)
	if hub == nil {
		hub = r.hub
	}
	if hub == nil {
		return
	}

	hub.CaptureException(err)
}
