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

	"github.com/rs/zerolog/log"
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

	// Real OTel SDK initialisation goes here. For now we log the intent so
	// operators see at startup that OTel is configured.
	log.Info().
		Str("service", cfg.ServiceName).
		Str("endpoint", cfg.Endpoint).
		Str("sampler_arg", cfg.SamplerArg).
		Msg("telemetry: OpenTelemetry endpoint configured — full SDK wire-up pending")

	return func(context.Context) error {
		log.Info().Msg("telemetry: shutdown")
		return nil
	}
}
