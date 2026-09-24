package sloglogger

import (
	"context"
	"io"
	"log/slog"

	"graph-code-challenge/internal/logger"
	"graph-code-challenge/internal/requestid"
)

var _ logger.Logger = (*Logger)(nil)

type Logger struct {
	inner *slog.Logger
}

func New(output io.Writer, level slog.Leveler) *Logger {
	return &Logger{
		inner: slog.New(slog.NewJSONHandler(output, &slog.HandlerOptions{Level: level})),
	}
}

func (l *Logger) Debug(ctx context.Context, message string, attributes ...any) {
	l.log(ctx, slog.LevelDebug, message, attributes...)
}

func (l *Logger) Info(ctx context.Context, message string, attributes ...any) {
	l.log(ctx, slog.LevelInfo, message, attributes...)
}

func (l *Logger) Warn(ctx context.Context, message string, attributes ...any) {
	l.log(ctx, slog.LevelWarn, message, attributes...)
}

func (l *Logger) Error(ctx context.Context, message string, attributes ...any) {
	l.log(ctx, slog.LevelError, message, attributes...)
}

func (l *Logger) log(ctx context.Context, level slog.Level, message string, attributes ...any) {
	if id := requestid.FromContext(ctx); id != "" {
		attributes = append([]any{"request_id", id}, attributes...)
	}

	l.inner.Log(ctx, level, message, attributes...)
}
