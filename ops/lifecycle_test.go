package ops

import (
	"context"
	"reflect"
	"testing"
)

func TestEndUnwindsNestedOperationContexts(t *testing.T) {
	baseCtx := context.Background()

	rootCtx, err := StartRoot(baseCtx, "root")
	if err != nil {
		t.Fatalf("StartRoot returned error: %v", err)
	}

	t.Run("StartRoot attaches the root operation", func(t *testing.T) {
		rootOp := GetOperationFromContext(rootCtx)
		if rootOp == nil {
			t.Fatal("StartRoot did not attach an operation to the returned context")
		}
		if got, want := rootOp.Ops(), []string{"root"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("root operation path mismatch: got %v want %v", got, want)
		}
	})

	childCtx, err := Start(rootCtx, "child")
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	t.Run("Start attaches a child operation", func(t *testing.T) {
		childOp := GetOperationFromContext(childCtx)
		if childOp == nil {
			t.Fatal("Start did not attach a child operation to the returned context")
		}
		if got, want := childOp.Ops(), []string{"root", "child"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("child operation path mismatch: got %v want %v", got, want)
		}
	})

	unwoundCtx, err := End(childCtx)
	if err != nil {
		t.Fatalf("End(child) returned error: %v", err)
	}

	t.Run("End restores the parent operation context", func(t *testing.T) {
		unwoundOp := GetOperationFromContext(unwoundCtx)
		if unwoundOp == nil {
			t.Fatal("End(child) did not restore the parent operation context")
		}
		if got, want := unwoundOp.Ops(), []string{"root"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("parent operation path mismatch after End(child): got %v want %v", got, want)
		}
	})

	finishedCtx, err := End(unwoundCtx)
	if err != nil {
		t.Fatalf("End(root) returned error: %v", err)
	}

	t.Run("End on the root clears the active operation", func(t *testing.T) {
		if got := GetOperationFromContext(finishedCtx); got != nil {
			t.Fatalf("End(root) should restore a context with no active operation, got %v", got.Ops())
		}
	})

	t.Run("operation changes do not mutate the original context", func(t *testing.T) {
		if got := GetOperationFromContext(baseCtx); got != nil {
			t.Fatalf("base context should remain unchanged, got %v", got.Ops())
		}
	})
}
