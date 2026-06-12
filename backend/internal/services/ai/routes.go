// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package ai

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"github.com/matharnica/vakt/internal/license"
)

// RegisterOptions buendelt die optionalen Service-Dependencies, die in
// Sprint 15 (S15-1/2/3) hinzukamen — Redis fuer Rate-Limit + Cache, sowie
// die Tracker-Konfig.
type RegisterOptions struct {
	Redis            *redis.Client
	RateLimitRPM     int
	DailyTokenLimit  int
	CacheTTLSeconds  int
	CostPerMTokenIn  int64
	CostPerMTokenOut int64
	// FailOpenOnOutage flips rate-limit / quota / CE-counter behaviour to
	// allow traffic when the backend (Redis or Postgres) is unreachable.
	// Default false — fail-closed, consistent with ADR-0044 (auth lockout).
	FailOpenOnOutage bool
	// OrgSettings resolves per-org model/URL overrides (S79-7).
	// When nil the system-wide model/URL is used for all orgs.
	OrgSettings OrgSettingsFunc
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
		tracker := NewUsageTracker(opts.Redis, db, UsageTrackerConfig{
			RateLimitRPM:     opts.RateLimitRPM,
			DailyTokenLimit:  opts.DailyTokenLimit,
			CacheTTLSeconds:  opts.CacheTTLSeconds,
			CostPerMTokenIn:  opts.CostPerMTokenIn,
			CostPerMTokenOut: opts.CostPerMTokenOut,
		}).WithFailOpenOnOutage(opts.FailOpenOnOutage)
		svc.WithUsageTracker(tracker)
	}
	// Apply per-org model/URL overrides (S79-7). Caller may supply opts.OrgSettings;
	// otherwise, if db is available, fall back to a built-in resolver using raw SQL
	// so this wiring requires no change to cmd/api/main.go.
	if opts.OrgSettings != nil {
		svc.WithOrgSettings(opts.OrgSettings)
	} else if db != nil {
		svc.WithOrgSettings(defaultOrgSettingsFn(db))
	}
	h := NewHandler(svc)

	// Every LLM-producing endpoint goes through the same CE-quota gate.
	// Centralising in middleware closed audit finding F3 — GenerateReport
	// and the agent endpoints were previously the unmonitored bypass.
	aiLimit := RequireAILimit(svc)

	g.GET("/ai/status", h.Status)
	// CE monthly usage counter — used by frontend to show "18/25 Anfragen diesen Monat".
	g.GET("/ai/usage", h.Usage)
	// S32-3: Ollama model list for org settings model dropdown.
	g.GET("/ai/models", h.ListOllamaModels)
	g.POST("/ai/report", h.GenerateReport, aiLimit)
	g.POST("/ai/advice", h.ComplianceAdvice, aiLimit)
	// AI Copilot — Policy-Drafting + Incident-Response-Guide (Sprint 12)
	g.POST("/ai/draft-policy", h.DraftPolicy, aiLimit)
	g.POST("/ai/incident-guide", h.IncidentResponseGuide, aiLimit)
	// Sprint 15 / S15-5: SSE-Streaming-Endpoint fuer AI-Advisor + Documentation.
	g.POST("/ai/chat/stream", h.ChatStream, aiLimit)
	// Sprint 18 / S18-3: Agent-Run-Endpoint (Plan/Execute/Reflect, SSE).
	// S32-2: runMgr für Write-Tool-Approval-Flow (ApproveCard im Frontend).
	runMgr := &AgentRunManager{}
	runner := NewAgentRunnerWithManager(svc.client, svc.model, db, svc.usage, DefaultAgentTools(db), runMgr)
	agentH := NewAgentHandler(svc.client, svc.model, runner, runMgr, db)
	// Agent run is itself a (chain of) LLM call(s) — same CE gate applies.
	// Approve/Reject are not LLM-generating, so they only run after the
	// initial AgentRun was already accounted for.
	// Agent run requires the agent_write_tools feature (Pro+).
	// The endpoint is experimental — callers receive X-Vakt-Status: experimental.
	g.POST("/ai/agent/run", agentH.AgentRun, aiLimit, license.Require(license.FeatureAgentWriteTools))
	g.POST("/ai/agent/runs/:run_id/approve", agentH.ApproveRun, license.Require(license.FeatureAgentWriteTools))
	g.POST("/ai/agent/runs/:run_id/reject", agentH.RejectRun, license.Require(license.FeatureAgentWriteTools))
	// Sprint 52 (S52-2): Gap-Explain SSE streaming per control.
	g.POST("/ai/controls/:id/explain", h.GapExplain, aiLimit)
	// Sprint 52 (S52-3): Risk narrative generation + persistence.
	g.POST("/ai/risks/:id/narrative", h.RiskNarrative, aiLimit)
	// Sprint 52 (S52-6): AI Insights list + dismiss.
	g.GET("/ai/insights", h.ListInsights)
	g.DELETE("/ai/insights/:id", h.DismissInsight)
}

// defaultOrgSettingsFn builds an OrgSettingsFunc backed by raw SQL so
// RegisterWithOptions can apply per-org overrides without importing admin.
func defaultOrgSettingsFn(db *pgxpool.Pool) OrgSettingsFunc {
	return func(ctx context.Context, orgID string) (OrgSettings, error) {
		var modelOverride, baseURLOverride *string
		err := db.QueryRow(ctx,
			`SELECT ai_model_override, ai_base_url_override FROM organizations WHERE id = $1::uuid`,
			orgID,
		).Scan(&modelOverride, &baseURLOverride)
		if err != nil {
			return OrgSettings{}, err
		}
		s := OrgSettings{}
		if modelOverride != nil {
			s.ModelOverride = *modelOverride
		}
		if baseURLOverride != nil {
			s.BaseURLOverride = *baseURLOverride
		}
		return s, nil
	}
}
