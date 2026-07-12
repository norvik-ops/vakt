// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktvault

import (
	"github.com/labstack/echo/v4"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/shared/platform/features"
)

// Register wires SecretOps routes under the provided group.
// h must be a fully initialised Handler (see Init / NewHandler).
func Register(g *echo.Group, h *Handler) {
	rw := auth.RequireRole("Admin", "SecurityAnalyst")
	admin := auth.RequireRole("Admin")

	// Projects — read: analyst+; delete: admin only
	g.POST("/projects", h.CreateProject, rw)
	g.GET("/projects", h.ListProjects, rw)
	g.GET("/projects/:id", h.GetProject, rw)
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

	// Share links — admin only (creates an unauthenticated access URL).
	// S127-3 (D6): the CONSUMING route GET /share/:token is public (the external
	// recipient has no session) and now lives in RegisterPublic — mounting it here
	// under `protected` 401'd every external share link. Creation stays admin-gated.
	g.POST("/projects/:project_id/envs/:env_id/secrets/:key/share", h.CreateShareLink, admin)

	// API tokens — Pro feature (FeatureAPI)
	g.POST("/tokens", h.CreateToken, admin, features.Require(features.FeatureAPI))
	g.GET("/tokens", h.ListTokens, rw, features.Require(features.FeatureAPI))
	g.DELETE("/tokens/:id", h.RevokeToken, admin, features.Require(features.FeatureAPI))

	// Import & export — import: admin; export: analyst+
	g.POST("/projects/:project_id/import", h.ImportSecrets, admin)
	g.GET("/projects/:project_id/envs/:env_id/export", h.ExportSecrets, rw)

	// Advanced vault workflows (rotation, git leak scans, access reviews) are
	// Pro — gated by FeatureSecVault, mirroring the public pricing page.
	// Basic secret storage (projects, envs, CRUD, sharing, import/export) is Community.
	vaultPro := features.Require(features.FeatureSecVault)

	// Secret rotation — admin only
	g.POST("/projects/:project_id/envs/:env_id/secrets/:key/rotate", h.RotateSecret, admin, vaultPro)

	// Git scanner — analyst+ to trigger/view
	g.POST("/git-scans", h.TriggerGitScan, rw, vaultPro)
	g.GET("/git-scans", h.ListGitScans, rw, vaultPro)
	g.GET("/git-scans/:id", h.GetGitScan, rw, vaultPro)
	g.GET("/git-scans/:id/results", h.GetGitScanResults, rw, vaultPro)
	g.POST("/git-scans/results/:result_id/dismiss", h.DismissScanResult, rw, vaultPro)

	// S70-5: Vault Access Reviews (quarterly)
	g.GET("/access-reviews", h.ListAccessReviews, rw, vaultPro)
	g.POST("/access-reviews", h.CreateAccessReview, admin, vaultPro)
	g.GET("/access-reviews/:id", h.GetAccessReview, rw, vaultPro)
	g.POST("/access-reviews/:id/complete", h.CompleteAccessReview, admin, vaultPro)
}

// RegisterPublic mounts the token-only share-link consumer route (S127-3, D6).
// The external recipient of a share link has no Vakt session; UseShareLink is
// already validated by the URL token alone (vaktvault stores only its hash), so
// the caller mounts this on a PUBLIC group (no auth/CSRF/license) with an IP
// rate limiter.
func RegisterPublic(g *echo.Group, h *Handler) {
	g.GET("/share/:token", h.UseShareLink)
}
