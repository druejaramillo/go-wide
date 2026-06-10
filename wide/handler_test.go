package wide

import (
	"bytes"
	"context"
	"encoding/json"
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
	if err := ops.WithAttrs(rootCtx,
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
