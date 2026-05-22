// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package ai

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

// RegisterOptions buendelt die optionalen Service-Dependencies, die in
// Sprint 15 (S15-1/2/3) hinzukamen — Redis fuer Rate-Limit + Cache, sowie
// die Tracker-Konfig.
type RegisterOptions struct {
	Redis           *redis.Client
	RateLimitRPM    int
	DailyTokenLimit int
	CacheTTLSeconds int
	CostPerMTokenIn int64
	CostPerMTokenOut int64
}

// Register mounts AI report endpoints.
// provider: "disabled" | "openai" (OpenAI-compatible).
// The group must already have auth middleware applied.
//
// AI features are Community since v0.6.x — with qwen2.5:3b als Default
// (Apache 2.0, ~1.9 GB RAM, CPU-tauglich) ist die AI lokal in jeder
// Instanz lauffähig, das frühere Pro-Gate war daher Marketing-Limitierung
// ohne echten Schutz. Premium-Compliance-Features (TISAX, DORA, NIS2-
// Reporting, EU-AI-Act, AuditPDF, SSO, API-Access, SecReflex/SecPulse-
// Advanced, Granular-Permissions, Supplier-Portal) bleiben Pro.
func Register(g *echo.Group, db *pgxpool.Pool, provider, baseURL, apiKey, model string) {
	RegisterWithOptions(g, db, provider, baseURL, apiKey, model, RegisterOptions{})
}

// RegisterWithOptions ist die Sprint-15-Variante mit optionaler Rate-Limit-,
// Quota- und Cache-Verdrahtung. Wenn opts.Redis nil ist oder einzelne
// Konfig-Felder 0 sind, faellt der entsprechende Pfad auf "always allow"
// / "no cache" zurueck — abwaertskompatibel.
func RegisterWithOptions(g *echo.Group, db *pgxpool.Pool, provider, baseURL, apiKey, model string, opts RegisterOptions) {
	if provider == "disabled" || provider == "" {
		return
	}
	svc := NewService(db, baseURL, apiKey, model)
	if opts.Redis != nil {
		svc.WithUsageTracker(NewUsageTracker(opts.Redis, db, UsageTrackerConfig{
			RateLimitRPM:     opts.RateLimitRPM,
			DailyTokenLimit:  opts.DailyTokenLimit,
			CacheTTLSeconds:  opts.CacheTTLSeconds,
			CostPerMTokenIn:  opts.CostPerMTokenIn,
			CostPerMTokenOut: opts.CostPerMTokenOut,
		}))
	}
	h := NewHandler(svc)
	g.GET("/ai/status", h.Status)
	g.POST("/ai/report", h.GenerateReport)
	g.POST("/ai/advice", h.ComplianceAdvice)
	// AI Copilot — Policy-Drafting + Incident-Response-Guide (Sprint 12)
	g.POST("/ai/draft-policy", h.DraftPolicy)
	g.POST("/ai/incident-guide", h.IncidentResponseGuide)
	// Sprint 15 / S15-5: SSE-Streaming-Endpoint fuer AI-Advisor + Documentation.
	g.POST("/ai/chat/stream", h.ChatStream)
	// Sprint 18 / S18-3: Agent-Run-Endpoint (Plan/Execute/Reflect, SSE).
	runner := NewAgentRunner(svc.client, svc.model, db, svc.usage, DefaultAgentTools(db))
	agentH := NewAgentHandler(svc.client, svc.model, runner)
	g.POST("/ai/agent/run", agentH.AgentRun)
}
