package sloglogger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"graph-code-challenge/internal/logger/sloglogger"
	"graph-code-challenge/internal/requestid"
)

func TestLoggerWritesStructuredAttributesAndRequestID(t *testing.T) {
	var output bytes.Buffer
	log := sloglogger.New(&output, slog.LevelDebug)
	ctx := requestid.NewContext(context.Background(), "req-42")

	log.Info(ctx, "task created", "task_id", 7)

	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("decode log entry: %v", err)
	}

	for key, want := range map[string]any{
		"level":      "INFO",
		"msg":        "task created",
		"request_id": "req-42",
		"task_id":    float64(7),
	} {
		if got := entry[key]; got != want {
			t.Errorf("%s = %v, want %v", key, got, want)
		}
	}
}

func TestLoggerHonorsLevel(t *testing.T) {
	var output bytes.Buffer
	log := sloglogger.New(&output, slog.LevelInfo)

	log.Debug(context.Background(), "hidden")
	log.Info(context.Background(), "visible")

	if got := bytes.Count(output.Bytes(), []byte("\n")); got != 1 {
		t.Fatalf("log entries = %d, want 1: %s", got, output.String())
	}
}
