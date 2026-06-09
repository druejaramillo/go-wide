package ops

import (
	"context"
	"log/slog"
	"reflect"
	"testing"
)

func TestWithAttrsAttachesMetadataToTheActiveOperationOnly(t *testing.T) {
	rootCtx, err := StartRoot(context.Background(), "root")
	assertNoError(t, err)

	rootAttrs := []slog.Attr{
		slog.String("request_id", "req-123"),
		slog.String("tenant", "acme"),
	}
	assertNoError(t, WithAttrs(rootCtx, rootAttrs...))

	t.Run("WithAttrs stores metadata on the active root operation", func(t *testing.T) {
		rootOp := GetOperationFromContext(rootCtx)
		if rootOp == nil {
			t.Fatal("expected an active root operation")
		}
		if got := rootOp.Attrs(); !reflect.DeepEqual(got, rootAttrs) {
			t.Fatalf("root operation attrs = %v, want %v", got, rootAttrs)
		}
	})

	childCtx, err := Start(rootCtx, "child")
	assertNoError(t, err)

	childAttrs := []slog.Attr{slog.String("step", "charge-card")}
	assertNoError(t, WithAttrs(childCtx, childAttrs...))

	t.Run("WithAttrs stores metadata on the active child operation", func(t *testing.T) {
		childOp := GetOperationFromContext(childCtx)
		if childOp == nil {
			t.Fatal("expected an active child operation")
		}
		if got := childOp.Attrs(); !reflect.DeepEqual(got, childAttrs) {
			t.Fatalf("child operation attrs = %v, want %v", got, childAttrs)
		}
	})

	t.Run("child metadata does not back-write into the parent operation", func(t *testing.T) {
		rootOp := GetOperationFromContext(rootCtx)
		if rootOp == nil {
			t.Fatal("expected the root operation to remain available on the root context")
		}
		if got := rootOp.Attrs(); !reflect.DeepEqual(got, rootAttrs) {
			t.Fatalf("root operation attrs after child WithAttrs = %v, want %v", got, rootAttrs)
		}
	})
}

func TestWithAttrsRequiresAnActiveOperation(t *testing.T) {
	err := WithAttrs(context.Background(), slog.String("request_id", "req-123"))
	assertError(t, err)
}
