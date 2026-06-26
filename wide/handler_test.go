package wide

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"reflect"
	"sync"
	"testing"

	"github.com/druejaramillo/go-wide/ops"
)

func TestHandlerPassthroughWithoutOperationContext(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(NewHandler(slog.NewJSONHandler(&output, nil)))

	logger.WithGroup("request").With("id", "req-123").Info("passthrough log", slog.Int("attempt", 1))

	var got map[string]any
	if err := json.Unmarshal(output.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; output = %q", err, output.String())
	}

	request, ok := got["request"].(map[string]any)

	t.Run("preserves message", func(t *testing.T) {
		if got["msg"] != "passthrough log" {
			t.Fatalf("message = %v, want %q", got["msg"], "passthrough log")
		}
	})

	t.Run("preserves level", func(t *testing.T) {
		if got["level"] != "INFO" {
			t.Fatalf("level = %v, want %q", got["level"], "INFO")
		}
	})

	t.Run("preserves structured attrs", func(t *testing.T) {
		if !ok {
			t.Fatalf("request group = %T, want map[string]any", got["request"])
		}

		if request["attempt"] != float64(1) {
			t.Fatalf("request.attempt = %v, want %v", request["attempt"], 1)
		}
	})

	t.Run("preserves grouped attrs", func(t *testing.T) {
		if !ok {
			t.Fatalf("request group = %T, want map[string]any", got["request"])
		}

		if request["id"] != "req-123" {
			t.Fatalf("request.id = %v, want %q", request["id"], "req-123")
		}
	})
}

func TestHandlerPassthroughWithOperationContextWithoutRootAttachment(t *testing.T) {
	rootCtx, err := ops.StartRoot(context.Background(), "root")
	if err != nil {
		t.Fatalf("StartRoot() error = %v", err)
	}

	var output bytes.Buffer
	logger := slog.New(NewHandler(slog.NewJSONHandler(&output, nil)))

	logger.With("request_id", "req-123").InfoContext(rootCtx, "passthrough with operation", slog.Int("attempt", 1))

	var got map[string]any
	if err := json.Unmarshal(output.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; output = %q", err, output.String())
	}

	t.Run("preserves message", func(t *testing.T) {
		if got["msg"] != "passthrough with operation" {
			t.Fatalf("message = %v, want %q", got["msg"], "passthrough with operation")
		}
	})

	t.Run("preserves level", func(t *testing.T) {
		if got["level"] != "INFO" {
			t.Fatalf("level = %v, want %q", got["level"], "INFO")
		}
	})

	t.Run("emits ordinary attrs immediately", func(t *testing.T) {
		if got["request_id"] != "req-123" {
			t.Fatalf("request_id = %v, want %q", got["request_id"], "req-123")
		}

		if got["attempt"] != float64(1) {
			t.Fatalf("attempt = %v, want %v", got["attempt"], 1)
		}
	})
}

func TestHandlerRootOptionAttachesExplicitlyWithoutBreakingPassthrough(t *testing.T) {
	var output bytes.Buffer
	handler := NewHandler(slog.NewJSONHandler(&output, nil))

	rootCtx, err := ops.StartRoot(context.Background(), "root", handler.RootOption())
	if err != nil {
		t.Fatalf("StartRoot() error = %v", err)
	}

	logger := slog.New(handler)
	logger.InfoContext(rootCtx, "attached passthrough log", slog.String("request_id", "req-123"))

	var got map[string]any
	if err := json.Unmarshal(output.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; output = %q", err, output.String())
	}

	t.Run("allows explicit root attachment", func(t *testing.T) {
		if got["msg"] != "attached passthrough log" {
			t.Fatalf("message = %v, want %q", got["msg"], "attached passthrough log")
		}
	})

	t.Run("keeps passthrough output ordinary", func(t *testing.T) {
		if got["request_id"] != "req-123" {
			t.Fatalf("request_id = %v, want %q", got["request_id"], "req-123")
		}
	})
}

