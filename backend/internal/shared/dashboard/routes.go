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
// UpdateScoreConfig is additionally gated to Admin.
//
// R1-W5A-N2: die beiden Notification-Routen standen bis hierher ungegated, und
// zwar unter der Begründung „per-user self-service, der Handler scopet auf
// user_id" — hier im Kommentar und wortgleich als Ausnahme in
// internal/rbaccov. Die Begründung war nie wahr: `user_notifications` hat
// keine Spalte `user_id` (Migration 021 legt die Tabelle an, 225 ergänzt nur
// einen Index; alle Schreiber — notify.Send, demoseed, vaktcomply — füllen
// ausschließlich org_id). Beide Schreibpfade sind org-weit:
//
//	MarkNotificationRead → UPDATE … WHERE id=$1 AND org_id=$2
//	MarkAllRead          → UPDATE … WHERE org_id=$1 AND read=false
//
// Die Zeile gehört allen Nutzern der Org gemeinsam. Ein Viewer konnte damit den
// Ungelesen-Status für die gesamte Organisation zurücksetzen — in einem
// Compliance-Produkt heißt das: überfällige Controls und Meldefristen
// verschwinden für jeden aus der Glocke, ohne Spur.
//
// Entschieden wurde für das Gate, nicht für eine korrigierte Begründung: eine
// org-weite Statusänderung darf keine Nur-Lese-Rolle erreichen. Die Rolle ist
// die normale Schreibrolle (Admin + SecurityAnalyst), nicht Admin — das
// Wegklicken eines Hinweises ist keine Konfigurationsänderung wie
// UpdateScoreConfig direkt darüber.
//
// Der eigentlich richtige Zuschnitt wäre ein Lese-Status PRO NUTZER. Das
// braucht eine Migration (Spalte oder Zuordnungstabelle) und ist damit eine
// eigene Story — bis dahin ist das Datenmodell org-weit, und das Gate bildet
// genau das ab.
func Register(g *echo.Group, db *pgxpool.Pool, rdb *redis.Client) {
	svc := NewService(db)
	h := NewHandler(svc, rdb)
	admin := auth.RequireRole("Admin")
	write := auth.RequireRole("Admin", "SecurityAnalyst")
	g.GET("/score", h.GetScore)
	g.GET("/score/config", h.GetScoreConfig)
	g.PUT("/score/config", h.UpdateScoreConfig, admin)
	g.GET("/backup-status", h.GetBackupStatus)
	g.GET("/aggregate", h.GetAggregate)
	g.GET("/notifications", h.ListNotifications)
	g.POST("/notifications/read-all", h.MarkAllRead, write)
	g.POST("/notifications/:id/read", h.MarkNotificationRead, write)
	// Sprint 17 S17-1: SSE-Stream-Endpoint. Klient verbindet sich nach dem
	// initialen GET /notifications und empfängt Deltas via Server-Sent Events
	// (siehe ADR-0019).
	g.GET("/notifications/stream", h.StreamNotifications)
}
