# 📝 Log Correlation com OpenTelemetry

## ✨ **O que foi implementado?**

Sistema de correlação automática de logs que injeta `trace_id` e `span_id` em todos os logs, permitindo rastrear logs através de toda a cadeia de requisições distribuídas.

## 🚀 **Funcionalidades**

### ✅ **1. Logger Correlacionado Automático**
- Injeta automaticamente `trace_id`, `span_id` e `trace_sampled` nos logs
- Funciona com qualquer contexto que contenha span ativo
- Zero configuração adicional necessária

### ✅ **2. Métodos Convenientes para Logs**
```go
// Logs básicos com correlação automática
client.DebugWithTrace(ctx, "Debug message", "key", "value")
client.InfoWithTrace(ctx, "Info message", "key", "value")
client.WarnWithTrace(ctx, "Warning message", "key", "value")
client.ErrorWithTrace(ctx, "Error message", "key", "value")

// Log de erro com registro no span
client.LogError(ctx, err, "Descrição do erro", "extra", "data")

// Log de requisições HTTP estruturado
client.LogHTTPRequest(ctx, "GET", "/api/users", 200, duration)

// Log com atributos que vão para span e log
client.LogWithSpanAttributes(ctx, slog.LevelInfo, "Processando", map[string]any{
    "user_id": 123,
    "action": "create_order",
})
```

## 📋 **Como Usar**

### **1. Configuração (automática)**
```go
client, err := telemetry.NewClient(ctx, telemetry.Config{
    ConfigPath:  "otel-config.yaml",
    ServiceName: "meu-servico",
})
// Logger correlacionado já está configurado em client.Logger
```

### **2. Logs Simples com Correlação**
```go
func handler(w http.ResponseWriter, r *http.Request) {
    ctx, span := client.Tracer.Start(r.Context(), "HandlerName")
    defer span.End()
    
    // Este log terá trace_id e span_id automaticamente
    client.InfoWithTrace(ctx, "Processando requisição", "user_id", "123")
}
```

### **3. Logs de Erro com Span**
```go
if err != nil {
    // Registra erro no span E no log com correlação
    client.LogError(ctx, err, "Falha ao processar", "operation", "create_user")
    return
}
```

### **4. Logs HTTP Estruturados**
```go
// Log automático para requisições HTTP
duration := time.Since(startTime)
client.LogHTTPRequest(ctx, r.Method, r.URL.Path, 200, duration, "bytes", len(response))
```

### **5. Logs + Atributos do Span**
```go
// Adiciona os mesmos dados no log E no span atual
client.LogWithSpanAttributes(ctx, slog.LevelInfo, "Operação concluída", map[string]any{
    "order_id": 456,
    "total": 99.99,
    "items": 3,
})
```

## 📊 **Formato dos Logs**

### **Exemplo de log com correlação:**
```json
{
  "time": "2025-12-18T15:30:45Z",
  "level": "INFO",
  "msg": "Processando requisição de soma",
  "param_a": 10.5,
  "param_b": 20.3,
  "endpoint": "/soma",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7",
  "trace_sampled": true
}
```

### **Log de erro:**
```json
{
  "time": "2025-12-18T15:30:46Z",
  "level": "ERROR", 
  "msg": "Erro ao chamar serviço externo",
  "error": "connection timeout",
  "target_service": "calc-service",
  "endpoint": "/calc",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7"
}
```

## 🔍 **Benefícios**

1. **Rastreabilidade Completa**: Logs podem ser correlacionados através de múltiplos serviços
2. **Debug Simplificado**: Encontre todos os logs de uma requisição específica pelo trace_id
3. **Observabilidade**: Conecte logs, traces e métricas automaticamente
4. **Zero Overhead**: Correlação só acontece quando há span ativo
5. **Flexibilidade**: Use métodos específicos ou logger padrão com contexto

## 🧪 **Testando**

Execute o serviço e faça uma requisição:
```bash
curl "http://localhost:8085/soma?a=10&b=20"
```

Você verá logs como:
```json
{"level":"INFO","msg":"Processando requisição de soma","param_a":10,"param_b":20,"endpoint":"/soma","trace_id":"abc123","span_id":"def456","time":"2025-12-18T15:30:45Z"}
```

## ⚡ **Performance**

- **Impacto mínimo**: Verificação rápida se span está ativo
- **Lazy evaluation**: Trace IDs só são extraídos quando necessário  
- **Sem alocações extras**: Reutiliza estruturas do OpenTelemetry
- **Configurável**: Pode usar qualquer slog.Handler como base