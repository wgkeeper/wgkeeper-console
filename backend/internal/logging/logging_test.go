package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestRequestIDIsLogged(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, "json", slog.LevelInfo).With("component", "test")

	ctx := WithRequestID(context.Background(), "3f7a2c1e-8b4d-4e6f-9a0b-1c2d3e4f5a6b")
	logger.InfoContext(ctx, "with id")
	logger.Info("without id")

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2: %s", len(lines), buf.String())
	}
	var first, second map[string]any
	if err := json.Unmarshal(lines[0], &first); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(lines[1], &second); err != nil {
		t.Fatal(err)
	}
	if first["request_id"] != "3f7a2c1e-8b4d-4e6f-9a0b-1c2d3e4f5a6b" || first["component"] != "test" {
		t.Fatalf("first line = %v, want request_id and the logger's attributes", first)
	}
	if _, ok := second["request_id"]; ok {
		t.Fatalf("a record without a request context got a request_id: %v", second)
	}
}

func TestRequestIDFromEmptyContext(t *testing.T) {
	if got := RequestID(context.Background()); got != "" {
		t.Fatalf("RequestID() = %q, want empty", got)
	}
}
