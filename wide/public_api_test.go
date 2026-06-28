/*
Package wide_test provides tests for the public API of the wide package.

It is intnentially not part of the "wide" package so it can only see the public API.
*/
package wide_test

import (
	"log/slog"
	"reflect"
	"testing"

	"github.com/druejaramillo/go-wide/wide"
)

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

func attrsToTree(attrs []slog.Attr) map[string]any {
	out := map[string]any{}
	for _, attr := range attrs {
		out[attr.Key] = attrValueTree(attr.Value)
	}
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
