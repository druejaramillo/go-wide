package sentry

import (
	"context"
	"errors"
	"testing"

	sentrysdk "github.com/getsentry/sentry-go"
)

func TestReporterCaptureUsesHubFromContext(t *testing.T) {
	baseHub, baseTransport := newTestHubWithTransport(t)
	ctxHub, ctxTransport := newTestHubWithTransport(t)
	reporter := NewReporter(baseHub)

	reportErr := errors.New("checkout failed")
	ctx := sentrysdk.SetHubOnContext(context.Background(), ctxHub)

	reporter.Capture(ctx, reportErr)

	if got := len(baseTransport.Events()); got != 0 {
		t.Fatalf("base transport captured %d events, want 0", got)
	}

	events := ctxTransport.Events()
	if len(events) != 1 {
		t.Fatalf("context transport captured %d events, want 1", len(events))
	}

	if got := events[0].Exception[0].Value; got != reportErr.Error() {
		t.Fatalf("captured exception value = %q, want %q", got, reportErr.Error())
	}
}

func TestReporterCaptureFallsBackToBaseHub(t *testing.T) {
	baseHub, baseTransport := newTestHubWithTransport(t)
	reporter := NewReporter(baseHub)

	reportErr := errors.New("checkout failed")
	reporter.Capture(context.Background(), reportErr)

	events := baseTransport.Events()
	if len(events) != 1 {
		t.Fatalf("base transport captured %d events, want 1", len(events))
	}

	if got := events[0].Exception[0].Value; got != reportErr.Error() {
		t.Fatalf("captured exception value = %q, want %q", got, reportErr.Error())
	}
}

func TestReporterCaptureNilErrorIsNoOp(t *testing.T) {
	baseHub, baseTransport := newTestHubWithTransport(t)
	reporter := NewReporter(baseHub)

	reporter.Capture(context.Background(), nil)

	if got := len(baseTransport.Events()); got != 0 {
		t.Fatalf("base transport captured %d events, want 0", got)
	}
}
