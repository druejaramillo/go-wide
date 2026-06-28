package sentry

import (
	"context"
	"errors"
	"testing"

	wideops "github.com/druejaramillo/go-wide/ops"
	sentrysdk "github.com/getsentry/sentry-go"
)

func TestObserverStartRootDerivesSentryContext(t *testing.T) {
	observer := NewObserver(newTestHub(t))

	rootCtx, err := wideops.StartRoot(context.Background(), "checkout", observer.RootOption())
	if err != nil {
		t.Fatalf("StartRoot() error = %v", err)
	}

	if got := sentrysdk.GetHubFromContext(rootCtx); got == nil {
		t.Fatal("GetHubFromContext(rootCtx) = nil, want non-nil hub")
	}

	transaction := sentrysdk.TransactionFromContext(rootCtx)
	if transaction == nil {
		t.Fatal("TransactionFromContext(rootCtx) = nil, want root transaction")
	}

	if transaction.Name != "checkout" {
		t.Fatalf("transaction.Name = %q, want %q", transaction.Name, "checkout")
	}
}

func TestObserverStartChildDerivesChildSpan(t *testing.T) {
	observer := NewObserver(newTestHub(t))

	rootCtx, err := wideops.StartRoot(context.Background(), "checkout", observer.RootOption())
	if err != nil {
		t.Fatalf("StartRoot() error = %v", err)
	}

	rootTransaction := sentrysdk.TransactionFromContext(rootCtx)
	if rootTransaction == nil {
		t.Fatal("TransactionFromContext(rootCtx) = nil, want root transaction")
	}

	childCtx, err := wideops.Start(rootCtx, "charge")
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	childSpan := sentrysdk.SpanFromContext(childCtx)
	if childSpan == nil {
		t.Fatal("SpanFromContext(childCtx) = nil, want child span")
	}

	if childSpan == rootTransaction {
		t.Fatal("SpanFromContext(childCtx) = root transaction, want child span")
	}

	if childSpan.Op != "charge" {
		t.Fatalf("childSpan.Op = %q, want %q", childSpan.Op, "charge")
	}

	if childSpan.ParentSpanID != rootTransaction.SpanID {
		t.Fatalf("childSpan.ParentSpanID = %q, want %q", childSpan.ParentSpanID, rootTransaction.SpanID)
	}

	if childSpan.TraceID != rootTransaction.TraceID {
		t.Fatalf("childSpan.TraceID = %q, want %q", childSpan.TraceID, rootTransaction.TraceID)
	}
}

func TestObserverEndChildFinishesChildSpanAndRestoresRootTransaction(t *testing.T) {
	observer := NewObserver(newTestHub(t))

	rootCtx, err := wideops.StartRoot(context.Background(), "checkout", observer.RootOption())
	if err != nil {
		t.Fatalf("StartRoot() error = %v", err)
	}

	rootTransaction := sentrysdk.TransactionFromContext(rootCtx)
	if rootTransaction == nil {
		t.Fatal("TransactionFromContext(rootCtx) = nil, want root transaction")
	}

	childCtx, err := wideops.Start(rootCtx, "charge")
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	childSpan := sentrysdk.SpanFromContext(childCtx)
	if childSpan == nil {
		t.Fatal("SpanFromContext(childCtx) = nil, want child span")
	}

	parentCtx, err := wideops.End(childCtx)
	if err != nil {
		t.Fatalf("End(childCtx) error = %v", err)
	}

	if childSpan.EndTime.IsZero() {
		t.Fatal("childSpan.EndTime is zero, want finished child span")
	}

	restoredTransaction := sentrysdk.TransactionFromContext(parentCtx)
	if restoredTransaction == nil {
		t.Fatal("TransactionFromContext(parentCtx) = nil, want root transaction")
	}

	if restoredTransaction != rootTransaction {
		t.Fatal("TransactionFromContext(parentCtx) did not restore the root transaction")
	}
}

