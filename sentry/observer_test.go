package sentry

import (
	"context"
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

func newTestHub(t *testing.T) *sentrysdk.Hub {
	t.Helper()

	client, err := sentrysdk.NewClient(sentrysdk.ClientOptions{
		Dsn:              "https://public@example.com/1",
		Transport:        &sentrysdk.MockTransport{},
		EnableTracing:    true,
		TracesSampleRate: 1,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	return sentrysdk.NewHub(client, sentrysdk.NewScope())
}
