// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package ai

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

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
	if provider == "disabled" || provider == "" {
		return
	}
	svc := NewService(db, baseURL, apiKey, model)
	h := NewHandler(svc)
	g.GET("/ai/status", h.Status)
	g.POST("/ai/report", h.GenerateReport)
	g.POST("/ai/advice", h.ComplianceAdvice)
	// AI Copilot — Policy-Drafting + Incident-Response-Guide (Sprint 12)
	g.POST("/ai/draft-policy", h.DraftPolicy)
	g.POST("/ai/incident-guide", h.IncidentResponseGuide)
}
