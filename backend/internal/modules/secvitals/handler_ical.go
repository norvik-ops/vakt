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
	milestones, err := h.q.ListCKICalMilestones(ctx, oid)
	if err != nil {
		log.Error().Err(err).Str("org_id", oid).Msg("ical: query milestones")
		return errResp(c, http.StatusInternalServerError, "failed to load deadlines", "CK_ICAL_ERROR")
	}
	for _, m := range milestones {
		events = append(events, icalEvent{
			uid:         m.ID + "@vakt",
			dtstart:     m.MilestoneDate.Time.Format("20060102"),
			summary:     m.Title,
			description: m.Description,
		})
	}

	// --- Source 2: Open/in-progress CAPAs with due dates ---
	capas, err := h.q.ListCKICalCAPAs(ctx, oid)
	if err != nil {
		log.Error().Err(err).Str("org_id", oid).Msg("ical: query capas")
		return errResp(c, http.StatusInternalServerError, "failed to load deadlines", "CK_ICAL_ERROR")
	}
	for _, ca := range capas {
		events = append(events, icalEvent{
			uid:         ca.ID + "@vakt",
			dtstart:     ca.DueDate.Time.Format("20060102"),
			summary:     "CAPA fällig: " + ca.Title,
			description: "Corrective and Preventive Action",
		})
	}

	// --- Source 3: Evidence expiring within the future ---
	evidences, err := h.q.ListCKICalExpiringEvidence(ctx, oid)
	if err != nil {
		log.Error().Err(err).Str("org_id", oid).Msg("ical: query evidence")
		return errResp(c, http.StatusInternalServerError, "failed to load deadlines", "CK_ICAL_ERROR")
	}
	for _, ev := range evidences {
		events = append(events, icalEvent{
			uid:         ev.ID + "@vakt",
			dtstart:     ev.ExpiresAt.Time.UTC().Format("20060102"),
			summary:     "Nachweis läuft ab: " + ev.Title,
			description: "Compliance-Nachweis läuft ab",
		})
	}

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
		fmt.Fprintf(&sb, "UID:%s\r\n", icalEscape(ev.uid))
		fmt.Fprintf(&sb, "DTSTAMP:%s\r\n", dtstamp)
		fmt.Fprintf(&sb, "DTSTART;VALUE=DATE:%s\r\n", ev.dtstart)
		fmt.Fprintf(&sb, "SUMMARY:%s\r\n", icalEscape(ev.summary))
		if ev.description != "" {
			fmt.Fprintf(&sb, "DESCRIPTION:%s\r\n", icalEscape(ev.description))
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