func TestObserverEndRootFinishesRootTransaction(t *testing.T) {
	observer := NewObserver(newTestHub(t))

	rootCtx, err := wideops.StartRoot(context.Background(), "checkout", observer.RootOption())
	if err != nil {
		t.Fatalf("StartRoot() error = %v", err)
	}

	rootTransaction := sentrysdk.TransactionFromContext(rootCtx)
	if rootTransaction == nil {
		t.Fatal("TransactionFromContext(rootCtx) = nil, want root transaction")
	}

	_, err = wideops.End(rootCtx)
	if err != nil {
		t.Fatalf("End(rootCtx) error = %v", err)
	}

	if rootTransaction.EndTime.IsZero() {
		t.Fatal("rootTransaction.EndTime is zero, want finished root transaction")
	}
}

func TestObserverErrorCapturesExceptionAndMarksActiveSpanFailed(t *testing.T) {
	hub, transport := newTestHubWithTransport(t)
	observer := NewObserver(hub)

	rootCtx, err := wideops.StartRoot(context.Background(), "checkout", observer.RootOption())
	if err != nil {
		t.Fatalf("StartRoot() error = %v", err)
	}

	childCtx, err := wideops.Start(rootCtx, "charge")
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	childSpan := sentrysdk.SpanFromContext(childCtx)
	if childSpan == nil {
		t.Fatal("SpanFromContext(childCtx) = nil, want child span")
	}

	reportErr := errors.New("card declined")
	_, err = wideops.Error(childCtx, reportErr)
	if err != nil {
		t.Fatalf("Error(childCtx, reportErr) error = %v", err)
	}

	if childSpan.Status != sentrysdk.SpanStatusInternalError {
		t.Fatalf("childSpan.Status = %v, want %v", childSpan.Status, sentrysdk.SpanStatusInternalError)
	}

	events := transport.Events()
	if len(events) != 1 {
		t.Fatalf("captured event count = %d, want 1", len(events))
	}

	if got := events[0].Exception[0].Value; got != reportErr.Error() {
		t.Fatalf("captured exception value = %q, want %q", got, reportErr.Error())
	}
}

func TestObserverErrorCapturesEveryNonNilOperationError(t *testing.T) {
	hub, transport := newTestHubWithTransport(t)
	observer := NewObserver(hub)

	rootCtx, err := wideops.StartRoot(context.Background(), "checkout", observer.RootOption())
	if err != nil {
		t.Fatalf("StartRoot() error = %v", err)
	}

	childCtx, err := wideops.Start(rootCtx, "charge")
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	firstErr := errors.New("card declined")
	secondErr := errors.New("issuer unavailable")

	childCtx, err = wideops.Error(childCtx, firstErr)
	if err != nil {
		t.Fatalf("Error(childCtx, firstErr) error = %v", err)
	}

	_, err = wideops.Error(childCtx, secondErr)
	if err != nil {
		t.Fatalf("Error(childCtx, secondErr) error = %v", err)
	}

	events := transport.Events()
	if len(events) != 2 {
		t.Fatalf("captured event count = %d, want 2", len(events))
	}

	if got := events[0].Exception[0].Value; got != firstErr.Error() {
		t.Fatalf("first captured exception value = %q, want %q", got, firstErr.Error())
	}

	if got := events[1].Exception[0].Value; got != secondErr.Error() {
		t.Fatalf("second captured exception value = %q, want %q", got, secondErr.Error())
	}
}

func newTestHub(t *testing.T) *sentrysdk.Hub {
	t.Helper()

	hub, _ := newTestHubWithTransport(t)
	return hub
}

func newTestHubWithTransport(t *testing.T) (*sentrysdk.Hub, *sentrysdk.MockTransport) {
	t.Helper()

	transport := &sentrysdk.MockTransport{}

	client, err := sentrysdk.NewClient(sentrysdk.ClientOptions{
		Dsn:              "https://public@example.com/1",
		Transport:        transport,
		EnableTracing:    true,
		TracesSampleRate: 1,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	return sentrysdk.NewHub(client, sentrysdk.NewScope()), transport
}
