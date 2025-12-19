package telemetry

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

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

// RegisterRuntimeMetrics provides Go runtime metrics
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
