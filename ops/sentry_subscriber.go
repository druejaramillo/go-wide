package ops

import (
	"context"

	"github.com/getsentry/sentry-go"
)

type SentrySubscriber struct{}

func NewSentrySubscriber() *SentrySubscriber {
	return &SentrySubscriber{}
}

func (s *SentrySubscriber) NotifyStartOperation(ctx context.Context, op *Operation) context.Context {
	hub := sentry.GetHubFromContext(ctx)
	return sentry.SetHubOnContext(ctx, hub.Clone())
}

func (s *SentrySubscriber) NotifyEndOperation(ctx context.Context, op *Operation) context.Context {
	return ctx
}
