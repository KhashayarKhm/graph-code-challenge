package logger

import "context"

type Logger interface {
	Debug(ctx context.Context, message string, attributes ...any)
	Info(ctx context.Context, message string, attributes ...any)
	Warn(ctx context.Context, message string, attributes ...any)
	Error(ctx context.Context, message string, attributes ...any)
}
