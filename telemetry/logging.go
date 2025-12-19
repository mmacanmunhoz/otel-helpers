package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// CorrelatedHandler wraps slog.Handler to inject trace information
type CorrelatedHandler struct {
	handler slog.Handler
}

// NewCorrelatedLogger creates a logger that automatically injects trace/span IDs
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

// Enabled reports whether the handler handles records at the given level
func (h *CorrelatedHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

// WithAttrs returns a new handler whose attributes consist of both the receiver's attributes and the arguments
func (h *CorrelatedHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &CorrelatedHandler{handler: h.handler.WithAttrs(attrs)}
}

// WithGroup returns a new handler with the given group appended to the receiver's existing groups
func (h *CorrelatedHandler) WithGroup(name string) slog.Handler {
	return &CorrelatedHandler{handler: h.handler.WithGroup(name)}
}

// InfoWithTrace logs an info message with trace correlation
func (c *TelemetryClient) InfoWithTrace(ctx context.Context, msg string, args ...any) {
	c.Logger.InfoContext(ctx, msg, args...)
}

// LogError logs an error and records it in the current span
func (c *TelemetryClient) LogError(ctx context.Context, err error, msg string, args ...any) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.RecordError(err)
	}

	allArgs := append([]any{"error", err.Error()}, args...)
	c.Logger.ErrorContext(ctx, msg, allArgs...)
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

// LogWithSpanAttributes logs a message and adds the same attributes to the current span
func (c *TelemetryClient) LogWithSpanAttributes(ctx context.Context, level slog.Level, msg string, attrs map[string]any) {
	span := trace.SpanFromContext(ctx)

	// Convert to slog args
	args := make([]any, 0, len(attrs)*2)
	spanAttrs := make([]attribute.KeyValue, 0, len(attrs))

	for key, value := range attrs {
		args = append(args, key, value)

		// Add to span based on type
		switch v := value.(type) {
		case string:
			spanAttrs = append(spanAttrs, attribute.String(key, v))
		case int:
			spanAttrs = append(spanAttrs, attribute.Int(key, v))
		case int64:
			spanAttrs = append(spanAttrs, attribute.Int64(key, v))
		case float64:
			spanAttrs = append(spanAttrs, attribute.Float64(key, v))
		case bool:
			spanAttrs = append(spanAttrs, attribute.Bool(key, v))
		default:
			spanAttrs = append(spanAttrs, attribute.String(key, fmt.Sprintf("%v", v)))
		}
	}

	if span.IsRecording() {
		span.SetAttributes(spanAttrs...)
	}

	c.Logger.Log(ctx, level, msg, args...)
}
