package telemetry

import (
	"context"
	"os"

	otelconf "go.opentelemetry.io/contrib/otelconf/v0.3.0"
	"go.opentelemetry.io/otel"
)

func Setup(ctx context.Context, confPath string) (func(context.Context) error, error) {
	b, err := os.ReadFile(confPath)
	if err != nil {
		return nil, err
	}

	b = []byte(os.ExpandEnv(string(b)))

	conf, err := otelconf.ParseYAML(b)
	if err != nil {
		return nil, err
	}

	sdk, err := otelconf.NewSDK(otelconf.WithContext(ctx), otelconf.WithOpenTelemetryConfiguration(*conf))
	if err != nil {
		return nil, err
	}

	otel.SetTracerProvider(sdk.TracerProvider())
	otel.SetMeterProvider(sdk.MeterProvider())
	return sdk.Shutdown, nil
}
