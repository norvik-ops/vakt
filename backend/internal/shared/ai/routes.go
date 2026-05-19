// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package ai

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/matharnica/vakt/internal/license"
)

// Register mounts AI report endpoints.
// provider: "disabled" | "openai" (OpenAI-compatible).
// The group must already have auth middleware applied.
func Register(g *echo.Group, db *pgxpool.Pool, provider, baseURL, apiKey, model string) {
	if provider == "disabled" || provider == "" {
		return
	}
	svc := NewService(db, baseURL, apiKey, model)
	h := NewHandler(svc)
	// AI Advisor endpoints — Pro feature
	g.GET("/ai/status", h.Status, license.Require(license.FeatureAIAdvisor))
	g.POST("/ai/report", h.GenerateReport, license.Require(license.FeatureAIAdvisor))
	g.POST("/ai/advice", h.ComplianceAdvice, license.Require(license.FeatureAIAdvisor))
}
