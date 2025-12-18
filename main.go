package main

import (
	"context"
	"fmt"
	"helpers/telemetry"
	"io/ioutil"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	ctx := context.Background()

	shutdown, err := telemetry.Setup(ctx, "otel-config.yaml")
	if err != nil {
		log.Fatalf("falha ao inicializar OTEL: %v", err)
	}
	defer shutdown(ctx)

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	tracer := otel.Tracer("serviceconfig12")
	meter := otel.Meter("serviceconfig12")

	requestCounter, requestDuration, externalCallsCounter, errorCounter := CreateMetrics(meter)

	http.HandleFunc("/soma", func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		ctx, span := tracer.Start(r.Context(), "SomaHandler")
		defer span.End()

		// Incrementar contador de requests
		requestCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("method", r.Method),
			attribute.String("endpoint", "/soma"),
		))

		aStr := r.URL.Query().Get("a")
		bStr := r.URL.Query().Get("b")

		a, err1 := strconv.ParseFloat(aStr, 64)
		b, err2 := strconv.ParseFloat(bStr, 64)
		if err1 != nil || err2 != nil {
			span.RecordError(fmt.Errorf("parâmetros inválidos"))
			logWithTrace(ctx, slog.LevelError, "parâmetros inválidos", "a", err1, "b", err2)

			// Incrementar contador de erros
			errorCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("error_type", "invalid_parameters"),
				attribute.String("endpoint", "/soma"),
			))

			// Registrar duração da request
			requestDuration.Record(ctx, time.Since(startTime).Seconds(), metric.WithAttributes(
				attribute.String("method", r.Method),
				attribute.String("endpoint", "/soma"),
				attribute.String("status", "400"),
			))

			http.Error(w, "Parâmetros inválidos. Use /soma?a=1&b=2", http.StatusBadRequest)
			return
		}

		span.SetAttributes(attribute.Float64("param.a", a), attribute.Float64("param.b", b))

		client := &http.Client{Timeout: 2 * time.Second}
		req, _ := http.NewRequest("GET", fmt.Sprintf("http://localhost:8082/calc?a=%f&b=%f", a, b), nil)

		otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

		// Incrementar contador de chamadas externas
		externalCallsCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("target_service", "calc-service"),
			attribute.String("endpoint", "/calc"),
		))

		resp, err := client.Do(req)
		if err != nil {
			span.RecordError(err)

			// Incrementar contador de erros
			errorCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("error_type", "external_service_error"),
				attribute.String("endpoint", "/soma"),
			))

			// Registrar duração da request
			requestDuration.Record(ctx, time.Since(startTime).Seconds(), metric.WithAttributes(
				attribute.String("method", r.Method),
				attribute.String("endpoint", "/soma"),
				attribute.String("status", "500"),
			))

			http.Error(w, "erro ao chamar serviço 2", http.StatusInternalServerError)
			logWithTrace(ctx, slog.LevelError, "erro ao chamar o serviço 2", "error", err)
			return
		}
		defer resp.Body.Close()

		body, _ := ioutil.ReadAll(resp.Body)
		logWithTrace(ctx, slog.LevelInfo, "chamada para o serviço 2 realizada com sucesso", "response", resp.Status)

		// Registrar duração da request bem-sucedida
		requestDuration.Record(ctx, time.Since(startTime).Seconds(), metric.WithAttributes(
			attribute.String("method", r.Method),
			attribute.String("endpoint", "/soma"),
			attribute.String("status", "200"),
		))

		fmt.Fprintf(w, "Resultado do serviço2: %s", body)
	})

	fmt.Println("Serviço 1 ouvindo em :8085")
	log.Fatal(http.ListenAndServe(":8085", nil))
}

func CreateMetrics(meter metric.Meter) (metric.Int64Counter, metric.Float64Histogram, metric.Int64Counter, metric.Int64Counter) {
	requestCounter, err := meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
		metric.WithUnit("1"),
	)
	if err != nil {
		log.Fatalf("falha ao criar contador de requests: %v", err)
	}

	requestDuration, err := meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("Duration of HTTP requests in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		log.Fatalf("falha ao criar histograma de duração: %v", err)
	}

	externalCallsCounter, err := meter.Int64Counter(
		"external_calls_total",
		metric.WithDescription("Total number of external service calls"),
		metric.WithUnit("1"),
	)
	if err != nil {
		log.Fatalf("falha ao criar contador de chamadas externas: %v", err)
	}

	errorCounter, err := meter.Int64Counter(
		"errors_total",
		metric.WithDescription("Total number of errors"),
		metric.WithUnit("1"),
	)
	if err != nil {
		log.Fatalf("falha ao criar contador de erros: %v", err)
	}
	return requestCounter, requestDuration, externalCallsCounter, errorCounter
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
