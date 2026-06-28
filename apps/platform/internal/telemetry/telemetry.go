// Package telemetry wires the Go server's slog output to PostHog via the
// OpenTelemetry OTLP logs exporter. It is a no-op when POSTHOG_API_KEY is unset.
package telemetry

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

const (
	defaultServiceName = "tendersbay-platform"
	defaultHost        = "https://eu.i.posthog.com"
)

// Config holds the PostHog OTLP-logs connection settings.
type Config struct {
	APIKey      string
	Host        string
	ServiceName string
}

// ConfigFromEnv reads POSTHOG_API_KEY / POSTHOG_HOST, defaulting the host to the
// EU endpoint and the service name to tendersbay-platform.
func ConfigFromEnv() Config {
	host := os.Getenv("POSTHOG_HOST")
	if host == "" {
		host = defaultHost
	}
	return Config{
		APIKey:      os.Getenv("POSTHOG_API_KEY"),
		Host:        host,
		ServiceName: defaultServiceName,
	}
}

// Setup installs the default slog logger. With a key, slog records are exported
// to PostHog's OTLP logs endpoint; without one, slog writes to stdout and the
// returned shutdown is a no-op.
func Setup(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	noop := func(context.Context) error { return nil }

	if cfg.APIKey == "" {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
		return noop, nil
	}

	exporter, err := otlploghttp.New(ctx,
		otlploghttp.WithEndpointURL(cfg.Host+"/otlp/v1/logs"),
		otlploghttp.WithHeaders(map[string]string{
			"Authorization": "Bearer " + cfg.APIKey,
		}),
	)
	if err != nil {
		return noop, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceNameKey.String(cfg.ServiceName)),
	)
	if err != nil {
		_ = exporter.Shutdown(ctx)
		return noop, err
	}

	provider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
	)
	global.SetLoggerProvider(provider)
	slog.SetDefault(otelslog.NewLogger(cfg.ServiceName))

	return provider.Shutdown, nil
}
