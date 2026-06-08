package ops

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestErrorObserverIsRootOwnedAndNotifiedOnEveryNonNilError(t *testing.T) {
	observer := &recordingErrorObserver{}

	rootCtx, err := StartRoot(context.Background(), "root", WithErrorObserver(observer))
	assertNoError(t, err)

	childCtx, err := Start(rootCtx, "child")
	assertNoError(t, err)

	firstErr := errors.New("first operation error")
	secondErr := errors.New("second operation error")

	reportedCtx, err := Error(childCtx, firstErr)
	assertNoError(t, err)

	t.Run("the first explicit error is stored on the returned context", func(t *testing.T) {
		if got := GetErrorFromContext(reportedCtx); got != firstErr {
			t.Fatalf("GetErrorFromContext() = %v, want %v", got, firstErr)
		}
	})

	reportedCtx, err = Error(reportedCtx, secondErr)
	assertNoError(t, err)

	t.Run("later explicit errors do not replace the stored operation error", func(t *testing.T) {
		if got := GetErrorFromContext(reportedCtx); got != firstErr {
			t.Fatalf("GetErrorFromContext() after second Error = %v, want %v", got, firstErr)
		}
	})

	nilErrCtx, err := Error(reportedCtx, nil)
	assertNoError(t, err)

	t.Run("nil errors are a no-op", func(t *testing.T) {
		if got := GetErrorFromContext(nilErrCtx); got != firstErr {
			t.Fatalf("GetErrorFromContext() after Error(nil) = %v, want %v", got, firstErr)
		}
	})

	t.Run("root-owned error observers see every non-nil error call", func(t *testing.T) {
		if got, want := observer.paths, [][]string{{"root", "child"}, {"root", "child"}}; !reflect.DeepEqual(got, want) {
			t.Fatalf("observer paths = %v, want %v", got, want)
		}
		if got, want := observer.errs, []error{firstErr, secondErr}; !reflect.DeepEqual(got, want) {
			t.Fatalf("observer errors = %v, want %v", got, want)
		}
	})
}

type recordingErrorObserver struct {
	paths [][]string
	errs  []error
}

func (o *recordingErrorObserver) OnError(ctx context.Context, op *Operation, err error) {
	o.paths = append(o.paths, op.Ops())
	o.errs = append(o.errs, err)
}
