// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvitals

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
)

// --- Maßnahmen-Katalog (control measures) ---

// ListMeasures returns all measures for a control.
func (s *Service) ListMeasures(ctx context.Context, orgID, controlID string) ([]ControlMeasure, error) {
	return s.repo.ListMeasures(ctx, orgID, controlID)
}

// CreateMeasure creates a new custom measure for a control.
func (s *Service) CreateMeasure(ctx context.Context, orgID, controlID string, in CreateMeasureInput) (ControlMeasure, error) {
	return s.repo.CreateMeasure(ctx, orgID, controlID, in)
}

// UpdateMeasure updates an existing measure.
func (s *Service) UpdateMeasure(ctx context.Context, orgID, measureID string, in UpdateMeasureInput) (ControlMeasure, error) {
	return s.repo.UpdateMeasure(ctx, orgID, measureID, in)
}

// DeleteMeasure deletes a non-builtin measure.
func (s *Service) DeleteMeasure(ctx context.Context, orgID, measureID string) error {
	return s.repo.DeleteMeasure(ctx, orgID, measureID)
}

// SeedBuiltinMeasures seeds the default recommended measures for important ISO 27001 controls
// across all organisations. Called on startup after ReseedBuiltinControls.
func (s *Service) SeedBuiltinMeasures(ctx context.Context) {
	orgs, err := s.repo.ListAllOrgs(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("seed measures: failed to list orgs")
		return
	}

	catalogue := builtinMeasures()

	for _, orgID := range orgs {
		for controlCode, measures := range catalogue {
			controlUUID, err := s.repo.FindControlByCode(ctx, orgID, controlCode)
			if err != nil {
				log.Warn().Err(err).Str("control", controlCode).Str("org_id", orgID).Msg("seed measures: find control")
				continue
			}
			if controlUUID == "" {
				// Control not yet seeded for this org — skip silently.
				continue
			}
			if err := s.repo.SeedMeasuresForControl(ctx, orgID, controlUUID, measures); err != nil {
				log.Warn().Err(err).Str("control", controlCode).Str("org_id", orgID).Msg("seed measures: insert")
			}
		}
		log.Info().Str("org_id", orgID).Msg("seeded builtin measures")
	}
}