func TestHandlerRootOptionAttachesStateToTheRoot(t *testing.T) {
	unattachedCtx, err := ops.StartRoot(context.Background(), "root")
	if err != nil {
		t.Fatalf("StartRoot() without RootOption error = %v", err)
	}

	handler := NewHandler(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	attachedCtx, err := ops.StartRoot(context.Background(), "root", handler.RootOption())
	if err != nil {
		t.Fatalf("StartRoot() with RootOption error = %v", err)
	}

	t.Run("root starts unattached by default", func(t *testing.T) {
		if rootConfigHasAttachmentState(t, unattachedCtx) {
			t.Fatal("expected default root config to have no attachment state")
		}
	})

	t.Run("RootOption attaches handler-owned root state", func(t *testing.T) {
		if !rootConfigHasAttachmentState(t, attachedCtx) {
			t.Fatal("expected RootOption to attach state to the root config")
		}
	})

	t.Run("RootOption registers a root-owned lifecycle hook", func(t *testing.T) {
		counts := rootConfigObserverCounts(t, attachedCtx)
		if counts.lifecycle == 0 {
			t.Fatal("expected RootOption to register at least one lifecycle observer")
		}
	})
}

func TestDerivedHandlerRetainsRootOptionAndPassthrough(t *testing.T) {
	var output bytes.Buffer
	derived := NewHandler(slog.NewJSONHandler(&output, nil)).WithGroup("request").WithAttrs([]slog.Attr{slog.String("id", "req-123")})

	attachable, ok := derived.(interface{ RootOption() ops.Option })
	if !ok {
		t.Fatal("derived handler does not expose RootOption()")
	}

	rootCtx, err := ops.StartRoot(context.Background(), "root", attachable.RootOption())
	if err != nil {
		t.Fatalf("StartRoot() error = %v", err)
	}

	logger := slog.New(derived)
	logger.InfoContext(rootCtx, "derived handler passthrough", slog.Int("attempt", 1))

	var got map[string]any
	if err := json.Unmarshal(output.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; output = %q", err, output.String())
	}

	request, ok := got["request"].(map[string]any)

	t.Run("retains grouped passthrough output", func(t *testing.T) {
		if !ok {
			t.Fatalf("request group = %T, want map[string]any", got["request"])
		}

		if request["id"] != "req-123" {
			t.Fatalf("request.id = %v, want %q", request["id"], "req-123")
		}

		if request["attempt"] != float64(1) {
			t.Fatalf("request.attempt = %v, want %v", request["attempt"], 1)
		}
	})
}

func TestNewHandlerAcceptsOptionSlices(t *testing.T) {
	var opts []Option
	handler := NewHandler(slog.NewJSONHandler(&bytes.Buffer{}, nil), opts...)

	if handler == nil {
		t.Fatal("NewHandler() returned nil")
	}
}

func TestHandlerPassesOrdinaryRecordsToDownstreamWithoutOperationContext(t *testing.T) {
	downstream := &recordingHandler{}
	logger := slog.New(NewHandler(downstream))

	logger.With("request_id", "req-123").Info("passthrough log", slog.Int("attempt", 1))

	records := downstream.Records()
	if len(records) != 1 {
		t.Fatalf("record count = %d, want 1", len(records))
	}

	record := records[0]
	attrs := recordAttrs(record)

	t.Run("preserves ordinary record message", func(t *testing.T) {
		if record.Message != "passthrough log" {
			t.Fatalf("message = %q, want %q", record.Message, "passthrough log")
		}
	})

	t.Run("preserves ordinary record level", func(t *testing.T) {
		if record.Level != slog.LevelInfo {
			t.Fatalf("level = %v, want %v", record.Level, slog.LevelInfo)
		}
	})

	t.Run("preserves ordinary record attrs", func(t *testing.T) {
		if attrs["request_id"] != "req-123" {
			t.Fatalf("request_id = %v, want %q", attrs["request_id"], "req-123")
		}

		if attrs["attempt"] != int64(1) {
			t.Fatalf("attempt = %v, want %v", attrs["attempt"], 1)
		}
	})

	t.Run("does not inject operation attrs", func(t *testing.T) {
		if _, ok := attrs["root"]; ok {
			t.Fatalf("unexpected root attr in ordinary passthrough record: %v", attrs)
		}
	})
}

func TestAttachedRootPassthroughEmitsImmediateOrdinaryRecordsForChildOperations(t *testing.T) {
	downstream := &recordingHandler{}
	handler := NewHandler(downstream)

	rootCtx, err := ops.StartRoot(context.Background(), "root", handler.RootOption())
	if err != nil {
		t.Fatalf("StartRoot() error = %v", err)
	}

	childCtx, err := ops.Start(rootCtx, "child")
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	logger := slog.New(handler)
	logger.With("request_id", "req-123").InfoContext(childCtx, "child passthrough", slog.Int("attempt", 1))

	beforeEnd := downstream.Records()
	if len(beforeEnd) != 1 {
		t.Fatalf("record count before End = %d, want 1", len(beforeEnd))
	}

	record := beforeEnd[0]
	attrs := recordAttrs(record)

	t.Run("emits before operation end", func(t *testing.T) {
		if record.Message != "child passthrough" {
			t.Fatalf("message = %q, want %q", record.Message, "child passthrough")
		}
	})

	t.Run("keeps child passthrough record ordinary", func(t *testing.T) {
		if attrs["request_id"] != "req-123" {
			t.Fatalf("request_id = %v, want %q", attrs["request_id"], "req-123")
		}

		if attrs["attempt"] != int64(1) {
			t.Fatalf("attempt = %v, want %v", attrs["attempt"], 1)
		}

		if _, ok := attrs["root"]; ok {
			t.Fatalf("unexpected root attr in ordinary passthrough record: %v", attrs)
		}
		if _, ok := attrs["child"]; ok {
			t.Fatalf("unexpected child attr in ordinary passthrough record: %v", attrs)
		}
	})

	if _, err := ops.End(childCtx); err != nil {
		t.Fatalf("End(child) error = %v", err)
	}
	if _, err := ops.End(rootCtx); err != nil {
		t.Fatalf("End(root) error = %v", err)
	}

	t.Run("does not emit extra records during end", func(t *testing.T) {
		afterEnd := downstream.Records()
		if len(afterEnd) != 1 {
			t.Fatalf("record count after End = %d, want 1", len(afterEnd))
		}
	})
}

func TestAggregateRootEmissionForSingleOperation(t *testing.T) {
	downstream := newDisabledRecordingHandler()
	handler := NewHandler(downstream, WithAggregate())

	rootCtx, err := ops.StartRoot(context.Background(), "root", handler.RootOption())
	if err != nil {
		t.Fatalf("StartRoot() error = %v", err)
	}
	if err := ops.WithAttrs(
		rootCtx,
		slog.String("request_id", "req-123"),
		slog.String("tenant", "acme"),
	); err != nil {
		t.Fatalf("WithAttrs() error = %v", err)
	}

	logger := slog.New(handler)
	logger.InfoContext(rootCtx, "root log", slog.Int("attempt", 1))

	t.Run("buffers logs until root end", func(t *testing.T) {
		if got := downstream.Records(); len(got) != 0 {
			t.Fatalf("record count before End(root) = %d, want 0", len(got))
		}
	})

	_, err = ops.End(rootCtx)
	if err != nil {
		t.Fatalf("End(root) error = %v", err)
	}

	records := downstream.Records()
	if len(records) != 1 {
		t.Fatalf("record count after End(root) = %d, want 1", len(records))
	}

	record := records[0]
	attrs := recordAttrs(record)

	t.Run("emits exactly one finalized slog record on root end", func(t *testing.T) {
		if len(records) != 1 {
			t.Fatalf("record count = %d, want 1", len(records))
		}
	})

	t.Run("includes reserved root wide-event attrs", func(t *testing.T) {
		if attrs["request_id"] != "req-123" {
			t.Fatalf("request_id = %v, want %q", attrs["request_id"], "req-123")
		}
		if attrs["tenant"] != "acme" {
			t.Fatalf("tenant = %v, want %q", attrs["tenant"], "acme")
		}
		if attrs["version"] != int64(1) {
			t.Fatalf("version = %v, want %v", attrs["version"], 1)
		}
		if _, ok := attrs["status"]; !ok {
			t.Fatalf("status attr missing from finalized record: %v", attrs)
		}
		if _, ok := attrs["start"]; !ok {
			t.Fatalf("start attr missing from finalized record: %v", attrs)
		}
		if _, ok := attrs["end"]; !ok {
			t.Fatalf("end attr missing from finalized record: %v", attrs)
		}
		if _, ok := attrs["logs"]; !ok {
			t.Fatalf("logs attr missing from finalized record: %v", attrs)
		}
	})

	t.Run("bypasses ordinary downstream filtering during final emission", func(t *testing.T) {
		if len(records) != 1 {
			t.Fatalf("record count = %d, want 1", len(records))
		}
	})
}

func TestAggregateEndedChildContextFallsBackToPassthrough(t *testing.T) {
	downstream := &recordingHandler{}
	handler := NewHandler(downstream, WithAggregate())

	rootCtx, err := ops.StartRoot(context.Background(), "root", handler.RootOption())
	if err != nil {
		t.Fatalf("StartRoot() error = %v", err)
	}

	childCtx, err := ops.Start(rootCtx, "child")
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	logger := slog.New(handler)
	logger.InfoContext(childCtx, "before child end")

	if _, err := ops.End(childCtx); err != nil {
		t.Fatalf("End(child) error = %v", err)
	}

	logger.With("request_id", "req-123").InfoContext(childCtx, "after child end", slog.Int("attempt", 2))

	beforeRootEnd := downstream.Records()
	if len(beforeRootEnd) != 1 {
		t.Fatalf("record count before End(root) = %d, want 1", len(beforeRootEnd))
	}

	passthrough := beforeRootEnd[0]
	passthroughAttrs := recordAttrs(passthrough)

	if passthrough.Message != "after child end" {
		t.Fatalf("passthrough message = %q, want %q", passthrough.Message, "after child end")
	}
	if passthroughAttrs["request_id"] != "req-123" {
		t.Fatalf("request_id = %v, want %q", passthroughAttrs["request_id"], "req-123")
	}
	if passthroughAttrs["attempt"] != int64(2) {
		t.Fatalf("attempt = %v, want %v", passthroughAttrs["attempt"], 2)
	}
	if _, ok := passthroughAttrs["logs"]; ok {
		t.Fatalf("unexpected aggregate attrs in passthrough record: %v", passthroughAttrs)
	}

	if _, err := ops.End(rootCtx); err != nil {
		t.Fatalf("End(root) error = %v", err)
	}

	records := downstream.Records()
	if len(records) != 2 {
		t.Fatalf("record count after End(root) = %d, want 2", len(records))
	}

	attrs := recordAttrTree(records[1])
	child, ok := attrs["child"].(map[string]any)
	if !ok {
		t.Fatalf("child = %T, want map[string]any", attrs["child"])
	}

	logs, ok := child["logs"].(map[string]any)
	if !ok {
		t.Fatalf("child.logs = %T, want map[string]any", child["logs"])
	}

	if _, ok := logs["before child end"]; !ok {
		t.Fatalf("logs missing buffered child message: %v", logs)
	}
	if _, ok := logs["after child end"]; ok {
		t.Fatalf("logs unexpectedly included ended child context message: %v", logs)
	}
}

func TestAggregateRootEndReturnsTypedErrorWhenFinalEmissionFails(t *testing.T) {
	emitErr := errors.New("downstream write failed")
	handler := NewHandler(&erroringHandler{err: emitErr}, WithAggregate())

	rootCtx, err := ops.StartRoot(context.Background(), "root", handler.RootOption())
	if err != nil {
		t.Fatalf("StartRoot() error = %v", err)
	}

	slog.New(handler).InfoContext(rootCtx, "root log")

	_, err = ops.End(rootCtx)
	if err == nil {
		t.Fatal("End(root) error = nil, want typed root-end emission error")
	}
	if !errors.Is(err, ops.ErrRootEndEmission) {
		t.Fatalf("errors.Is(%v, %v) = false, want true", err, ops.ErrRootEndEmission)
	}
	if !errors.Is(err, emitErr) {
		t.Fatalf("errors.Is(%v, %v) = false, want true", err, emitErr)
	}
}

func TestAggregateOverflowFallsBackToDiagnosticThenPassthrough(t *testing.T) {
	downstream := &recordingHandler{}
	handler := NewHandler(downstream, WithAggregate(), WithAggregateLimit(1))

	rootCtx, err := ops.StartRoot(context.Background(), "root", handler.RootOption())
	if err != nil {
		t.Fatalf("StartRoot() error = %v", err)
	}

	logger := slog.New(handler)
	logger.InfoContext(rootCtx, "first buffered")

	if got := downstream.Records(); len(got) != 0 {
		t.Fatalf("record count before overflow = %d, want 0", len(got))
	}

	logger.InfoContext(rootCtx, "second overflows")

	afterOverflow := downstream.Records()
	if len(afterOverflow) != 1 {
		t.Fatalf("record count after overflow = %d, want 1", len(afterOverflow))
	}

	diagnostic := afterOverflow[0]
	diagnosticAttrs := recordAttrs(diagnostic)

	if diagnostic.Message != "wide aggregate overflow" {
		t.Fatalf("diagnostic message = %q, want %q", diagnostic.Message, "wide aggregate overflow")
	}
	if diagnosticAttrs["reason"] != "limit_exceeded" {
		t.Fatalf("reason = %v, want %q", diagnosticAttrs["reason"], "limit_exceeded")
	}
	if diagnosticAttrs["limit"] != int64(1) {
		t.Fatalf("limit = %v, want %v", diagnosticAttrs["limit"], 1)
	}

	logger.With("request_id", "req-123").InfoContext(rootCtx, "after overflow", slog.Int("attempt", 3))

	beforeRootEnd := downstream.Records()
	if len(beforeRootEnd) != 2 {
		t.Fatalf("record count before End(root) = %d, want 2", len(beforeRootEnd))
	}

	passthrough := beforeRootEnd[1]
	passthroughAttrs := recordAttrs(passthrough)

	if passthrough.Message != "after overflow" {
		t.Fatalf("passthrough message = %q, want %q", passthrough.Message, "after overflow")
	}
	if passthroughAttrs["request_id"] != "req-123" {
		t.Fatalf("request_id = %v, want %q", passthroughAttrs["request_id"], "req-123")
	}
	if passthroughAttrs["attempt"] != int64(3) {
		t.Fatalf("attempt = %v, want %v", passthroughAttrs["attempt"], 3)
	}
	if _, ok := passthroughAttrs["logs"]; ok {
		t.Fatalf("unexpected aggregate attrs in passthrough record: %v", passthroughAttrs)
	}

	if _, err := ops.End(rootCtx); err != nil {
		t.Fatalf("End(root) error = %v", err)
	}

	afterRootEnd := downstream.Records()
	if len(afterRootEnd) != 2 {
		t.Fatalf("record count after End(root) = %d, want 2", len(afterRootEnd))
	}
}

func TestAggregateInvalidLimitReturnsTypedRootOptionError(t *testing.T) {
	handler := NewHandler(&recordingHandler{}, WithAggregate(), WithAggregateLimit(-1))

	rootCtx, err := ops.StartRoot(context.Background(), "root", handler.RootOption())
	if err == nil {
		t.Fatal("StartRoot() error = nil, want typed invalid option usage error")
	}
	if !errors.Is(err, ops.ErrInvalidOptionUsage) {
		t.Fatalf("errors.Is(%v, %v) = false, want true", err, ops.ErrInvalidOptionUsage)
	}
	if got := ops.GetOperationFromContext(rootCtx); got != nil {
		t.Fatalf("StartRoot() should not attach an operation on invalid option usage, got %v", got.Ops())
	}
}

func TestAggregateRootEmissionIncludesChildOperationAttrsAsDirectGroups(t *testing.T) {
	downstream := &recordingHandler{}
	handler := NewHandler(downstream, WithAggregate())

	rootCtx, err := ops.StartRoot(context.Background(), "root", handler.RootOption())
	if err != nil {
		t.Fatalf("StartRoot() error = %v", err)
	}

	if err := ops.WithAttrs(rootCtx, slog.String("request_id", "req-123")); err != nil {
		t.Fatalf("WithAttrs(root) error = %v", err)
	}

	childCtx, err := ops.Start(rootCtx, "child")
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if err := ops.WithAttrs(childCtx, slog.String("step", "charge-card")); err != nil {
		t.Fatalf("WithAttrs(child) error = %v", err)
	}

	if _, err := ops.End(childCtx); err != nil {
		t.Fatalf("End(child) error = %v", err)
	}

	if _, err := ops.End(rootCtx); err != nil {
		t.Fatalf("End(root) error = %v", err)
	}

	records := downstream.Records()
	if len(records) != 1 {
		t.Fatalf("record count after End(root) = %d, want 1", len(records))
	}

	attrs := recordAttrTree(records[0])
	child, ok := attrs["child"].(map[string]any)
	if !ok {
		t.Fatalf("child = %T, want map[string]any", attrs["child"])
	}

	if attrs["request_id"] != "req-123" {
		t.Fatalf("request_id = %v, want %q", attrs["request_id"], "req-123")
	}

	if child["step"] != "charge-card" {
		t.Fatalf("child.step = %v, want %q", child["step"], "charge-card")
	}
}

func TestAggregateRootEmissionPreservesLoggerDerivedGroupedAttrs(t *testing.T) {
	downstream := &recordingHandler{}
	handler := NewHandler(downstream, WithAggregate())

	rootCtx, err := ops.StartRoot(context.Background(), "root", handler.RootOption())
	if err != nil {
		t.Fatalf("StartRoot() error = %v", err)
	}

	logger := slog.New(handler).WithGroup("request").With("id", "req-123")
	logger.InfoContext(rootCtx, "root log", slog.Int("attempt", 1))

	_, err = ops.End(rootCtx)
	if err != nil {
		t.Fatalf("End(root) error = %v", err)
	}

	records := downstream.Records()
	if len(records) != 1 {
		t.Fatalf("record count after End(root) = %d, want 1", len(records))
	}

	attrs := recordAttrTree(records[0])
	logs, ok := attrs["logs"].(map[string]any)
	if !ok {
		t.Fatalf("logs = %T, want map[string]any", attrs["logs"])
	}

	entry := logBucketByMessage(t, logs, "root log")

	request, ok := entry["request"].(map[string]any)
	if !ok {
		t.Fatalf("logs[%q].request = %T, want map[string]any", "root log", entry["request"])
	}

	if request["id"] != "req-123" {
		t.Fatalf("logs[%q].request.id = %v, want %q", "root log", request["id"], "req-123")
	}
}

func TestAggregateRootOptionFromDerivedHandlerDoesNotLeakLoggerStateIntoFinalWideEvent(t *testing.T) {
	var output bytes.Buffer
	derived := NewHandler(slog.NewJSONHandler(&output, nil), WithAggregate()).WithGroup("request").WithAttrs([]slog.Attr{slog.String("id", "req-123")})

	attachable, ok := derived.(interface{ RootOption() ops.Option })
	if !ok {
		t.Fatal("derived handler does not expose RootOption()")
	}

	rootCtx, err := ops.StartRoot(context.Background(), "root", attachable.RootOption())
	if err != nil {
		t.Fatalf("StartRoot() error = %v", err)
	}

	if err := ops.WithAttrs(rootCtx, slog.String("tenant", "acme")); err != nil {
		t.Fatalf("WithAttrs(root) error = %v", err)
	}

	slog.New(derived).InfoContext(rootCtx, "derived log")

	if _, err := ops.End(rootCtx); err != nil {
		t.Fatalf("End(root) error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(output.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; output = %q", err, output.String())
	}

	if got["tenant"] != "acme" {
		t.Fatalf("tenant = %v, want %q", got["tenant"], "acme")
	}

	if _, ok := got["request"]; ok {
		t.Fatalf("unexpected top-level request group in final wide-event: %v", got)
	}

	logs, ok := got["logs"].(map[string]any)
	if !ok {
		t.Fatalf("logs = %T, want map[string]any", got["logs"])
	}

	entry := logBucketByMessage(t, logs, "derived log")

	request, ok := entry["request"].(map[string]any)
	if !ok {
		t.Fatalf("logs[%q].request = %T, want map[string]any", "derived log", entry["request"])
	}

	if request["id"] != "req-123" {
		t.Fatalf("logs[%q].request.id = %v, want %q", "derived log", request["id"], "req-123")
	}
}

func TestAggregateRootEmissionScopesDerivedLoggerStateToOnlyDerivedRecords(t *testing.T) {
	downstream := &recordingHandler{}
	handler := NewHandler(downstream, WithAggregate())

	rootCtx, err := ops.StartRoot(context.Background(), "root", handler.RootOption())
	if err != nil {
		t.Fatalf("StartRoot() error = %v", err)
	}

	childCtx, err := ops.Start(rootCtx, "child")
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if err := ops.WithAttrs(childCtx, slog.String("step", "charge-card")); err != nil {
		t.Fatalf("WithAttrs(child) error = %v", err)
	}

	base := slog.New(handler)
	derived := base.WithGroup("request").With("id", "req-123")

	derived.InfoContext(childCtx, "derived log")
	base.InfoContext(childCtx, "base log")

	if _, err := ops.End(childCtx); err != nil {
		t.Fatalf("End(child) error = %v", err)
	}

	if _, err := ops.End(rootCtx); err != nil {
		t.Fatalf("End(root) error = %v", err)
	}

	records := downstream.Records()
	if len(records) != 1 {
		t.Fatalf("record count after End(root) = %d, want 1", len(records))
	}

	attrs := recordAttrTree(records[0])
	child, ok := attrs["child"].(map[string]any)
	if !ok {
		t.Fatalf("child = %T, want map[string]any", attrs["child"])
	}

	if child["step"] != "charge-card" {
		t.Fatalf("child.step = %v, want %q", child["step"], "charge-card")
	}

	if _, ok := child["request"]; ok {
		t.Fatalf("unexpected request group written into child operation attrs: %v", child)
	}

	logs, ok := child["logs"].(map[string]any)
	if !ok {
		t.Fatalf("child.logs = %T, want map[string]any", child["logs"])
	}

	derivedEntry := logBucketByMessage(t, logs, "derived log")
	baseEntry := logBucketByMessage(t, logs, "base log")

	request, ok := derivedEntry["request"].(map[string]any)
	if !ok {
		t.Fatalf("child.logs[%q].request = %T, want map[string]any", "derived log", derivedEntry["request"])
	}

	if request["id"] != "req-123" {
		t.Fatalf("child.logs[%q].request.id = %v, want %q", "derived log", request["id"], "req-123")
	}

	if _, ok := baseEntry["request"]; ok {
		t.Fatalf("unexpected request group on base logger entry: %v", baseEntry)
	}
}

func TestAggregateRootEmissionMergesRepeatedLogsByStructure(t *testing.T) {
	downstream := &recordingHandler{}
	handler := NewHandler(downstream, WithAggregate())

	rootCtx, err := ops.StartRoot(context.Background(), "root", handler.RootOption())
	if err != nil {
		t.Fatalf("StartRoot() error = %v", err)
	}

	logger := slog.New(handler).With("component", "billing")
	logger.InfoContext(rootCtx, "retrying charge", slog.String("phase", "charge"), slog.Int("attempt", 1))
	logger.InfoContext(rootCtx, "retrying charge", slog.String("phase", "charge"), slog.Int("attempt", 2))

	if _, err := ops.End(rootCtx); err != nil {
		t.Fatalf("End(root) error = %v", err)
	}

	records := downstream.Records()
	if len(records) != 1 {
		t.Fatalf("record count after End(root) = %d, want 1", len(records))
	}

	attrs := recordAttrTree(records[0])
	logs, ok := attrs["logs"].(map[string]any)
	if !ok {
		t.Fatalf("logs = %T, want map[string]any", attrs["logs"])
	}

	if len(logs) != 1 {
		t.Fatalf("log bucket count = %d, want 1 merged bucket; logs = %v", len(logs), logs)
	}

	bucket, ok := logs["retrying charge"].(map[string]any)
	if !ok {
		t.Fatalf("logs[%q] = %T, want map[string]any; logs = %v", "retrying charge", logs["retrying charge"], logs)
	}

	if bucket["message"] != "retrying charge" {
		t.Fatalf("bucket.message = %v, want %q", bucket["message"], "retrying charge")
	}

	if bucket["level"] != "INFO" {
		t.Fatalf("bucket.level = %v, want %q", bucket["level"], "INFO")
	}

	if bucket["count"] != int64(2) {
		t.Fatalf("bucket.count = %v, want %v", bucket["count"], 2)
	}

	if bucket["component"] != "billing" {
		t.Fatalf("bucket.component = %v, want %q", bucket["component"], "billing")
	}

	if bucket["phase"] != "charge" {
		t.Fatalf("bucket.phase = %v, want %q", bucket["phase"], "charge")
	}

	if _, ok := bucket["time"]; ok {
		t.Fatalf("bucket.time should not be present in structural summary, bucket = %v", bucket)
	}

	if _, ok := bucket["attempt"]; ok {
		t.Fatalf("bucket.attempt should be summarized under variants, bucket = %v", bucket)
	}

	variants, ok := bucket["variants"].(map[string]any)
	if !ok {
		t.Fatalf("bucket.variants = %T, want map[string]any", bucket["variants"])
	}

	attempts, ok := variants["attempt"].([]any)
	if !ok {
		t.Fatalf("bucket.variants.attempt = %T, want []any", variants["attempt"])
	}

	if !reflect.DeepEqual(attempts, []any{int64(1), int64(2)}) {
		t.Fatalf("bucket.variants.attempt = %v, want %v", attempts, []any{int64(1), int64(2)})
	}
}

func TestAggregateRootEmissionMergesSameNamedChildOperationsByStructure(t *testing.T) {
	downstream := &recordingHandler{}
	handler := NewHandler(downstream, WithAggregate())

	rootCtx, err := ops.StartRoot(context.Background(), "root", handler.RootOption())
	if err != nil {
		t.Fatalf("StartRoot() error = %v", err)
	}

	firstChildCtx, err := ops.Start(rootCtx, "charge")
	if err != nil {
		t.Fatalf("Start(first child) error = %v", err)
	}
	if err := ops.WithAttrs(
		firstChildCtx,
		slog.String("provider", "stripe"),
		slog.String("phase", "authorize"),
	); err != nil {
		t.Fatalf("WithAttrs(first child) error = %v", err)
	}
	if _, err := ops.End(firstChildCtx); err != nil {
		t.Fatalf("End(first child) error = %v", err)
	}

	secondChildCtx, err := ops.Start(rootCtx, "charge")
	if err != nil {
		t.Fatalf("Start(second child) error = %v", err)
	}
	if err := ops.WithAttrs(
		secondChildCtx,
		slog.String("provider", "stripe"),
		slog.String("phase", "capture"),
	); err != nil {
		t.Fatalf("WithAttrs(second child) error = %v", err)
	}
	if _, err := ops.End(secondChildCtx); err != nil {
		t.Fatalf("End(second child) error = %v", err)
	}

	if _, err := ops.End(rootCtx); err != nil {
		t.Fatalf("End(root) error = %v", err)
	}

	records := downstream.Records()
	if len(records) != 1 {
		t.Fatalf("record count after End(root) = %d, want 1", len(records))
	}

	attrs := recordAttrTree(records[0])
	child, ok := attrs["charge"].(map[string]any)
	if !ok {
		t.Fatalf("charge = %T, want map[string]any", attrs["charge"])
	}

	if child["count"] != int64(2) {
		t.Fatalf("charge.count = %v, want %v", child["count"], 2)
	}

	if child["provider"] != "stripe" {
		t.Fatalf("charge.provider = %v, want %q", child["provider"], "stripe")
	}

	if _, ok := child["phase"]; ok {
		t.Fatalf("charge.phase should be summarized under variants, child = %v", child)
	}

	variants, ok := child["variants"].(map[string]any)
	if !ok {
		t.Fatalf("charge.variants = %T, want map[string]any", child["variants"])
	}

	phases, ok := variants["phase"].([]any)
	if !ok {
		t.Fatalf("charge.variants.phase = %T, want []any", variants["phase"])
	}

	if !reflect.DeepEqual(phases, []any{"authorize", "capture"}) {
		t.Fatalf("charge.variants.phase = %v, want %v", phases, []any{"authorize", "capture"})
	}
}

func TestAggregateRootEmissionPreservesDivergentErrorOutcomesForSameNamedChildren(t *testing.T) {
	downstream := &recordingHandler{}
	handler := NewHandler(downstream, WithAggregate())

	rootCtx, err := ops.StartRoot(context.Background(), "root", handler.RootOption())
	if err != nil {
		t.Fatalf("StartRoot() error = %v", err)
	}

	firstChildCtx, err := ops.Start(rootCtx, "charge")
	if err != nil {
		t.Fatalf("Start(first child) error = %v", err)
	}
	if err := ops.WithAttrs(firstChildCtx, slog.String("provider", "stripe")); err != nil {
		t.Fatalf("WithAttrs(first child) error = %v", err)
	}
	if _, err := ops.End(firstChildCtx); err != nil {
		t.Fatalf("End(first child) error = %v", err)
	}

	secondChildCtx, err := ops.Start(rootCtx, "charge")
	if err != nil {
		t.Fatalf("Start(second child) error = %v", err)
	}
	if err := ops.WithAttrs(secondChildCtx, slog.String("provider", "stripe")); err != nil {
		t.Fatalf("WithAttrs(second child) error = %v", err)
	}
	secondChildCtx, err = ops.Error(secondChildCtx, context.DeadlineExceeded)
	if err != nil {
		t.Fatalf("Error(second child) error = %v", err)
	}
	if _, err := ops.End(secondChildCtx); err != nil {
		t.Fatalf("End(second child) error = %v", err)
	}

	if _, err := ops.End(rootCtx); err != nil {
		t.Fatalf("End(root) error = %v", err)
	}

	records := downstream.Records()
	if len(records) != 1 {
		t.Fatalf("record count after End(root) = %d, want 1", len(records))
	}

	attrs := recordAttrTree(records[0])
	child, ok := attrs["charge"].(map[string]any)
	if !ok {
		t.Fatalf("charge = %T, want map[string]any", attrs["charge"])
	}

	if child["count"] != int64(2) {
		t.Fatalf("charge.count = %v, want %v", child["count"], 2)
	}

	if child["provider"] != "stripe" {
		t.Fatalf("charge.provider = %v, want %q", child["provider"], "stripe")
	}

	if _, ok := child["status"]; ok {
		t.Fatalf("charge.status should be summarized under variants, child = %v", child)
	}

	if _, ok := child["error"]; ok {
		t.Fatalf("charge.error should be summarized under variants, child = %v", child)
	}

	variants, ok := child["variants"].(map[string]any)
	if !ok {
		t.Fatalf("charge.variants = %T, want map[string]any", child["variants"])
	}

	statuses, ok := variants["status"].([]any)
	if !ok {
		t.Fatalf("charge.variants.status = %T, want []any", variants["status"])
	}

	if !reflect.DeepEqual(statuses, []any{"ok", "error"}) {
		t.Fatalf("charge.variants.status = %v, want %v", statuses, []any{"ok", "error"})
	}

	errors, ok := variants["error"].([]any)
	if !ok {
		t.Fatalf("charge.variants.error = %T, want []any", variants["error"])
	}

	if !reflect.DeepEqual(errors, []any{context.DeadlineExceeded.Error()}) {
		t.Fatalf("charge.variants.error = %v, want %v", errors, []any{context.DeadlineExceeded.Error()})
	}
}

func TestAggregateRootEmissionMarksFinalStatusErrorAfterReportedError(t *testing.T) {
	downstream := &recordingHandler{}
	handler := NewHandler(downstream, WithAggregate())

	rootCtx, err := ops.StartRoot(context.Background(), "root", handler.RootOption())
	if err != nil {
		t.Fatalf("StartRoot() error = %v", err)
	}

	rootCtx, err = ops.Error(rootCtx, context.Canceled)
	if err != nil {
		t.Fatalf("Error() error = %v", err)
	}

	_, err = ops.End(rootCtx)
	if err != nil {
		t.Fatalf("End(root) error = %v", err)
	}

	records := downstream.Records()
	if len(records) != 1 {
		t.Fatalf("record count after End(root) = %d, want 1", len(records))
	}

	attrs := recordAttrs(records[0])
	if attrs["status"] != "error" {
		t.Fatalf("status = %v, want %q", attrs["status"], "error")
	}
}

func TestAggregateRootEmissionSerializesAsOrdinaryJSONRecord(t *testing.T) {
	var output bytes.Buffer
	handler := NewHandler(slog.NewJSONHandler(&output, nil), WithAggregate())

	rootCtx, err := ops.StartRoot(context.Background(), "root", handler.RootOption())
	if err != nil {
		t.Fatalf("StartRoot() error = %v", err)
	}

	if err := ops.WithAttrs(rootCtx, slog.String("request_id", "req-123")); err != nil {
		t.Fatalf("WithAttrs() error = %v", err)
	}

	slog.New(handler).InfoContext(rootCtx, "root log")

	_, err = ops.End(rootCtx)
	if err != nil {
		t.Fatalf("End(root) error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(output.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; output = %q", err, output.String())
	}

	if got["msg"] != "root" {
		t.Fatalf("msg = %v, want %q", got["msg"], "root")
	}

	if got["level"] != "INFO" {
		t.Fatalf("level = %v, want %q", got["level"], "INFO")
	}

	if got["request_id"] != "req-123" {
		t.Fatalf("request_id = %v, want %q", got["request_id"], "req-123")
	}

	if _, ok := got["logs"]; !ok {
		t.Fatalf("logs missing from serialized aggregate record: %v", got)
	}
}

type recordingHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *recordingHandler) Handle(_ context.Context, record slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, record.Clone())
	return nil
}

