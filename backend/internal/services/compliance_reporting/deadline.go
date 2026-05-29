// Package compliance_reporting provides shared deadline-tracking utilities for
// regulatory incident reporting (DORA, NIS2, DSGVO/GDPR).
//
// It eliminates duplication between the per-module deadline helpers in
// vaktcomply/service_incidents.go and any future module that needs the same
// traffic-light (Ampel) logic.
package compliance_reporting

import "time"

// DeadlineDefinition describes a single reporting deadline window.
type DeadlineDefinition struct {
	// Label is a human-readable identifier, e.g. "4h", "24h", "72h", "30d".
	Label    string
	Duration time.Duration
}

// DeadlineSet is an ordered list of deadline definitions for a specific
// regulatory framework.
type DeadlineSet []DeadlineDefinition

// DeadlineWindow is the computed state of one deadline for a concrete incident.
type DeadlineWindow struct {
	// Label matches the originating DeadlineDefinition.Label.
	Label      string
	DeadlineAt time.Time
	// ReportedAt is nil when the deadline has not yet been fulfilled.
	ReportedAt *time.Time
	// WarnSent indicates that a "deadline approaching" notification was sent.
	WarnSent bool
}

// Predefined deadline sets for the three supported frameworks.
var (
	// DORADeadlines reflects DORA Art. 19 ICT-incident reporting windows:
	// initial notification (4 h), early warning (24 h), intermediate report (72 h),
	// final report (30 days).
	DORADeadlines = DeadlineSet{
		{Label: "4h", Duration: 4 * time.Hour},
		{Label: "24h", Duration: 24 * time.Hour},
		{Label: "72h", Duration: 72 * time.Hour},
		{Label: "30d", Duration: 720 * time.Hour},
	}

	// NIS2Deadlines reflects NIS2 Art. 23 significant incident reporting windows:
	// early warning (24 h), notification (72 h), final report (30 days).
	NIS2Deadlines = DeadlineSet{
		{Label: "24h", Duration: 24 * time.Hour},
		{Label: "72h", Duration: 72 * time.Hour},
		{Label: "30d", Duration: 720 * time.Hour},
	}

	// DSGVODeadlines reflects DSGVO Art. 33 personal-data breach notification:
	// supervisory authority notification within 72 hours.
	DSGVODeadlines = DeadlineSet{
		{Label: "72h", Duration: 72 * time.Hour},
	}
)

// ComputeDeadlines calculates absolute deadline timestamps for each definition
// in defs, anchored to startAt (typically the incident discovery time).
// The returned slice preserves the order of defs.
func ComputeDeadlines(startAt time.Time, defs DeadlineSet) []DeadlineWindow {
	windows := make([]DeadlineWindow, len(defs))
	for i, d := range defs {
		windows[i] = DeadlineWindow{
			Label:      d.Label,
			DeadlineAt: startAt.Add(d.Duration),
		}
	}
	return windows
}

// AmpelStatus computes a traffic-light status string for the given set of
// deadline windows at the point in time now.
//
// Rules (evaluated against the nearest unreported deadline):
//   - "green"  — all deadlines are reported OR the nearest unreported deadline
//     is still more than 12 hours away
//   - "yellow" — the nearest unreported deadline is within 12 hours but not yet
//     past
//   - "red"    — at least one unreported deadline is already past
//
// If windows is empty, "green" is returned.
func AmpelStatus(windows []DeadlineWindow, now time.Time) string {
	status := "green"
	for _, w := range windows {
		if w.ReportedAt != nil {
			// This window is already fulfilled — skip.
			continue
		}
		if now.After(w.DeadlineAt) {
			// At least one deadline is overdue — worst possible status.
			return "red"
		}
		hoursLeft := w.DeadlineAt.Sub(now).Hours()
		if hoursLeft <= 12 {
			// Approaching but not yet overdue — escalate to yellow unless we
			// already know it's red (handled above).
			status = "yellow"
		}
	}
	return status
}
