/*
Package wide_test provides tests for the public API of the wide package.

It is intnentially not part of the "wide" package so it can only see the public API.
*/
package wide_test

import (
	"context"
	"log/slog"
	"reflect"
	"testing"
	"time"

	"github.com/druejaramillo/go-wide/ops"
	"github.com/druejaramillo/go-wide/wide"
)

var _ wide.Strategy = (*captureStrategy)(nil)

func TestNormalizeLogsAllowsExternalPackagesToReuseWideEventLogShaping(t *testing.T) {
	logs := attrsToTree(wide.NormalizeLogs([]wide.LogEntry{
		{
			Level:   slog.LevelInfo,
			Message: "retrying charge",
			Attrs: []slog.Attr{
				slog.String("component", "billing"),
				slog.String("phase", "charge"),
				slog.Int("attempt", 1),
			},
		},
		{
			Level:   slog.LevelInfo,
			Message: "retrying charge",
			Attrs: []slog.Attr{
				slog.String("component", "billing"),
				slog.String("phase", "charge"),
				slog.Int("attempt", 2),
			},
		},
	}))

	if len(logs) != 1 {
		t.Fatalf("normalized bucket count = %d, want 1; logs = %v", len(logs), logs)
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

func TestNormalizeSnapshotAllowsExternalPackagesToReuseWideEventOperationShaping(t *testing.T) {
	attrs := attrsToTree(wide.NormalizeSnapshot(wide.OperationSnapshot{
		Name: "root",
		Attrs: []slog.Attr{
			slog.String("request_id", "req-123"),
		},
		Children: []wide.OperationSnapshot{
			{
				Name:   "charge",
				Status: "error",
				Error:  "context deadline exceeded",
				Attrs: []slog.Attr{
					slog.String("provider", "stripe"),
				},
				Logs: []wide.LogEntry{
					{
						Level:   slog.LevelInfo,
						Message: "retrying charge",
						Attrs: []slog.Attr{
							slog.String("phase", "charge"),
							slog.Int("attempt", 1),
						},
					},
				},
			},
		},
	}))

	if attrs["request_id"] != "req-123" {
		t.Fatalf("request_id = %v, want %q", attrs["request_id"], "req-123")
	}

	child, ok := attrs["charge"].(map[string]any)
	if !ok {
		t.Fatalf("charge = %T, want map[string]any", attrs["charge"])
	}

	if child["provider"] != "stripe" {
		t.Fatalf("charge.provider = %v, want %q", child["provider"], "stripe")
	}

	if child["status"] != "error" {
		t.Fatalf("charge.status = %v, want %q", child["status"], "error")
	}

	if child["error"] != "context deadline exceeded" {
		t.Fatalf("charge.error = %v, want %q", child["error"], "context deadline exceeded")
	}

	logs, ok := child["logs"].(map[string]any)
	if !ok {
		t.Fatalf("charge.logs = %T, want map[string]any", child["logs"])
	}

	bucket, ok := logs["retrying charge"].(map[string]any)
	if !ok {
		t.Fatalf("charge.logs[%q] = %T, want map[string]any; logs = %v", "retrying charge", logs["retrying charge"], logs)
	}

	if bucket["message"] != "retrying charge" {
		t.Fatalf("bucket.message = %v, want %q", bucket["message"], "retrying charge")
	}

	if bucket["level"] != "INFO" {
		t.Fatalf("bucket.level = %v, want %q", bucket["level"], "INFO")
	}

	if bucket["count"] != int64(1) {
		t.Fatalf("bucket.count = %v, want %v", bucket["count"], 1)
	}

	if bucket["phase"] != "charge" {
		t.Fatalf("bucket.phase = %v, want %q", bucket["phase"], "charge")
	}

	if bucket["attempt"] != int64(1) {
		t.Fatalf("bucket.attempt = %v, want %v", bucket["attempt"], 1)
	}

	if _, ok := bucket["variants"]; ok {
		t.Fatalf("bucket.variants should be absent for a single log entry, bucket = %v", bucket)
	}
}

func TestNormalizeSnapshotMergesSameNamedChildrenForExternalStrategies(t *testing.T) {
	attrs := attrsToTree(wide.NormalizeSnapshot(wide.OperationSnapshot{
		Name: "root",
		Children: []wide.OperationSnapshot{
			{
				Name:   "charge",
				Status: "ok",
				Attrs: []slog.Attr{
					slog.String("provider", "stripe"),
					slog.String("phase", "authorize"),
				},
			},
			{
				Name:   "charge",
				Status: "error",
				Error:  context.DeadlineExceeded.Error(),
				Attrs: []slog.Attr{
					slog.String("provider", "stripe"),
					slog.String("phase", "capture"),
				},
			},
		},
	}))

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

	phases, ok := variants["phase"].([]any)
	if !ok {
		t.Fatalf("charge.variants.phase = %T, want []any", variants["phase"])
	}

	if !reflect.DeepEqual(phases, []any{"authorize", "capture"}) {
		t.Fatalf("charge.variants.phase = %v, want %v", phases, []any{"authorize", "capture"})
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

func TestCustomStrategyCanConsumePublicSurfaceWithoutInternalImports(t *testing.T) {
	downstream := &publicRecordingHandler{}
	strategy := &captureStrategy{}
	handler := wide.NewHandler(downstream, wide.WithStrategy(strategy))

	rootCtx, err := ops.StartRoot(context.Background(), "root", handler.RootOption())
	if err != nil {
		t.Fatalf("StartRoot() error = %v", err)
	}

	if err := ops.WithAttrs(rootCtx, slog.String("request_id", "req-123")); err != nil {
		t.Fatalf("WithAttrs(root) error = %v", err)
	}

	childCtx, err := ops.Start(rootCtx, "charge")
	if err != nil {
		t.Fatalf("Start(child) error = %v", err)
	}

	if err := ops.WithAttrs(childCtx, slog.String("provider", "stripe")); err != nil {
		t.Fatalf("WithAttrs(child) error = %v", err)
	}

	slog.New(handler).InfoContext(childCtx, "retrying charge", slog.Int("attempt", 1))

	childCtx, err = ops.Error(childCtx, context.DeadlineExceeded)
	if err != nil {
		t.Fatalf("Error(child) error = %v", err)
	}

	if _, err := ops.End(childCtx); err != nil {
		t.Fatalf("End(child) error = %v", err)
	}

	if _, err := ops.End(rootCtx); err != nil {
		t.Fatalf("End(root) error = %v", err)
	}

	if len(strategy.collected) != 1 {
		t.Fatalf("collect count = %d, want 1", len(strategy.collected))
	}

	entry := strategy.collected[0]
	if entry.Message != "retrying charge" {
		t.Fatalf("collected message = %q, want %q", entry.Message, "retrying charge")
	}

	entryAttrs := attrsToFlatMap(entry.Attrs)
	if entryAttrs["attempt"] != int64(1) {
		t.Fatalf("collected attempt = %v, want %v", entryAttrs["attempt"], 1)
	}

	if len(strategy.flushed) != 1 {
		t.Fatalf("flush count = %d, want 1", len(strategy.flushed))
	}

	snapshot := strategy.flushed[0]
	if snapshot.Name != "root" {
		t.Fatalf("snapshot.Name = %q, want %q", snapshot.Name, "root")
	}

	rootAttrs := attrsToFlatMap(snapshot.Attrs)
	if rootAttrs["request_id"] != "req-123" {
		t.Fatalf("snapshot request_id = %v, want %q", rootAttrs["request_id"], "req-123")
	}

	if len(snapshot.Children) != 1 {
		t.Fatalf("snapshot child count = %d, want 1", len(snapshot.Children))
	}

	child := snapshot.Children[0]
	if child.Name != "charge" {
		t.Fatalf("child.Name = %q, want %q", child.Name, "charge")
	}
	if child.Status != "error" {
		t.Fatalf("child.Status = %q, want %q", child.Status, "error")
	}
	if child.Error != context.DeadlineExceeded.Error() {
		t.Fatalf("child.Error = %q, want %q", child.Error, context.DeadlineExceeded.Error())
	}

	childAttrs := attrsToFlatMap(child.Attrs)
	if childAttrs["provider"] != "stripe" {
		t.Fatalf("child provider = %v, want %q", childAttrs["provider"], "stripe")
	}

	if len(child.Logs) != 1 {
		t.Fatalf("child log count = %d, want 1", len(child.Logs))
	}
	if child.Logs[0].Message != "retrying charge" {
		t.Fatalf("child log message = %q, want %q", child.Logs[0].Message, "retrying charge")
	}

	records := downstream.Records()
	if len(records) != 1 {
		t.Fatalf("record count = %d, want 1", len(records))
	}

	if records[0].Message != "custom-wide" {
		t.Fatalf("record message = %q, want %q", records[0].Message, "custom-wide")
	}

	recordAttrs := recordToTree(records[0])
	if recordAttrs["collected_logs"] != int64(1) {
		t.Fatalf("record collected_logs = %v, want %v", recordAttrs["collected_logs"], 1)
	}

	snapshotTree, ok := recordAttrs["snapshot"].(map[string]any)
	if !ok {
		t.Fatalf("record snapshot = %T, want map[string]any", recordAttrs["snapshot"])
	}

	if snapshotTree["request_id"] != "req-123" {
		t.Fatalf("record snapshot.request_id = %v, want %q", snapshotTree["request_id"], "req-123")
	}

	chargeTree, ok := snapshotTree["charge"].(map[string]any)
	if !ok {
		t.Fatalf("record snapshot.charge = %T, want map[string]any", snapshotTree["charge"])
	}

	if chargeTree["status"] != "error" {
		t.Fatalf("record snapshot.charge.status = %v, want %q", chargeTree["status"], "error")
	}
	if chargeTree["error"] != context.DeadlineExceeded.Error() {
		t.Fatalf("record snapshot.charge.error = %v, want %q", chargeTree["error"], context.DeadlineExceeded.Error())
	}
}

func TestCustomStrategyFlushesRawSnapshotsWithoutMergingSameNamedChildren(t *testing.T) {
	strategy := &captureStrategy{}
	handler := wide.NewHandler(&publicRecordingHandler{}, wide.WithStrategy(strategy))

	rootCtx, err := ops.StartRoot(context.Background(), "root", handler.RootOption())
	if err != nil {
		t.Fatalf("StartRoot() error = %v", err)
	}

	firstChildCtx, err := ops.Start(rootCtx, "charge")
	if err != nil {
		t.Fatalf("Start(first child) error = %v", err)
	}
	if err := ops.WithAttrs(firstChildCtx, slog.String("phase", "authorize")); err != nil {
		t.Fatalf("WithAttrs(first child) error = %v", err)
	}
	if _, err := ops.End(firstChildCtx); err != nil {
		t.Fatalf("End(first child) error = %v", err)
	}

	secondChildCtx, err := ops.Start(rootCtx, "charge")
	if err != nil {
		t.Fatalf("Start(second child) error = %v", err)
	}
	if err := ops.WithAttrs(secondChildCtx, slog.String("phase", "capture")); err != nil {
		t.Fatalf("WithAttrs(second child) error = %v", err)
	}
	if _, err := ops.End(secondChildCtx); err != nil {
		t.Fatalf("End(second child) error = %v", err)
	}

	if _, err := ops.End(rootCtx); err != nil {
		t.Fatalf("End(root) error = %v", err)
	}

	if len(strategy.flushed) != 1 {
		t.Fatalf("flush count = %d, want 1", len(strategy.flushed))
	}

	snapshot := strategy.flushed[0]
	if len(snapshot.Children) != 2 {
		t.Fatalf("snapshot child count = %d, want 2 separate raw child snapshots", len(snapshot.Children))
	}

	first := snapshot.Children[0]
	second := snapshot.Children[1]

	if first.Name != "charge" {
		t.Fatalf("first child name = %q, want %q", first.Name, "charge")
	}
	if second.Name != "charge" {
		t.Fatalf("second child name = %q, want %q", second.Name, "charge")
	}

	firstAttrs := attrsToFlatMap(first.Attrs)
	secondAttrs := attrsToFlatMap(second.Attrs)

	if firstAttrs["phase"] != "authorize" {
		t.Fatalf("first child phase = %v, want %q", firstAttrs["phase"], "authorize")
	}
	if secondAttrs["phase"] != "capture" {
		t.Fatalf("second child phase = %v, want %q", secondAttrs["phase"], "capture")
	}

	if _, ok := firstAttrs["count"]; ok {
		t.Fatalf("first child count should be absent from raw snapshot attrs: %v", firstAttrs)
	}
	if _, ok := secondAttrs["count"]; ok {
		t.Fatalf("second child count should be absent from raw snapshot attrs: %v", secondAttrs)
	}
	if _, ok := firstAttrs["variants"]; ok {
		t.Fatalf("first child variants should be absent from raw snapshot attrs: %v", firstAttrs)
	}
	if _, ok := secondAttrs["variants"]; ok {
		t.Fatalf("second child variants should be absent from raw snapshot attrs: %v", secondAttrs)
	}
}

func attrsToTree(attrs []slog.Attr) map[string]any {
	out := map[string]any{}
	for _, attr := range attrs {
		out[attr.Key] = attrValueTree(attr.Value)
	}
	return out
}

func attrsToFlatMap(attrs []slog.Attr) map[string]any {
	out := map[string]any{}
	for _, attr := range attrs {
		out[attr.Key] = attr.Value.Resolve().Any()
	}
	return out
}

type captureStrategy struct {
	collected []wide.LogEntry
	flushed   []wide.OperationSnapshot
}

func (s *captureStrategy) Collect(entry wide.LogEntry) {
	s.collected = append(s.collected, entry)
}

func (s *captureStrategy) Flush(snapshot wide.OperationSnapshot) slog.Record {
	s.flushed = append(s.flushed, snapshot)
	record := slog.NewRecord(time.Unix(0, 0), slog.LevelInfo, "custom-wide", 0)
	record.AddAttrs(
		slog.Int("collected_logs", len(s.collected)),
		slog.GroupAttrs("snapshot", wide.NormalizeSnapshot(snapshot)...),
	)
	return record
}

type publicRecordingHandler struct {
	records []slog.Record
}

func (h *publicRecordingHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *publicRecordingHandler) Handle(_ context.Context, record slog.Record) error {
	h.records = append(h.records, record.Clone())
	return nil
}

func (h *publicRecordingHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *publicRecordingHandler) WithGroup(string) slog.Handler {
	return h
}

func (h *publicRecordingHandler) Records() []slog.Record {
	return append([]slog.Record(nil), h.records...)
}

func recordToTree(record slog.Record) map[string]any {
	out := map[string]any{}
	record.Attrs(func(attr slog.Attr) bool {
		out[attr.Key] = attrValueTree(attr.Value)
		return true
	})
	return out
}

func attrValueTree(value slog.Value) any {
	value = value.Resolve()
	if value.Kind() != slog.KindGroup {
		return value.Any()
	}

	out := map[string]any{}
	for _, attr := range value.Group() {
		out[attr.Key] = attrValueTree(attr.Value)
	}
	return out
}
