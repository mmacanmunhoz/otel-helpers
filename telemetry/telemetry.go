package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"time"

	otelconf "go.opentelemetry.io/contrib/otelconf/v0.3.0"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Config holds telemetry configuration options
type Config struct {
	ConfigPath     string            // Path to YAML config file
	ServiceName    string            // Service name override
	ServiceVersion string            // Service version
	Environment    string            // Environment (dev, staging, prod)
	Attributes     map[string]string // Additional resource attributes
}

// TelemetryClient provides easy access to OpenTelemetry functionality
type TelemetryClient struct {
	shutdown func(context.Context) error
	Tracer   trace.Tracer
	Meter    metric.Meter
	Logger   *slog.Logger
}

// Setup initializes OpenTelemetry with configuration file
func Setup(ctx context.Context, confPath string) (func(context.Context) error, error) {
	return SetupWithConfig(ctx, Config{ConfigPath: confPath})
}

// SetupWithConfig initializes OpenTelemetry with detailed configuration
func SetupWithConfig(ctx context.Context, config Config) (func(context.Context) error, error) {
	b, err := os.ReadFile(config.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Set environment variables for config substitution
	if config.ServiceName != "" {
		os.Setenv("SERVICE_NAME", config.ServiceName)
	}
	if config.ServiceVersion != "" {
		os.Setenv("SERVICE_VERSION", config.ServiceVersion)
	}
	if config.Environment != "" {
		os.Setenv("ENVIRONMENT", config.Environment)
	}

	// Additional attributes
	for key, value := range config.Attributes {
		os.Setenv(key, value)
	}

	b = []byte(os.ExpandEnv(string(b)))

	conf, err := otelconf.ParseYAML(b)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %w", err)
	}

	sdk, err := otelconf.NewSDK(otelconf.WithContext(ctx), otelconf.WithOpenTelemetryConfiguration(*conf))
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenTelemetry SDK: %w", err)
	}

	otel.SetTracerProvider(sdk.TracerProvider())
	otel.SetMeterProvider(sdk.MeterProvider())
	return sdk.Shutdown, nil
}

// NewClient creates a new telemetry client with common functionality
func NewClient(ctx context.Context, config Config) (*TelemetryClient, error) {
	shutdown, err := SetupWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	serviceName := config.ServiceName
	if serviceName == "" {
		serviceName = "unknown-service"
	}

	// Create logger with correlation support
	logger := NewCorrelatedLogger(slog.NewJSONHandler(os.Stdout, nil))

	return &TelemetryClient{
		shutdown: shutdown,
		Tracer:   otel.Tracer(serviceName),
		Meter:    otel.Meter(serviceName),
		Logger:   logger,
	}, nil
}

// Shutdown gracefully shuts down telemetry
func (c *TelemetryClient) Shutdown(ctx context.Context) error {
	return c.shutdown(ctx)
}

// HTTPMetrics provides common HTTP metrics
type HTTPMetrics struct {
	RequestsTotal   metric.Int64Counter
	RequestDuration metric.Float64Histogram
	ErrorsTotal     metric.Int64Counter
}

// NewHTTPMetrics creates standard HTTP metrics
func (c *TelemetryClient) NewHTTPMetrics() (*HTTPMetrics, error) {
	requestsTotal, err := c.Meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create requests counter: %w", err)
	}

	requestDuration, err := c.Meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("Duration of HTTP requests in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create duration histogram: %w", err)
	}

	errorsTotal, err := c.Meter.Int64Counter(
		"http_errors_total",
		metric.WithDescription("Total number of HTTP errors"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create errors counter: %w", err)
	}

	return &HTTPMetrics{
		RequestsTotal:   requestsTotal,
		RequestDuration: requestDuration,
		ErrorsTotal:     errorsTotal,
	}, nil
}

// RecordRequest records an HTTP request with standard attributes
func (m *HTTPMetrics) RecordRequest(ctx context.Context, method, endpoint, statusCode string, duration time.Duration) {
	attrs := metric.WithAttributes(
		attribute.String("method", method),
		attribute.String("endpoint", endpoint),
		attribute.String("status_code", statusCode),
	)

	m.RequestsTotal.Add(ctx, 1, attrs)
	m.RequestDuration.Record(ctx, duration.Seconds(), attrs)
}

// RecordError records an HTTP error with standard attributes
func (m *HTTPMetrics) RecordError(ctx context.Context, errorType, endpoint string) {
	m.ErrorsTotal.Add(ctx, 1, metric.WithAttributes(
		attribute.String("error_type", errorType),
		attribute.String("endpoint", endpoint),
	))
}

// RuntimeMetrics provides Go runtime metrics
func (c *TelemetryClient) RegisterRuntimeMetrics() error {
	_, err := c.Meter.Int64ObservableGauge(
		"go_goroutines",
		metric.WithDescription("Number of goroutines"),
		metric.WithInt64Callback(func(_ context.Context, observer metric.Int64Observer) error {
			observer.Observe(int64(runtime.NumGoroutine()))
			return nil
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to create goroutines gauge: %w", err)
	}

	_, err = c.Meter.Int64ObservableGauge(
		"go_memstats_heap_bytes",
		metric.WithDescription("Heap memory in bytes"),
		metric.WithInt64Callback(func(_ context.Context, observer metric.Int64Observer) error {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			observer.Observe(int64(m.HeapInuse))
			return nil
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to create heap gauge: %w", err)
	}

	return nil
}

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

// DebugWithTrace logs a debug message with trace correlation
func (c *TelemetryClient) InfoWithTrace(ctx context.Context, msg string, args ...any) {
	c.Logger.InfoContext(ctx, msg, args...)
}

// ErrorWithTrace logs an error message with trace correlation
func (c *TelemetryClient) LogError(ctx context.Context, err error, msg string, args ...any) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.RecordError(err)
	}

	allArgs := append([]any{"error", err.Error()}, args...)
	c.Logger.ErrorContext(ctx, msg, allArgs...)
} // LogHTTPRequest logs HTTP request details with trace correlation
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
