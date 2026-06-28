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
