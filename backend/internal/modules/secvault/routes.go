// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvault

import (
	"github.com/labstack/echo/v4"

	"github.com/sechealth-app/sechealth/internal/license"
)

// Register wires SecretOps routes under the provided group.
// h must be a fully initialised Handler (see Init / NewHandler).
func Register(g *echo.Group, h *Handler) {
	// Projects
	g.POST("/projects", h.CreateProject)
	g.GET("/projects", h.ListProjects)
	g.DELETE("/projects/:id", h.DeleteProject)

	// Environments
	g.POST("/projects/:project_id/envs", h.CreateEnvironment)
	g.GET("/projects/:project_id/envs", h.ListEnvironments)

	// Secrets
	g.PUT("/projects/:project_id/envs/:env_id/secrets/:key", h.SetSecret)
	g.GET("/projects/:project_id/envs/:env_id/secrets/:key", h.GetSecret)
	g.GET("/projects/:project_id/envs/:env_id/secrets", h.ListSecretKeys)
	g.DELETE("/projects/:project_id/envs/:env_id/secrets/:key", h.DeleteSecret)

	// Access log
	g.GET("/projects/:project_id/envs/:env_id/secrets/:key/log", h.GetAccessLog)

	// Access log (project-level)
	g.GET("/projects/:project_id/access-log", h.GetProjectAccessLog)

	// Health
	g.GET("/projects/:project_id/health", h.GetProjectHealth)

	// Share links
	g.POST("/projects/:project_id/envs/:env_id/secrets/:key/share", h.CreateShareLink)
	g.GET("/share/:token", h.UseShareLink)

	// API tokens — Pro feature (FeatureAPI)
	g.POST("/tokens", h.CreateToken, license.Require(license.FeatureAPI))
	g.GET("/tokens", h.ListTokens, license.Require(license.FeatureAPI))
	g.DELETE("/tokens/:id", h.RevokeToken, license.Require(license.FeatureAPI))

	// Import & export
	g.POST("/projects/:project_id/import", h.ImportSecrets)
	g.GET("/projects/:project_id/envs/:env_id/export", h.ExportSecrets)

	// Secret rotation
	g.POST("/projects/:project_id/envs/:env_id/secrets/:key/rotate", h.RotateSecret)

	// Git scanner
	g.POST("/git-scans", h.TriggerGitScan)
	g.GET("/git-scans", h.ListGitScans)
	g.GET("/git-scans/:id", h.GetGitScan)
	g.GET("/git-scans/:id/results", h.GetGitScanResults)
	g.POST("/git-scans/results/:result_id/dismiss", h.DismissScanResult)
}