func (h *recordingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &recordingHandlerWithAttrs{parent: h, attrs: append([]slog.Attr(nil), attrs...)}
}

func (h *recordingHandler) WithGroup(name string) slog.Handler {
	return &recordingHandlerWithGroup{parent: h, name: name}
}

func (h *recordingHandler) Records() []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]slog.Record(nil), h.records...)
}

type disabledRecordingHandler struct {
	*recordingHandler
}

type erroringHandler struct {
	err error
}

func (h *erroringHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *erroringHandler) Handle(context.Context, slog.Record) error {
	return h.err
}

func (h *erroringHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *erroringHandler) WithGroup(string) slog.Handler {
	return h
}

func newDisabledRecordingHandler() *disabledRecordingHandler {
	return &disabledRecordingHandler{
		recordingHandler: &recordingHandler{},
	}
}

func (h *disabledRecordingHandler) Enabled(context.Context, slog.Level) bool {
	return false
}

type recordingHandlerWithAttrs struct {
	parent *recordingHandler
	attrs  []slog.Attr
}

func (h *recordingHandlerWithAttrs) Enabled(ctx context.Context, level slog.Level) bool {
	return h.parent.Enabled(ctx, level)
}

func (h *recordingHandlerWithAttrs) Handle(ctx context.Context, record slog.Record) error {
	clone := record.Clone()
	clone.AddAttrs(h.attrs...)
	return h.parent.Handle(ctx, clone)
}

