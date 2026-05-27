// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package telemetry provides an opt-in OpenTelemetry initialiser for Vakt.
//
// The package is designed around ADR-0011: OpenTelemetry instrumentation is
// always present in the code, but the exporter is a no-op unless the operator
// sets OTEL_EXPORTER_OTLP_ENDPOINT explicitly. There is no auto-discovery,
// no default endpoint, no phone-home — the operator declares what to send,
// and to where.
//
// This implementation deliberately stays dependency-light: it reads the OTel
// configuration from environment variables, and if no endpoint is set it
// returns a noop function. When a real OTel SDK is added, the noop branch
// becomes the boot-time check that prevents accidental external sends.
package telemetry

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Config captures the OTel environment knobs we honour. All fields are
// strings — empty means "use default" (which for endpoint means "disabled").
type Config struct {
	Endpoint      string // OTEL_EXPORTER_OTLP_ENDPOINT
	ServiceName   string // OTEL_SERVICE_NAME, default "vakt-api"
	ResourceAttrs string // OTEL_RESOURCE_ATTRIBUTES, comma-separated key=value
	SamplerArg    string // OTEL_TRACES_SAMPLER_ARG, default "0.1"
	HeadersRaw    string // OTEL_EXPORTER_OTLP_HEADERS
}

// FromEnv reads OTel configuration from process environment.
func FromEnv() Config {
	cfg := Config{
		Endpoint:      os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		ServiceName:   os.Getenv("OTEL_SERVICE_NAME"),
		ResourceAttrs: os.Getenv("OTEL_RESOURCE_ATTRIBUTES"),
		SamplerArg:    os.Getenv("OTEL_TRACES_SAMPLER_ARG"),
		HeadersRaw:    os.Getenv("OTEL_EXPORTER_OTLP_HEADERS"),
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = "vakt-api"
	}
	if cfg.SamplerArg == "" {
		cfg.SamplerArg = "0.1"
	}
	return cfg
}

// Shutdown is returned by Init. The caller MUST defer it so spans get flushed
// on process exit. In disabled mode it is a no-op.
type Shutdown func(ctx context.Context) error

// Init configures the OTel SDK based on cfg. If cfg.Endpoint is empty, a no-op
// is returned and a single info log is emitted noting the disabled state — no
// accidental external traffic.
//
// When a full OTel SDK is wired (go.opentelemetry.io/otel/sdk/trace etc.) the
// noop branch becomes the production gate: missing OTEL_EXPORTER_OTLP_ENDPOINT
// keeps the system offline.
func Init(cfg Config) Shutdown {
	if cfg.Endpoint == "" {
		log.Info().
			Str("service", cfg.ServiceName).
			Msg("telemetry: OTEL_EXPORTER_OTLP_ENDPOINT not set — OpenTelemetry export disabled (no traces leave this instance)")
		return func(context.Context) error { return nil }
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// OTLP/HTTP exporter — simpler than gRPC, no separate TLS config needed
	// when targeting an internal Tempo/Otel collector.
	exporterOpts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(strings.TrimPrefix(strings.TrimPrefix(cfg.Endpoint, "https://"), "http://")),
	}
	if strings.HasPrefix(cfg.Endpoint, "http://") {
		exporterOpts = append(exporterOpts, otlptracehttp.WithInsecure())
	}
	if cfg.HeadersRaw != "" {
		exporterOpts = append(exporterOpts, otlptracehttp.WithHeaders(parseHeaders(cfg.HeadersRaw)))
	}
	exp, err := otlptrace.New(ctx, otlptracehttp.NewClient(exporterOpts...))
	if err != nil {
		log.Error().Err(err).Msg("telemetry: OTLP exporter init failed — falling back to noop")
		return func(context.Context) error { return nil }
	}

	res, _ := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(append(parseAttrs(cfg.ResourceAttrs),
			attribute.String("service.name", cfg.ServiceName))...),
	)

	sampleRate, _ := strconv.ParseFloat(cfg.SamplerArg, 64)
	if sampleRate <= 0 || sampleRate > 1 {
		sampleRate = 0.1
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp,
			sdktrace.WithMaxQueueSize(2048),
			sdktrace.WithBatchTimeout(5*time.Second),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampleRate))),
	)
	otel.SetTracerProvider(tp)

	log.Info().
		Str("service", cfg.ServiceName).
		Str("endpoint", cfg.Endpoint).
		Float64("sample_rate", sampleRate).
		Msg("telemetry: OpenTelemetry initialised, exporting via OTLP/HTTP")

	return func(ctx context.Context) error {
		log.Info().Msg("telemetry: shutting down, flushing pending spans")
		shutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return tp.Shutdown(shutCtx)
	}
}

// parseHeaders converts OTEL_EXPORTER_OTLP_HEADERS ("k1=v1,k2=v2") to a map.
func parseHeaders(raw string) map[string]string {
	out := map[string]string{}
	for _, kv := range strings.Split(raw, ",") {
		kv = strings.TrimSpace(kv)
		if eq := strings.IndexByte(kv, '='); eq > 0 {
			out[kv[:eq]] = kv[eq+1:]
		}
	}
	return out
}

// parseAttrs converts OTEL_RESOURCE_ATTRIBUTES ("k1=v1,k2=v2") to OTel attrs.
func parseAttrs(raw string) []attribute.KeyValue {
	if raw == "" {
		return nil
	}
	var out []attribute.KeyValue
	for _, kv := range strings.Split(raw, ",") {
		kv = strings.TrimSpace(kv)
		if eq := strings.IndexByte(kv, '='); eq > 0 {
			out = append(out, attribute.String(kv[:eq], kv[eq+1:]))
		}
	}
	return out
}
