package ops

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestInvalidLifecycleUsageReturnsErrors(t *testing.T) {
	t.Run("Start requires an existing root operation", func(t *testing.T) {
		startedCtx, err := Start(context.Background(), "child")
		assertError(t, err)
		assertErrorIs(t, err, ErrNoActiveOperation)
		assertContextNotNil(t, startedCtx)

		if got := GetOperationFromContext(startedCtx); got != nil {
			t.Fatalf("Start without a root operation should not attach an operation, got %v", got.Ops())
		}
	})

	t.Run("StartRoot rejects creating a nested root operation", func(t *testing.T) {
		rootCtx, err := StartRoot(context.Background(), "root")
		assertNoError(t, err)

		nestedRootCtx, err := StartRoot(rootCtx, "nested-root")
		assertError(t, err)
		assertErrorIs(t, err, ErrNestedRootOperation)
		assertContextNotNil(t, nestedRootCtx)

		got := GetOperationFromContext(nestedRootCtx)
		if got == nil {
			t.Fatal("StartRoot should preserve the existing root operation on validation failure")
		}
		if want := []string{"root"}; !reflect.DeepEqual(got.Ops(), want) {
			t.Fatalf("StartRoot should preserve the active operation path on validation failure: got %v want %v", got.Ops(), want)
		}
	})

	t.Run("End requires an active operation", func(t *testing.T) {
		endedCtx, err := End(context.Background())
		assertError(t, err)
		assertErrorIs(t, err, ErrNoActiveOperation)
		assertContextNotNil(t, endedCtx)

		if got := GetOperationFromContext(endedCtx); got != nil {
			t.Fatalf("End without an active operation should not attach one, got %v", got.Ops())
		}
	})

	t.Run("Error requires an active operation", func(t *testing.T) {
		reportedCtx, err := Error(context.Background(), errors.New("boom"))
		assertError(t, err)
		assertErrorIs(t, err, ErrNoActiveOperation)
		assertContextNotNil(t, reportedCtx)

		if got := GetOperationFromContext(reportedCtx); got != nil {
			t.Fatalf("Error without an active operation should not attach one, got %v", got.Ops())
		}
		if got := GetErrorFromContext(reportedCtx); got != nil {
			t.Fatalf("Error without an active operation should not store an operation error, got %v", got)
		}
	})
}

func assertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
}

func assertErrorIs(t *testing.T, err error, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("errors.Is(%v, %v) = false, want true", err, target)
	}
}

func assertContextNotNil(t *testing.T, ctx context.Context) {
	t.Helper()
	if ctx == nil {
		t.Fatal("expected a non-nil context")
	}
}
