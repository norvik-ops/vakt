// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package dashboard aggregates cross-module metrics into a single security
// score and manages in-app notifications stored in the user_notifications
// table. It queries SecPulse findings, SecPrivacy breaches, and SecVitals
// frameworks directly via raw SQL so it remains decoupled from each module's
// service layer.
package dashboard

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"github.com/matharnica/vakt/internal/auth"
)

// Register mounts all dashboard routes under the provided Echo group. The caller
// must pass a group off the `protected` chain rooted at /api/v1/dashboard so the
// routes inherit auth, CSRF, MFA and per-org rate limiting.
//
// S121-B3 (R3): previously mounted on the bare `api` group with only inline
// auth middleware — the mutating PUT /score/config had neither CSRF protection
// nor a role gate, so a Viewer with a Bearer token (no CSRF cookie) could rewrite
// the org-wide security-score weighting. Mounting on `protected` restores CSRF/MFA;
// UpdateScoreConfig is additionally gated to Admin. Per-user notification actions
// stay open to any authenticated user.
func Register(g *echo.Group, db *pgxpool.Pool, rdb *redis.Client) {
	svc := NewService(db)
	h := NewHandler(svc, rdb)
	admin := auth.RequireRole("Admin")
	g.GET("/score", h.GetScore)
	g.GET("/score/config", h.GetScoreConfig)
	g.PUT("/score/config", h.UpdateScoreConfig, admin)
	g.GET("/backup-status", h.GetBackupStatus)
	g.GET("/aggregate", h.GetAggregate)
	g.GET("/notifications", h.ListNotifications)
	g.POST("/notifications/read-all", h.MarkAllRead)
	g.POST("/notifications/:id/read", h.MarkNotificationRead)
	// Sprint 17 S17-1: SSE-Stream-Endpoint. Klient verbindet sich nach dem
	// initialen GET /notifications und empfängt Deltas via Server-Sent Events
	// (siehe ADR-0019).
	g.GET("/notifications/stream", h.StreamNotifications)
}
