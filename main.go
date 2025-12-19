package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/mmacanmunhoz/otel-helpers/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
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

	// Set the correlated logger as default
	slog.SetDefault(client.Logger)

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
			paramErr := fmt.Errorf("parâmetros inválidos: a=%v, b=%v", err1, err2)
			client.LogError(ctx, paramErr, "Erro ao processar parâmetros da requisição", "param_a", aStr, "param_b", bStr)

			// Record error using library
			httpMetrics.RecordError(ctx, "invalid_parameters", "/soma")
			httpMetrics.RecordRequest(ctx, r.Method, "/soma", "400", time.Since(startTime))

			// Log HTTP error
			client.LogHTTPRequest(ctx, r.Method, "/soma", 400, time.Since(startTime), "error_type", "invalid_parameters")

			http.Error(w, "Parâmetros inválidos. Use /soma?a=1&b=2", http.StatusBadRequest)
			return
		}

		// Log request with span attributes
		client.LogWithSpanAttributes(ctx, slog.LevelInfo, "Processando requisição de soma", map[string]any{
			"param_a":  a,
			"param_b":  b,
			"endpoint": "/soma",
		})

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
			client.LogError(ctx, err, "Erro ao chamar serviço externo", "target_service", "calc-service", "endpoint", "/calc")

			// Record error using library
			httpMetrics.RecordError(ctx, "external_service_error", "/soma")
			httpMetrics.RecordRequest(ctx, r.Method, "/soma", "500", time.Since(startTime))

			// Log HTTP error
			client.LogHTTPRequest(ctx, r.Method, "/soma", 500, time.Since(startTime), "error_type", "external_service_error")

			http.Error(w, "erro ao chamar serviço 2", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		body, _ := ioutil.ReadAll(resp.Body)
		client.InfoWithTrace(ctx, "Chamada para serviço externo realizada com sucesso",
			"response_status", resp.Status,
			"target_service", "calc-service",
			"response_body", string(body))

		// Record successful request using library
		httpMetrics.RecordRequest(ctx, r.Method, "/soma", "200", time.Since(startTime))

		// Log HTTP request completion
		client.LogHTTPRequest(ctx, r.Method, "/soma", 200, time.Since(startTime), "response_body", string(body))

		fmt.Fprintf(w, "Resultado do serviço2: %s", body)
	})

	client.InfoWithTrace(context.Background(), "Serviço iniciado", "port", ":8085")
	log.Fatal(http.ListenAndServe(":8085", nil))
}
