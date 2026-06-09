package wide

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
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
