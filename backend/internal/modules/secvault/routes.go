// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvault

import (
	"github.com/labstack/echo/v4"

	"github.com/sechealth-app/sechealth/internal/auth"
	"github.com/sechealth-app/sechealth/internal/license"
)

// Register wires SecretOps routes under the provided group.
// h must be a fully initialised Handler (see Init / NewHandler).
func Register(g *echo.Group, h *Handler) {
	rw := auth.RequireRole("Admin", "SecurityAnalyst")
	admin := auth.RequireRole("Admin")

	// Projects — read: analyst+; delete: admin only
	g.POST("/projects", h.CreateProject, rw)
	g.GET("/projects", h.ListProjects, rw)
	g.DELETE("/projects/:id", h.DeleteProject, admin)

	// Environments
	g.POST("/projects/:project_id/envs", h.CreateEnvironment, rw)
	g.GET("/projects/:project_id/envs", h.ListEnvironments, rw)

	// Secrets — read/write: analyst+; delete: admin only
	g.PUT("/projects/:project_id/envs/:env_id/secrets/:key", h.SetSecret, rw)
	g.GET("/projects/:project_id/envs/:env_id/secrets/:key", h.GetSecret, rw)
	g.GET("/projects/:project_id/envs/:env_id/secrets", h.ListSecretKeys, rw)
	g.DELETE("/projects/:project_id/envs/:env_id/secrets/:key", h.DeleteSecret, admin)

	// Access log
	g.GET("/projects/:project_id/envs/:env_id/secrets/:key/log", h.GetAccessLog, rw)

	// Access log (project-level)
	g.GET("/projects/:project_id/access-log", h.GetProjectAccessLog, rw)

	// Health
	g.GET("/projects/:project_id/health", h.GetProjectHealth, rw)

	// Share links — admin only (creates an unauthenticated access URL)
	g.POST("/projects/:project_id/envs/:env_id/secrets/:key/share", h.CreateShareLink, admin)
	g.GET("/share/:token", h.UseShareLink) // public — validated by token only

	// API tokens — Pro feature (FeatureAPI)
	g.POST("/tokens", h.CreateToken, admin, license.Require(license.FeatureAPI))
	g.GET("/tokens", h.ListTokens, rw, license.Require(license.FeatureAPI))
	g.DELETE("/tokens/:id", h.RevokeToken, admin, license.Require(license.FeatureAPI))

	// Import & export — import: admin; export: analyst+
	g.POST("/projects/:project_id/import", h.ImportSecrets, admin)
	g.GET("/projects/:project_id/envs/:env_id/export", h.ExportSecrets, rw)

	// Secret rotation — admin only
	g.POST("/projects/:project_id/envs/:env_id/secrets/:key/rotate", h.RotateSecret, admin)

	// Git scanner — analyst+ to trigger/view
	g.POST("/git-scans", h.TriggerGitScan, rw)
	g.GET("/git-scans", h.ListGitScans, rw)
	g.GET("/git-scans/:id", h.GetGitScan, rw)
	g.GET("/git-scans/:id/results", h.GetGitScanResults, rw)
	g.POST("/git-scans/results/:result_id/dismiss", h.DismissScanResult, rw)
}
