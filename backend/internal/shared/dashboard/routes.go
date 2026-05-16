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
)

// Register mounts all dashboard routes under the provided Echo group,
// protecting each endpoint with the supplied auth middleware. The caller is
// responsible for passing a group already rooted at /api/v1/dashboard.
func Register(g *echo.Group, db *pgxpool.Pool, rdb *redis.Client, auth echo.MiddlewareFunc) {
	h := NewHandler(db, rdb)
	g.GET("/score", h.GetScore, auth)
	g.GET("/score/config", h.GetScoreConfig, auth)
	g.PUT("/score/config", h.UpdateScoreConfig, auth)
	g.GET("/backup-status", h.GetBackupStatus, auth)
	g.GET("/aggregate", h.GetAggregate, auth)
	g.GET("/notifications", h.ListNotifications, auth)
	g.POST("/notifications/read-all", h.MarkAllRead, auth)
	g.POST("/notifications/:id/read", h.MarkNotificationRead, auth)
}
