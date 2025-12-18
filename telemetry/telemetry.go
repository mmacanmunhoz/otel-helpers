package telemetry

import (
	"context"
	"fmt"
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

	return &TelemetryClient{
		shutdown: shutdown,
		Tracer:   otel.Tracer(serviceName),
		Meter:    otel.Meter(serviceName),
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
