package err

import (
	"context"

	"github.com/getsentry/sentry-go"
)

type SentryReporter struct{}

func NewSentryReporter() *SentryReporter {
	return &SentryReporter{}
}

func (r *SentryReporter) Capture(ctx context.Context, err error) {
	hub := sentry.GetHubFromContext(ctx)
	hub.CaptureException(err)
}
