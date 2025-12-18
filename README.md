# OpenTelemetry Helpers

Este projeto demonstra uma implementação de microserviço em Go com observabilidade completa usando OpenTelemetry, incluindo tracing distribuído e logging estruturado.

## 🚀 Características

- **Tracing distribuído** usando OpenTelemetry
- **Logging estruturado** com slog
- **Propagação de contexto** entre serviços
- **Configuração declarativa** via YAML
- **Integração com OTLP** para exportação de telemetria

## 📁 Estrutura do Projeto

```
.
├── go.mod              # Dependências do módulo Go
├── main.go             # Aplicação principal com servidor HTTP
├── otel-config.yaml    # Configuração do OpenTelemetry
├── telemetry/
│   └── telemetry.go    # Setup e configuração do OpenTelemetry
└── README.md
```

## 🛠 Pré-requisitos

- Go 1.23.0 ou superior
- Collector OpenTelemetry rodando em `localhost:4318` (opcional)
- Serviço adicional rodando em `localhost:8082` (para demonstração completa)

## 📦 Instalação

1. Clone o repositório:
```bash
git clone <url-do-repositorio>
cd helpers
```

2. Instale as dependências:
```bash
go mod download
```

## 🏃 Como Executar

1. **Inicie o coletor OpenTelemetry** (opcional):
```bash
# Exemplo usando docker
docker run -p 4318:4318 otel/opentelemetry-collector
```

2. **Execute a aplicação**:
```bash
go run main.go
```

3. **Teste a aplicação**:
```bash
curl "http://localhost:8085/soma?a=10&b=5"
```

## 🔧 Configuração

### Arquivo `otel-config.yaml`

O arquivo de configuração define:

- **Recurso**: Nome do serviço e ambiente
- **Propagadores**: Contexto de trace e baggage
- **Exportador**: OTLP HTTP para `localhost:4318`
- **Processamento**: Batch processing para otimização

```yaml
file_format: "0.3"
disabled: false
resource:
  schema_url: https://opentelemetry.io/schemas/1.26.0
  attributes:
    - name: service.name
      value: "serviceconfig12"
    - name: environment
      value: "prod"

propagator:
  composite: [ tracecontext, baggage ]

tracer_provider:
  processors:
    - batch:
        exporter:
          otlp:
            protocol: http/protobuf
            endpoint: http://localhost:4318
```

### Variáveis de Ambiente

O arquivo de configuração suporta expansão de variáveis de ambiente. Exemplo:
```yaml
endpoint: ${OTEL_ENDPOINT:-http://localhost:4318}
```

## 🌐 API Endpoints

### `GET /soma`

Realiza uma operação de soma e demonstra tracing distribuído.

**Parâmetros:**
- `a` (float): Primeiro número
- `b` (float): Segundo número

**Exemplo:**
```bash
curl "http://localhost:8085/soma?a=10&b=5"
```

**Comportamento:**
1. Cria um span para a operação
2. Valida os parâmetros de entrada
3. Adiciona atributos ao span
4. Faz uma chamada HTTP para `localhost:8082/calc`
5. Propaga o contexto de trace
6. Retorna o resultado

## 📊 Observabilidade

### Tracing

- Cada requisição gera spans com informações detalhadas
- Propagação automática de contexto entre serviços
- Registro de erros e atributos customizados
- Export para sistemas compatíveis com OTLP

### Logging

- Logs estruturados em formato JSON
- Correlação automática com trace e span IDs
- Diferentes níveis de log (Info, Error)
- Contexto preservado entre chamadas

### Exemplo de Log:
```json
{
  "time": "2025-12-18T10:30:00Z",
  "level": "INFO",
  "msg": "chamada para o serviço 2 realizada com sucesso",
  "response": "200 OK",
  "trace_id": "abc123...",
  "span_id": "def456..."
}
```

## 🔗 Integração com Outros Serviços

Esta aplicação foi projetada para se comunicar com outros serviços:

1. **Serviço de Cálculo** (`localhost:8082`): Recebe requisições com propagação de contexto
2. **Collector OpenTelemetry** (`localhost:4318`): Recebe dados de telemetria

## 🧪 Desenvolvimento

### Estrutura do Código

- `main.go`: Servidor HTTP principal e handlers
- `telemetry/telemetry.go`: Setup e configuração do OpenTelemetry
- `logWithTrace()`: Função helper para logging com correlação

### Dependências Principais

- `go.opentelemetry.io/otel`: SDK core do OpenTelemetry
- `go.opentelemetry.io/contrib/otelconf`: Configuração declarativa
- `log/slog`: Logging estruturado nativo do Go

## 🤝 Contribuição

1. Faça um fork do projeto
2. Crie uma branch para sua feature (`git checkout -b feature/AmazingFeature`)
3. Commit suas mudanças (`git commit -m 'Add some AmazingFeature'`)
4. Push para a branch (`git push origin feature/AmazingFeature`)
5. Abra um Pull Request

## 📄 Licença

Este projeto é distribuído sob a licença MIT. Veja `LICENSE` para mais informações.

## 📞 Suporte

Para questões e suporte:
- Abra uma issue no repositório
- Entre em contato com a equipe de desenvolvimento