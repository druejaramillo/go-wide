package ops

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestRunReportsCallbackErrorsAndUnwindsTheOperation(t *testing.T) {
	lifecycleObserver := &recordingLifecycleObserver{}
	errorObserver := &recordingErrorObserver{}

	rootCtx, err := StartRoot(
		context.Background(),
		"root",
		WithLifecycleObserver(lifecycleObserver),
		WithErrorObserver(errorObserver),
	)
	assertNoError(t, err)

	runErr := errors.New("run failed")
	var callbackPaths [][]string

	err = Run(rootCtx, "child", func(runCtx context.Context) error {
		op := GetOperationFromContext(runCtx)
		if op == nil {
			t.Fatal("Run callback did not receive an active operation")
		}
		callbackPaths = append(callbackPaths, op.Ops())
		return runErr
	})

	t.Run("Run returns the callback error", func(t *testing.T) {
		if !errors.Is(err, runErr) {
			t.Fatalf("Run() error = %v, want %v", err, runErr)
		}
	})

	t.Run("Run starts the child operation before invoking the callback", func(t *testing.T) {
		if got, want := callbackPaths, [][]string{{"root", "child"}}; !reflect.DeepEqual(got, want) {
			t.Fatalf("callback operation paths = %v, want %v", got, want)
		}
	})

	t.Run("Run reports callback failure through the root-owned error observer", func(t *testing.T) {
		if got, want := errorObserver.paths, [][]string{{"root", "child"}}; !reflect.DeepEqual(got, want) {
			t.Fatalf("observer paths = %v, want %v", got, want)
		}
		if got, want := errorObserver.errs, []error{runErr}; !reflect.DeepEqual(got, want) {
			t.Fatalf("observer errors = %v, want %v", got, want)
		}
	})

	t.Run("Run unwinds the child operation before returning", func(t *testing.T) {
		if got, want := lifecycleObserver.endPaths, [][]string{{"root", "child"}}; !reflect.DeepEqual(got, want) {
			t.Fatalf("observer end paths = %v, want %v", got, want)
		}
	})
}
