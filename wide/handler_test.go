package wide

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
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