// builtinMeasures returns the catalogue of recommended measures keyed by ISO 27001 control_id code.
func builtinMeasures() map[string][]CreateMeasureInput {
	m := func(title, desc, diff string) CreateMeasureInput {
		return CreateMeasureInput{Title: title, Description: desc, Difficulty: diff}
	}
	return map[string][]CreateMeasureInput{
		// A.5.1 — Informationssicherheitsrichtlinien
		"A.5.1": {
			m("Richtliniendokument erstellen", "Erstellen Sie ein zentrales IS-Richtliniendokument mit Geltungsbereich, Verantwortlichkeiten und Grundsätzen. Vorlage: Mindestens 3 Seiten, jährlich überprüft.", "easy"),
			m("Freigabe durch Geschäftsführung einholen", "Lassen Sie die Richtlinie formal durch die Geschäftsführung genehmigen und unterschreiben. Dokumentieren Sie das Datum der Genehmigung.", "easy"),
			m("Richtlinie kommunizieren", "Verteilen Sie die Richtlinie an alle Mitarbeiter (z.B. per E-Mail, Intranet). Dokumentieren Sie den Versand als Nachweis.", "easy"),
		},
		// A.5.1.1 — same measures apply to the sub-control
		"A.5.1.1": {
			m("Richtliniendokument erstellen", "Erstellen Sie ein zentrales IS-Richtliniendokument mit Geltungsbereich, Verantwortlichkeiten und Grundsätzen. Vorlage: Mindestens 3 Seiten, jährlich überprüft.", "easy"),
			m("Freigabe durch Geschäftsführung einholen", "Lassen Sie die Richtlinie formal durch die Geschäftsführung genehmigen und unterschreiben. Dokumentieren Sie das Datum der Genehmigung.", "easy"),
			m("Richtlinie kommunizieren", "Verteilen Sie die Richtlinie an alle Mitarbeiter (z.B. per E-Mail, Intranet). Dokumentieren Sie den Versand als Nachweis.", "easy"),
		},
		// A.5.24 — Planung und Vorbereitung des IS-Vorfallmanagements
		"A.5.24": {
			m("Incident-Response-Plan erstellen", "Definieren Sie klare Eskalationswege, Kontaktlisten und Erstmaßnahmen für Sicherheitsvorfälle.", "medium"),
			m("Meldepflichten dokumentieren", "Dokumentieren Sie gesetzliche Meldepflichten (NIS2: 24h Erstmeldung, BSI: 72h DSGVO). Erstellen Sie eine Meldecheckliste.", "medium"),
			m("Übung durchführen", "Führen Sie mindestens jährlich eine Tabletop-Übung für einen fiktiven Vorfall durch. Protokollieren Sie die Ergebnisse.", "hard"),
		},
		// A.6.3 — Informationssicherheitsbewusstsein
		"A.6.3": {
			m("Awareness-Training planen", "Planen Sie ein jährliches Pflichttraining für alle Mitarbeiter. Nutzen Sie SecReflex für Phishing-Simulationen.", "easy"),
			m("Schulungsnachweis führen", "Dokumentieren Sie Teilnahme und Datum jeder Schulung pro Mitarbeiter als Compliance-Nachweis.", "easy"),
		},
		// A.8.8 — Management technischer Schwachstellen
		"A.8.8": {
			m("Schwachstellen-Scanner einrichten", "Richten Sie regelmäßige automatische Scans ein (z.B. Trivy für Container, Nuclei für Web-Apps). Nutzen Sie SecPulse.", "medium"),
			m("Patch-Prozess definieren", "Legen Sie SLAs für Patches fest: Kritisch ≤24h, Hoch ≤7d, Mittel ≤30d. Dokumentieren Sie Ausnahmen.", "medium"),
			m("Schwachstellen-Register pflegen", "Führen Sie ein aktuelles Register aller bekannten Schwachstellen mit Status und Verantwortlichem.", "easy"),
		},
		// A.12.6 / A.12.6.1 — Management technischer Schwachstellen (ältere ISO-Nummerierung)
		"A.12.6": {
			m("Schwachstellen-Scanner einrichten", "Richten Sie regelmäßige automatische Scans ein (z.B. Trivy für Container, Nuclei für Web-Apps). Nutzen Sie SecPulse.", "medium"),
			m("Patch-Prozess definieren", "Legen Sie SLAs für Patches fest: Kritisch ≤24h, Hoch ≤7d, Mittel ≤30d. Dokumentieren Sie Ausnahmen.", "medium"),
			m("Schwachstellen-Register pflegen", "Führen Sie ein aktuelles Register aller bekannten Schwachstellen mit Status und Verantwortlichem.", "easy"),
		},
		"A.12.6.1": {
			m("Schwachstellen-Scanner einrichten", "Richten Sie regelmäßige automatische Scans ein (z.B. Trivy für Container, Nuclei für Web-Apps). Nutzen Sie SecPulse.", "medium"),
			m("Patch-Prozess definieren", "Legen Sie SLAs für Patches fest: Kritisch ≤24h, Hoch ≤7d, Mittel ≤30d. Dokumentieren Sie Ausnahmen.", "medium"),
			m("Schwachstellen-Register pflegen", "Führen Sie ein aktuelles Register aller bekannten Schwachstellen mit Status und Verantwortlichem.", "easy"),
		},
		// A.8.13 — Informationssicherung (Backup)
		"A.8.13": {
			m("Backup-Konzept erstellen", "Dokumentieren Sie Backup-Frequenz (täglich), Aufbewahrungszeit und Speicherorte (3-2-1-Regel).", "easy"),
			m("Wiederherstellung testen", "Testen Sie mindestens jährlich die Wiederherstellung aus Backups. Protokollieren Sie RPO und RTO.", "medium"),
		},
		// A.12.3 / A.12.3.1 — Datensicherung (ältere ISO-Nummerierung)
		"A.12.3": {
			m("Backup-Konzept erstellen", "Dokumentieren Sie Backup-Frequenz (täglich), Aufbewahrungszeit und Speicherorte (3-2-1-Regel).", "easy"),
			m("Wiederherstellung testen", "Testen Sie mindestens jährlich die Wiederherstellung aus Backups. Protokollieren Sie RPO und RTO.", "medium"),
		},
		"A.12.3.1": {
			m("Backup-Konzept erstellen", "Dokumentieren Sie Backup-Frequenz (täglich), Aufbewahrungszeit und Speicherorte (3-2-1-Regel).", "easy"),
			m("Wiederherstellung testen", "Testen Sie mindestens jährlich die Wiederherstellung aus Backups. Protokollieren Sie RPO und RTO.", "medium"),
		},
		// A.8.16 — Überwachungsaktivitäten
		"A.8.16": {
			m("Log-Management einrichten", "Zentralisieren Sie System- und Sicherheitslogs. Definieren Sie Aufbewahrungsdauer (mind. 12 Monate für NIS2).", "medium"),
			m("Alerting konfigurieren", "Richten Sie automatische Alarme für kritische Ereignisse ein (failed logins, privilege escalation, etc.).", "medium"),
		},
		// A.5.21 — Lieferkettensicherheit
		"A.5.21": {
			m("Lieferanten-Register erstellen", "Führen Sie ein Register aller IT-Dienstleister mit Risikoeinstufung und Vertragsreferenz.", "easy"),
			m("AVV abschließen", "Stellen Sie sicher, dass alle Auftragsverarbeiter einen gültigen AVV nach Art. 28 DSGVO unterzeichnet haben.", "medium"),
			m("Lieferanten-Audit planen", "Führen Sie für kritische Lieferanten mindestens jährlich ein Sicherheits-Assessment durch.", "hard"),
		},
		// A.5.22 — Lieferkettenüberwachung
		"A.5.22": {
			m("Lieferanten-Register erstellen", "Führen Sie ein Register aller IT-Dienstleister mit Risikoeinstufung und Vertragsreferenz.", "easy"),
			m("AVV abschließen", "Stellen Sie sicher, dass alle Auftragsverarbeiter einen gültigen AVV nach Art. 28 DSGVO unterzeichnet haben.", "medium"),
			m("Lieferanten-Audit planen", "Führen Sie für kritische Lieferanten mindestens jährlich ein Sicherheits-Assessment durch.", "hard"),
		},
		// A.8.24 — Kryptographie
		"A.8.24": {
			m("Kryptokonzept erstellen", "Dokumentieren Sie erlaubte Verschlüsselungsalgorithmen, Schlüssellängen und Zertifikats-Management-Prozesse.", "medium"),
			m("Zertifikate inventarisieren", "Führen Sie eine Liste aller TLS-Zertifikate mit Ablaufdatum. Richten Sie Erneuerungs-Alerts ein.", "easy"),
		},
		// A.10.1 / A.10.1.1 / A.10.1.2 — Kryptographie (ältere ISO-Nummerierung)
		"A.10.1": {
			m("Kryptokonzept erstellen", "Dokumentieren Sie erlaubte Verschlüsselungsalgorithmen, Schlüssellängen und Zertifikats-Management-Prozesse.", "medium"),
			m("Zertifikate inventarisieren", "Führen Sie eine Liste aller TLS-Zertifikate mit Ablaufdatum. Richten Sie Erneuerungs-Alerts ein.", "easy"),
		},
		"A.10.1.1": {
			m("Kryptokonzept erstellen", "Dokumentieren Sie erlaubte Verschlüsselungsalgorithmen, Schlüssellängen und Zertifikats-Management-Prozesse.", "medium"),
			m("Zertifikate inventarisieren", "Führen Sie eine Liste aller TLS-Zertifikate mit Ablaufdatum. Richten Sie Erneuerungs-Alerts ein.", "easy"),
		},
		"A.10.1.2": {
			m("Kryptokonzept erstellen", "Dokumentieren Sie erlaubte Verschlüsselungsalgorithmen, Schlüssellängen und Zertifikats-Management-Prozesse.", "medium"),
			m("Zertifikate inventarisieren", "Führen Sie eine Liste aller TLS-Zertifikate mit Ablaufdatum. Richten Sie Erneuerungs-Alerts ein.", "easy"),
		},
	}
}

