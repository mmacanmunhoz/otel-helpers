package telemetry

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

// Example shows how to use the telemetry library
func ExampleUsage() {
	ctx := context.Background()

	// Method 1: Simple setup (backward compatible)
	shutdown, err := Setup(ctx, "otel-config.yaml")
	if err != nil {
		log.Fatal(err)
	}
	defer shutdown(ctx)

	// Method 2: Advanced setup with configuration
	client, err := NewClient(ctx, Config{
		ConfigPath:     "otel-config.yaml",
		ServiceName:    "my-awesome-service",
		ServiceVersion: "1.0.0",
		Environment:    "production",
		Attributes: map[string]string{
			"TEAM":   "backend",
			"REGION": "us-east-1",
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Shutdown(ctx)

	// Register runtime metrics
	if err := client.RegisterRuntimeMetrics(); err != nil {
		log.Printf("Failed to register runtime metrics: %v", err)
	}

	// Create HTTP metrics
	httpMetrics, err := client.NewHTTPMetrics()
	if err != nil {
		log.Fatal(err)
	}

	// HTTP handler example
	http.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		// Start tracing
		ctx, span := client.Tracer.Start(r.Context(), "GetUsers")
		defer span.End()

		// Add span attributes
		span.SetAttributes(
			attribute.String("user.id", r.Header.Get("User-ID")),
			attribute.String("request.path", r.URL.Path),
		)

		// Simulate processing
		processUsers(ctx, client)

		// Record metrics
		statusCode := "200"
		httpMetrics.RecordRequest(ctx, r.Method, "/api/users", statusCode, time.Since(startTime))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Users retrieved successfully"))
	})
}

func processUsers(ctx context.Context, client *TelemetryClient) {
	// Child span example
	_, span := client.Tracer.Start(ctx, "DatabaseQuery")
	defer span.End()

	// Simulate work
	time.Sleep(50 * time.Millisecond)

	span.SetAttributes(attribute.Int("users.count", 42))
}

// HTTPMiddleware provides tracing and metrics for HTTP handlers
func (c *TelemetryClient) HTTPMiddleware(httpMetrics *HTTPMetrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()

			// Start span
			ctx, span := c.Tracer.Start(r.Context(), r.URL.Path)
			defer span.End()

			// Add basic attributes
			span.SetAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.url", r.URL.String()),
				attribute.String("http.user_agent", r.UserAgent()),
			)

			// Wrap response writer to capture status code
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Execute handler
			next.ServeHTTP(wrapped, r.WithContext(ctx))

			// Record metrics
			statusCode := strconv.Itoa(wrapped.statusCode)
			httpMetrics.RecordRequest(ctx, r.Method, r.URL.Path, statusCode, time.Since(startTime))

			// Record error if status >= 400
			if wrapped.statusCode >= 400 {
				errorType := "client_error"
				if wrapped.statusCode >= 500 {
					errorType = "server_error"
				}
				httpMetrics.RecordError(ctx, errorType, r.URL.Path)
			}

			// Set span status
			span.SetAttributes(attribute.Int("http.status_code", wrapped.statusCode))
			if wrapped.statusCode >= 400 {
				span.RecordError(nil)
			}
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
