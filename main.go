package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/mmacanmunhoz/otel-helpers/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	ctx := context.Background()

	// Initialize telemetry using the new library API
	client, err := telemetry.NewClient(ctx, telemetry.Config{
		ConfigPath:     "otel-config.yaml",
		ServiceName:    "serviceconfig12",
		ServiceVersion: "1.0.0",
		Environment:    "prod",
		Attributes: map[string]string{
			"TEAM":   "backend",
			"REGION": "local",
		},
	})
	if err != nil {
		log.Fatalf("falha ao inicializar OTEL: %v", err)
	}
	defer client.Shutdown(ctx)

	// Register runtime metrics (optional)
	if err := client.RegisterRuntimeMetrics(); err != nil {
		log.Printf("Falha ao registrar métricas de runtime: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Create HTTP metrics using the library
	httpMetrics, err := client.NewHTTPMetrics()
	if err != nil {
		log.Fatalf("falha ao criar métricas HTTP: %v", err)
	}

	// Create additional metrics for external calls
	externalCallsCounter, err := client.Meter.Int64Counter(
		"external_calls_total",
		metric.WithDescription("Total number of external service calls"),
		metric.WithUnit("1"),
	)
	if err != nil {
		log.Fatalf("falha ao criar contador de chamadas externas: %v", err)
	}

	http.HandleFunc("/soma", func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		ctx, span := client.Tracer.Start(r.Context(), "SomaHandler")
		defer span.End()

		aStr := r.URL.Query().Get("a")
		bStr := r.URL.Query().Get("b")

		a, err1 := strconv.ParseFloat(aStr, 64)
		b, err2 := strconv.ParseFloat(bStr, 64)
		if err1 != nil || err2 != nil {
			span.RecordError(fmt.Errorf("parâmetros inválidos"))
			logWithTrace(ctx, slog.LevelError, "parâmetros inválidos", "a", err1, "b", err2)
			
			// Record error using library
			httpMetrics.RecordError(ctx, "invalid_parameters", "/soma")
			httpMetrics.RecordRequest(ctx, r.Method, "/soma", "400", time.Since(startTime))
			
			http.Error(w, "Parâmetros inválidos. Use /soma?a=1&b=2", http.StatusBadRequest)
			return
		}

		span.SetAttributes(attribute.Float64("param.a", a), attribute.Float64("param.b", b))

		client_http := &http.Client{Timeout: 2 * time.Second}
		req, _ := http.NewRequest("GET", fmt.Sprintf("http://localhost:8082/calc?a=%f&b=%f", a, b), nil)

		otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

		// Incrementar contador de chamadas externas
		externalCallsCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("target_service", "calc-service"),
			attribute.String("endpoint", "/calc"),
		))

		resp, err := client_http.Do(req)
		if err != nil {
			span.RecordError(err)
			
			// Record error using library
			httpMetrics.RecordError(ctx, "external_service_error", "/soma")
			httpMetrics.RecordRequest(ctx, r.Method, "/soma", "500", time.Since(startTime))
			
			http.Error(w, "erro ao chamar serviço 2", http.StatusInternalServerError)
			logWithTrace(ctx, slog.LevelError, "erro ao chamar o serviço 2", "error", err)
			return
		}
		defer resp.Body.Close()

		body, _ := ioutil.ReadAll(resp.Body)
		logWithTrace(ctx, slog.LevelInfo, "chamada para o serviço 2 realizada com sucesso", "response", resp.Status)

		// Record successful request using library
		httpMetrics.RecordRequest(ctx, r.Method, "/soma", "200", time.Since(startTime))

		fmt.Fprintf(w, "Resultado do serviço2: %s", body)
	})

	fmt.Println("Serviço 1 ouvindo em :8085")
	log.Fatal(http.ListenAndServe(":8085", nil))
}

func logWithTrace(ctx context.Context, level slog.Level, msg string, args ...any) {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		args = append(args,
			"trace_id", span.SpanContext().TraceID().String(),
			"span_id", span.SpanContext().SpanID().String(),
		)
	}
	slog.Log(ctx, level, msg, args...)
}