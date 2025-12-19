package telemetry

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/trace"
)

type CorrelatedHandler struct {
	handler slog.Handler
}

func NewCorrelatedLogger(handler slog.Handler) *slog.Logger {
	return slog.New(&CorrelatedHandler{handler: handler})
}

// Handle processes log records and injects trace correlation data
func (h *CorrelatedHandler) Handle(ctx context.Context, record slog.Record) error {
	// Extract trace information from context
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		spanContext := span.SpanContext()
		if spanContext.IsValid() {
			// Add trace and span IDs to the log record
			record.AddAttrs(
				slog.String("trace_id", spanContext.TraceID().String()),
				slog.String("span_id", spanContext.SpanID().String()),
			)

			// Add trace flags if present
			if spanContext.TraceFlags().IsSampled() {
				record.AddAttrs(slog.Bool("trace_sampled", true))
			}
		}
	}

	return h.handler.Handle(ctx, record)
}

func (h *CorrelatedHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *CorrelatedHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &CorrelatedHandler{handler: h.handler.WithAttrs(attrs)}
}

func (h *CorrelatedHandler) WithGroup(name string) slog.Handler {
	return &CorrelatedHandler{handler: h.handler.WithGroup(name)}
}

// LogHTTPRequest logs HTTP request details with trace correlation
func (c *TelemetryClient) LogHTTPRequest(ctx context.Context, method, path string, statusCode int, duration time.Duration, args ...any) {
	allArgs := append([]any{
		"http_method", method,
		"http_path", path,
		"http_status_code", statusCode,
		"duration_ms", duration.Milliseconds(),
	}, args...)

	level := slog.LevelInfo
	if statusCode >= 400 {
		level = slog.LevelWarn
	}
	if statusCode >= 500 {
		level = slog.LevelError
	}

	c.Logger.Log(ctx, level, "HTTP request completed", allArgs...)
}