func (h *recordingHandlerWithAttrs) WithAttrs(attrs []slog.Attr) slog.Handler {
	combined := append(append([]slog.Attr(nil), h.attrs...), attrs...)
	return &recordingHandlerWithAttrs{parent: h.parent, attrs: combined}
}

func (h *recordingHandlerWithAttrs) WithGroup(name string) slog.Handler {
	return &recordingHandlerWithGroup{parent: h.parent, name: name}
}

type recordingHandlerWithGroup struct {
	parent *recordingHandler
	name   string
}

func (h *recordingHandlerWithGroup) Enabled(ctx context.Context, level slog.Level) bool {
	return h.parent.Enabled(ctx, level)
}

func (h *recordingHandlerWithGroup) Handle(ctx context.Context, record slog.Record) error {
	clone := record.Clone()
	var grouped []slog.Attr
	clone.Attrs(func(attr slog.Attr) bool {
		grouped = append(grouped, attr)
		return true
	})
	clone = slog.NewRecord(clone.Time, clone.Level, clone.Message, clone.PC)
	clone.AddAttrs(slog.Attr{Key: h.name, Value: slog.GroupValue(grouped...)})
	return h.parent.Handle(ctx, clone)
}

func (h *recordingHandlerWithGroup) WithAttrs(attrs []slog.Attr) slog.Handler {
	return (&recordingHandlerWithAttrs{parent: h.parent, attrs: attrs}).WithGroup(h.name)
}

