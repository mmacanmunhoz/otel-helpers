# OpenTelemetry Go Library

Uma biblioteca Go simples e poderosa para integração com OpenTelemetry usando configuração YAML declarativa.

## 🚀 Características

- ✅ **Configuração YAML declarativa** - Configure traces, métricas e logs via arquivo
- ✅ **API simples** - Poucos métodos para máxima produtividade  
- ✅ **Métricas HTTP prontas** - Contadores, histogramas e middleware incluídos
- ✅ **Métricas de runtime** - CPU, memória, goroutines automáticas
- ✅ **Backward compatible** - Funciona com código existente
- ✅ **Configuração flexível** - Environment variables e atributos customizados

## 📦 Instalação

```bash
go get github.com/seu-usuario/otel-helpers/telemetry
```

## 🎯 Uso Rápido

### 1. Uso Simples (Compatível com código existente)

```go
import "github.com/seu-usuario/otel-helpers/telemetry"

func main() {
    ctx := context.Background()
    
    // Setup simples
    shutdown, err := telemetry.Setup(ctx, "otel-config.yaml")
    if err != nil {
        log.Fatal(err)
    }
    defer shutdown(ctx)
    
    // Use OpenTelemetry normalmente
    tracer := otel.Tracer("my-service")
    meter := otel.Meter("my-service")
}
```

### 2. Uso Avançado (Cliente completo)

```go
import "github.com/seu-usuario/otel-helpers/telemetry"

func main() {
    ctx := context.Background()
    
    // Configuração avançada
    client, err := telemetry.NewClient(ctx, telemetry.Config{
        ConfigPath:     "otel-config.yaml",
        ServiceName:    "user-service",
        ServiceVersion: "1.2.3",
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
    
    // Registrar métricas de runtime (opcional)
    client.RegisterRuntimeMetrics()
    
    // Criar métricas HTTP
    httpMetrics, _ := client.NewHTTPMetrics()
    
    // Usar em handlers
    http.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
        startTime := time.Now()
        ctx, span := client.Tracer.Start(r.Context(), "GetUsers")
        defer span.End()
        
        // Sua lógica aqui
        processUsers()
        
        // Registrar métricas
        httpMetrics.RecordRequest(ctx, r.Method, "/api/users", "200", time.Since(startTime))
    })
}
```

### 3. Middleware HTTP (Automático)

```go
func main() {
    client, _ := telemetry.NewClient(ctx, config)
    httpMetrics, _ := client.NewHTTPMetrics()
    
    // Middleware que instrumenta automaticamente
    middleware := client.HTTPMiddleware(httpMetrics)
    
    mux := http.NewServeMux()
    mux.HandleFunc("/api/users", getUsersHandler)
    
    // Aplica instrumentação automaticamente
    http.ListenAndServe(":8080", middleware(mux))
}
```

## 📊 Métricas Incluídas

### HTTP Metrics
- `http_requests_total` - Contador de requests
- `http_request_duration_seconds` - Histograma de latência  
- `http_errors_total` - Contador de erros

### Runtime Metrics (opcional)
- `go_goroutines` - Número de goroutines
- `go_memstats_heap_bytes` - Uso de memória heap

### Atributos Padrão
- `method` - Método HTTP (GET, POST, etc.)
- `endpoint` - Endpoint acessado
- `status_code` - Status code da resposta
- `error_type` - Tipo de erro (client_error, server_error)

## ⚙️ Configuração

### Arquivo otel-config.yaml

```yaml
file_format: "0.3"
resource:
  attributes:
    - name: service.name
      value: ${SERVICE_NAME:-my-service}
    - name: service.version  
      value: ${SERVICE_VERSION:-1.0.0}
    - name: environment
      value: ${ENVIRONMENT:-development}

tracer_provider:
  processors:
    - batch:
        exporter:
          otlp:
            protocol: http/protobuf
            endpoint: ${OTEL_ENDPOINT:-http://localhost:4318}

meter_provider:
  readers:
    - periodic:
        interval: 5000
        exporter:
          otlp:
            protocol: http/protobuf
            endpoint: ${OTEL_ENDPOINT:-http://localhost:4318}
        cardinality_limits:
          default: 2000
          counter: 5000
          histogram: 1000
  views:
    - selector:
        instrument_name: "http_request_duration_seconds"
      stream:
        aggregation:
          explicit_bucket_histogram:
            boundaries: [0.001, 0.01, 0.1, 0.5, 1.0, 2.0, 5.0, 10.0]
```

### Variáveis de Ambiente Suportadas

- `SERVICE_NAME` - Nome do serviço
- `SERVICE_VERSION` - Versão do serviço  
- `ENVIRONMENT` - Ambiente (dev, staging, prod)
- `OTEL_ENDPOINT` - Endpoint do coletor OpenTelemetry
- Qualquer variável personalizada definida em `Config.Attributes`

## 🎛️ API Reference

### telemetry.Config

```go
type Config struct {
    ConfigPath     string            // Caminho para arquivo YAML
    ServiceName    string            // Nome do serviço
    ServiceVersion string            // Versão do serviço
    Environment    string            // Ambiente
    Attributes     map[string]string // Atributos adicionais
}
```

### telemetry.TelemetryClient

```go
type TelemetryClient struct {
    Tracer trace.Tracer  // Tracer OpenTelemetry
    Meter  metric.Meter  // Meter OpenTelemetry
}

// Métodos
func NewClient(ctx context.Context, config Config) (*TelemetryClient, error)
func (c *TelemetryClient) Shutdown(ctx context.Context) error
func (c *TelemetryClient) NewHTTPMetrics() (*HTTPMetrics, error)
func (c *TelemetryClient) RegisterRuntimeMetrics() error
func (c *TelemetryClient) HTTPMiddleware(httpMetrics *HTTPMetrics) func(http.Handler) http.Handler
```

### telemetry.HTTPMetrics

```go
type HTTPMetrics struct {
    RequestsTotal   metric.Int64Counter
    RequestDuration metric.Float64Histogram  
    ErrorsTotal     metric.Int64Counter
}

// Métodos
func (m *HTTPMetrics) RecordRequest(ctx context.Context, method, endpoint, statusCode string, duration time.Duration)
func (m *HTTPMetrics) RecordError(ctx context.Context, errorType, endpoint string)
```

## 🔧 Exemplo Completo

Ver [example.go](./example.go) para um exemplo completo de uso.

## 📈 Compatibilidade

- ✅ Go 1.21+
- ✅ OpenTelemetry Go SDK v1.37.0+
- ✅ otelconf v0.17.0+

## 🤝 Contribuição

1. Fork o projeto
2. Crie uma branch (`git checkout -b feature/amazing`)
3. Commit suas mudanças (`git commit -am 'Add amazing feature'`)
4. Push para a branch (`git push origin feature/amazing`)  
5. Abra um Pull Request

## 📄 Licença

MIT License - veja [LICENSE](../LICENSE) para detalhes.