// --- Collaborative Tasks ---

// ListTasks returns all tasks for the given compliance entity.
func (s *Service) ListTasks(ctx context.Context, orgID, entityType, entityID string) ([]Task, error) {
	tasks, err := s.repo.ListTasks(ctx, orgID, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	if tasks == nil {
		tasks = []Task{}
	}
	return tasks, nil
}

// CreateTask creates a new collaborative task for a compliance entity.
func (s *Service) CreateTask(ctx context.Context, orgID, entityType, entityID string, in CreateTaskInput) (Task, error) {
	return s.repo.CreateTask(ctx, orgID, entityType, entityID, in)
}

// UpdateTask applies a partial update to a task.
func (s *Service) UpdateTask(ctx context.Context, orgID, taskID string, in UpdateTaskInput) (Task, error) {
	return s.repo.UpdateTask(ctx, orgID, taskID, in)
}

// DeleteTask removes a task.
func (s *Service) DeleteTask(ctx context.Context, orgID, taskID string) error {
	return s.repo.DeleteTask(ctx, orgID, taskID)
}

// ListOverdueTasks returns open tasks past their due date for the org.
func (s *Service) ListOverdueTasks(ctx context.Context, orgID string) ([]Task, error) {
	tasks, err := s.repo.ListOverdueTasks(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list overdue tasks: %w", err)
	}
	if tasks == nil {
		tasks = []Task{}
	}
	return tasks, nil
}

// --- Comments ---

// ListComments returns all comments for a compliance entity.
func (s *Service) ListComments(ctx context.Context, orgID, entityType, entityID string) ([]Comment, error) {
	comments, err := s.repo.ListComments(ctx, orgID, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	if comments == nil {
		comments = []Comment{}
	}
	return comments, nil
}

// CreateComment posts a comment on a compliance entity.
func (s *Service) CreateComment(ctx context.Context, orgID, entityType, entityID string, in CreateCommentInput) (Comment, error) {
	return s.repo.CreateComment(ctx, orgID, entityType, entityID, in)
}

// DeleteComment removes a comment.
func (s *Service) DeleteComment(ctx context.Context, orgID, commentID string) error {
	return s.repo.DeleteComment(ctx, orgID, commentID)
}

// --- CAPA (Corrective and Preventive Actions) ---

// ListCAPAs returns CAPAs for an organisation, optionally filtered by status.
func (s *Service) ListCAPAs(ctx context.Context, orgID string, statusFilter string) ([]CAPA, error) {
	return s.repo.ListCAPAs(ctx, orgID, statusFilter)
}

// ListCAPAsForSource returns CAPAs linked to a specific source entity.
func (s *Service) ListCAPAsForSource(ctx context.Context, orgID, sourceType, sourceID string) ([]CAPA, error) {
	return s.repo.ListCAPAsForSource(ctx, orgID, sourceType, sourceID)
}

// GetCAPA returns a single CAPA by ID.
func (s *Service) GetCAPA(ctx context.Context, orgID, capaID string) (CAPA, error) {
	return s.repo.GetCAPA(ctx, orgID, capaID)
}

// CreateCAPA creates a new CAPA record.
func (s *Service) CreateCAPA(ctx context.Context, orgID string, in CreateCAPAInput) (CAPA, error) {
	return s.repo.CreateCAPA(ctx, orgID, in)
}

// UpdateCAPA applies partial updates to a CAPA.
func (s *Service) UpdateCAPA(ctx context.Context, orgID, capaID string, in UpdateCAPAInput) (CAPA, error) {
	return s.repo.UpdateCAPA(ctx, orgID, capaID, in)
}

// DeleteCAPA removes a CAPA record.
func (s *Service) DeleteCAPA(ctx context.Context, orgID, capaID string) error {
	return s.repo.DeleteCAPA(ctx, orgID, capaID)
}

// --- Control Review Cycles (Migration 075) ---

// RecordControlReview records a periodic review event for a compliance control.
// It updates the control's review timestamps and appends a row to the review history log.
func (s *Service) RecordControlReview(ctx context.Context, orgID, controlID string, in RecordReviewInput) (Control, error) {
	// Fetch current control to capture status_at_review.
	ctrl, err := s.repo.GetControl(ctx, orgID, controlID)
	if err != nil {
		return Control{}, fmt.Errorf("get control for review: %w", err)
	}
	statusAtReview := ctrl.Status
	if statusAtReview == "" {
		statusAtReview = ctrl.ManualStatus
	}
	return s.repo.RecordControlReview(ctx, orgID, controlID, in, statusAtReview)
}

// ListControlReviews returns the review history for a control.
func (s *Service) ListControlReviews(ctx context.Context, orgID, controlID string) ([]ControlReview, error) {
	return s.repo.ListControlReviews(ctx, orgID, controlID)
}

// ListOverdueControls returns controls whose review is past due.
func (s *Service) ListOverdueControls(ctx context.Context, orgID string) ([]Control, error) {
	return s.repo.ListOverdueControls(ctx, orgID)
}

// --- Paginated list methods (used by pagination-aware handlers) ---

// ListControlsPaged returns a page of controls with evidence counts, plus the total count.
func (s *Service) ListControlsPaged(ctx context.Context, orgID, frameworkID string, offset, limit int) ([]Control, int, error) {
	controls, total, err := s.repo.ListControlsPaged(ctx, orgID, frameworkID, offset, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("list controls paged: %w", err)
	}

	// Enrich with evidence counts (using counts for the full framework so we don't need extra per-page queries).
	counts, err := s.repo.CountEvidenceByControl(ctx, orgID, frameworkID)
	if err != nil {
		return nil, 0, fmt.Errorf("count evidence for controls paged: %w", err)
	}
	for i := range controls {
		controls[i].EvidenceCount = counts[controls[i].ID]
		controls[i].Status = resolveStatus(controls[i])
		if strings.HasPrefix(controls[i].ControlID, "DORA-") {
			if m, ok := doraISO27001Mapping[controls[i].ControlID]; ok {
				controls[i].ISO27001Mapping = m
			}
		}
	}
	return controls, total, nil
}

// ListRisksPaged returns a page of risks plus the total count.
func (s *Service) ListRisksPaged(ctx context.Context, orgID string, offset, limit int) ([]Risk, int, error) {
	return s.repo.ListRisksPaged(ctx, orgID, offset, limit)
}

// ListIncidentsPaged returns a page of incidents plus the total count.
func (s *Service) ListIncidentsPaged(ctx context.Context, orgID string, offset, limit int) ([]Incident, int, error) {
	incidents, total, err := s.repo.ListIncidentsPaged(ctx, orgID, offset, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("list incidents paged: %w", err)
	}
	for i := range incidents {
		incidents[i].DeadlineStatus = computeDeadlineStatus(&incidents[i])
	}
	return incidents, total, nil
}

// ListPoliciesPaged returns a page of policies plus the total count.
func (s *Service) ListPoliciesPaged(ctx context.Context, orgID string, offset, limit int) ([]Policy, int, error) {
	return s.repo.ListPoliciesPaged(ctx, orgID, offset, limit)
}

// ListCAPAsPaged returns a page of CAPAs plus the total count.
func (s *Service) ListCAPAsPaged(ctx context.Context, orgID, statusFilter string, offset, limit int) ([]CAPA, int, error) {
	return s.repo.ListCAPAsPaged(ctx, orgID, statusFilter, offset, limit)
}