func (h *recordingHandlerWithGroup) WithGroup(name string) slog.Handler {
	return &recordingHandlerWithGroup{parent: h.parent, name: name}
}

func recordAttrs(record slog.Record) map[string]any {
	attrs := map[string]any{}
	record.Attrs(func(attr slog.Attr) bool {
		attrs[attr.Key] = attr.Value.Any()
		return true
	})
	return attrs
}

func recordAttrTree(record slog.Record) map[string]any {
	attrs := map[string]any{}
	record.Attrs(func(attr slog.Attr) bool {
		attrs[attr.Key] = attrTreeValue(attr.Value)
		return true
	})
	return attrs
}

func attrTreeValue(value slog.Value) any {
	value = value.Resolve()
	if value.Kind() != slog.KindGroup {
		return value.Any()
	}

	attrs := map[string]any{}
	for _, attr := range value.Group() {
		attrs[attr.Key] = attrTreeValue(attr.Value)
	}
	return attrs
}

func logBucketByMessage(t *testing.T, logs map[string]any, message string) map[string]any {
	t.Helper()
	bucket, ok := logs[message].(map[string]any)
	if !ok {
		t.Fatalf("logs[%q] = %T, want map[string]any; logs = %v", message, logs[message], logs)
	}
	return bucket
}

