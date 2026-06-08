package ops

import (
	"context"
	"errors"
	"testing"
)

func TestErrorKeepsTheFirstExplicitOperationError(t *testing.T) {
	opCtx, err := StartRoot(context.Background(), "root")
	if err != nil {
		t.Fatalf("StartRoot returned error: %v", err)
	}

	firstErr := errors.New("first operation error")
	secondErr := errors.New("second operation error")
	reportedCtx := opCtx

	t.Run("the first Error call records the operation error", func(t *testing.T) {
		var err error
		reportedCtx, err = Error(reportedCtx, firstErr)
		assertNoError(t, err)

		if got := GetErrorFromContext(reportedCtx); got != firstErr {
			t.Fatalf("GetErrorFromContext() = %v, want %v", got, firstErr)
		}
	})

	t.Run("later Error calls do not replace the first operation error", func(t *testing.T) {
		var err error
		reportedCtx, err = Error(reportedCtx, secondErr)
		assertNoError(t, err)

		if got := GetErrorFromContext(reportedCtx); got != firstErr {
			t.Fatalf("GetErrorFromContext() after second Error = %v, want %v", got, firstErr)
		}
	})
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
