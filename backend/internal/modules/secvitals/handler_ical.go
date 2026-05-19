// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvitals

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// CalendarDeadlines handles GET /api/v1/secvitals/calendar/deadlines.ics.
// Returns an iCalendar feed of upcoming compliance deadlines:
// - Audit milestones (ck_audit_milestones)
// - Open/in-progress CAPAs with due dates
// - Evidence items approaching expiry
func (h *Handler) CalendarDeadlines(c echo.Context) error {
	ctx := c.Request().Context()
	oid := orgID(c)
	now := time.Now().UTC()

	type icalEvent struct {
		uid         string
		dtstart     string
		summary     string
		description string
	}

	var events []icalEvent

	// --- Source 1: Audit milestones ---
	milestoneRows, err := h.db.Query(ctx, `
		SELECT id::text, title, COALESCE(description, ''), milestone_date
		FROM ck_audit_milestones
		WHERE org_id = $1::uuid
		  AND status IN ('upcoming')
		  AND milestone_date >= CURRENT_DATE
		ORDER BY milestone_date
		LIMIT 100`, oid)
	if err != nil {
		log.Error().Err(err).Str("org_id", oid).Msg("ical: query milestones")
		return errResp(c, http.StatusInternalServerError, "failed to load deadlines", "CK_ICAL_ERROR")
	}
	defer milestoneRows.Close()
	for milestoneRows.Next() {
		var id, title, desc string
		var milestoneDate time.Time
		if err := milestoneRows.Scan(&id, &title, &desc, &milestoneDate); err != nil {
			log.Warn().Err(err).Msg("ical: scan milestone row")
			continue
		}
		events = append(events, icalEvent{
			uid:         id + "@vakt",
			dtstart:     milestoneDate.Format("20060102"),
			summary:     title,
			description: desc,
		})
	}
	milestoneRows.Close()

	// --- Source 2: Open/in-progress CAPAs with due dates ---
	capaRows, err := h.db.Query(ctx, `
		SELECT id::text, title, due_date
		FROM ck_capas
		WHERE org_id = $1::uuid
		  AND due_date IS NOT NULL
		  AND status IN ('open', 'in_progress')
		ORDER BY due_date
		LIMIT 100`, oid)
	if err != nil {
		log.Error().Err(err).Str("org_id", oid).Msg("ical: query capas")
		return errResp(c, http.StatusInternalServerError, "failed to load deadlines", "CK_ICAL_ERROR")
	}
	defer capaRows.Close()
	for capaRows.Next() {
		var id, title string
		var dueDate time.Time
		if err := capaRows.Scan(&id, &title, &dueDate); err != nil {
			log.Warn().Err(err).Msg("ical: scan capa row")
			continue
		}
		events = append(events, icalEvent{
			uid:         id + "@vakt",
			dtstart:     dueDate.Format("20060102"),
			summary:     "CAPA fällig: " + title,
			description: "Corrective and Preventive Action",
		})
	}
	capaRows.Close()

	// --- Source 3: Evidence expiring within 90 days ---
	evidenceRows, err := h.db.Query(ctx, `
		SELECT e.id::text, e.title, e.expires_at
		FROM ck_evidence e
		WHERE e.org_id = $1::uuid
		  AND e.expires_at IS NOT NULL
		  AND e.expires_at > NOW()
		ORDER BY e.expires_at
		LIMIT 50`, oid)
	if err != nil {
		log.Error().Err(err).Str("org_id", oid).Msg("ical: query evidence")
		return errResp(c, http.StatusInternalServerError, "failed to load deadlines", "CK_ICAL_ERROR")
	}
	defer evidenceRows.Close()
	for evidenceRows.Next() {
		var id, title string
		var expiresAt time.Time
		if err := evidenceRows.Scan(&id, &title, &expiresAt); err != nil {
			log.Warn().Err(err).Msg("ical: scan evidence row")
			continue
		}
		events = append(events, icalEvent{
			uid:         id + "@vakt",
			dtstart:     expiresAt.UTC().Format("20060102"),
			summary:     "Nachweis läuft ab: " + title,
			description: "Compliance-Nachweis läuft ab",
		})
	}
	evidenceRows.Close()

	// Build iCalendar output.
	dtstamp := now.Format("20060102T150405Z")
	var sb strings.Builder
	sb.WriteString("BEGIN:VCALENDAR\r\n")
	sb.WriteString("VERSION:2.0\r\n")
	sb.WriteString("PRODID:-//Vakt//Compliance Calendar//DE\r\n")
	sb.WriteString("CALSCALE:GREGORIAN\r\n")
	sb.WriteString("X-WR-CALNAME:Vakt Compliance\r\n")

	for _, ev := range events {
		sb.WriteString("BEGIN:VEVENT\r\n")
		sb.WriteString(fmt.Sprintf("UID:%s\r\n", icalEscape(ev.uid)))
		sb.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", dtstamp))
		sb.WriteString(fmt.Sprintf("DTSTART;VALUE=DATE:%s\r\n", ev.dtstart))
		sb.WriteString(fmt.Sprintf("SUMMARY:%s\r\n", icalEscape(ev.summary)))
		if ev.description != "" {
			sb.WriteString(fmt.Sprintf("DESCRIPTION:%s\r\n", icalEscape(ev.description)))
		}
		sb.WriteString("END:VEVENT\r\n")
	}

	sb.WriteString("END:VCALENDAR\r\n")

	c.Response().Header().Set("Content-Type", "text/calendar; charset=utf-8")
	c.Response().Header().Set("Content-Disposition", `attachment; filename="vakt-compliance.ics"`)
	return c.String(http.StatusOK, sb.String())
}

// icalEscape escapes special characters in iCalendar text values per RFC 5545.
func icalEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, ";", `\;`)
	s = strings.ReplaceAll(s, ",", `\,`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", "")
	return s
}
