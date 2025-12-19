package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	otelconf "go.opentelemetry.io/contrib/otelconf/v0.3.0"
	"go.opentelemetry.io/otel"
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
