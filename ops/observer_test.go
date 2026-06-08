package ops

import (
	"context"
	"reflect"
	"testing"
)

type observerContextKey string

const derivedKey observerContextKey = "derived"

func TestLifecycleObserverPropagatesFromRootAndCanDeriveContext(t *testing.T) {
	observer := &recordingLifecycleObserver{
		deriveRootStart: func(ctx context.Context) context.Context {
			return context.WithValue(ctx, derivedKey, "root-start")
		},
	}

	rootCtx, err := StartRoot(context.Background(), "root", WithLifecycleObserver(observer))
	assertNoError(t, err)

	t.Run("root start observer can derive context", func(t *testing.T) {
		if got, want := rootCtx.Value(derivedKey), "root-start"; got != want {
			t.Fatalf("root context derived value = %v, want %v", got, want)
		}
	})

	childCtx, err := Start(rootCtx, "child")
	assertNoError(t, err)

	t.Run("root-owned lifecycle observer sees root and child starts", func(t *testing.T) {
		if got, want := observer.startPaths, [][]string{{"root"}, {"root", "child"}}; !reflect.DeepEqual(got, want) {
			t.Fatalf("observer start paths = %v, want %v", got, want)
		}
	})

	t.Run("derived context is visible when child starts", func(t *testing.T) {
		if got, want := observer.startDerivedValues, []string{"", "root-start"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("observer start derived values = %v, want %v", got, want)
		}
	})

	_, err = End(childCtx)
	assertNoError(t, err)

	_, err = End(rootCtx)
	assertNoError(t, err)

	t.Run("root-owned lifecycle observer sees child and root ends", func(t *testing.T) {
		if got, want := observer.endPaths, [][]string{{"root", "child"}, {"root"}}; !reflect.DeepEqual(got, want) {
			t.Fatalf("observer end paths = %v, want %v", got, want)
		}
	})
}

type recordingLifecycleObserver struct {
	deriveRootStart    func(ctx context.Context) context.Context
	startPaths         [][]string
	startDerivedValues []string
	endPaths           [][]string
}

func (o *recordingLifecycleObserver) OnStart(ctx context.Context, op *Operation) context.Context {
	o.startPaths = append(o.startPaths, op.Ops())
	derived, _ := ctx.Value(derivedKey).(string)
	o.startDerivedValues = append(o.startDerivedValues, derived)
	if len(op.Ops()) == 1 && o.deriveRootStart != nil {
		return o.deriveRootStart(ctx)
	}
	return ctx
}

func (o *recordingLifecycleObserver) OnEnd(ctx context.Context, op *Operation) context.Context {
	o.endPaths = append(o.endPaths, op.Ops())
	return ctx
}