func rootConfigHasAttachmentState(t *testing.T, ctx context.Context) bool {
	t.Helper()
	counts := rootConfigObserverCounts(t, ctx)
	if counts.lifecycle != 0 || counts.error != 0 {
		return true
	}

	rootConfig := rootConfigValue(t, ctx)
	for _, field := range rootConfig.Fields() {
		if field.Kind() == reflect.Slice && field.Len() == 0 {
			continue
		}
		if !field.IsZero() {
			return true
		}
	}

	return false
}

type observerCounts struct {
	lifecycle int
	error     int
}

func rootConfigObserverCounts(t *testing.T, ctx context.Context) observerCounts {
	t.Helper()
	rootConfig := rootConfigValue(t, ctx)
	return observerCounts{
		lifecycle: rootConfig.FieldByName("LifecycleObservers").Len(),
		error:     rootConfig.FieldByName("ErrorObservers").Len(),
	}
}

func rootConfigValue(t *testing.T, ctx context.Context) reflect.Value {
	t.Helper()

	op := ops.GetOperationFromContext(ctx)
	if op == nil {
		t.Fatal("expected an active root operation")
	}

	rootConfig := reflect.ValueOf(op).Elem().FieldByName("rootConfig")
	if !rootConfig.IsValid() || rootConfig.IsNil() {
		t.Fatal("expected root operation to hold a root config")
	}

	return rootConfig.Elem()
}
