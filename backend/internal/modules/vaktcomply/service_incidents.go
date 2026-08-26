// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"

	"github.com/matharnica/vakt/internal/shared/notify"
)

// Gründe, aus denen eine Meldefrist-E-Mail nicht zugestellt wurde.
//
// R1-19-04: Beide Fälle waren vorher ein wortloses `return` aus einer Closure
// ohne Rückgabewert. Der Aufrufer schrieb danach unbeirrt „overdue
// notification sent" ins Log — eine gesetzliche Meldefrist, deren Erinnerung
// niemanden erreicht, sah im Log aus wie eine zugestellte.
var (
	// errNoNotifier: der Dienst wurde ohne Benachrichtigungsdienst gebaut.
	// Das ist ein Verdrahtungsfehler, kein Betriebszustand — deshalb ein
	// Fehler und kein stiller Normalfall.
	errNoNotifier = errors.New("kein Benachrichtigungsdienst verdrahtet")
	// errNoAdminRecipients: die Organisation hat keine Administrator-Adresse.
	errNoAdminRecipients = errors.New("keine Administrator-Adresse gefunden")
)

// deliverAdminEmail stellt eine Nachricht an alle Administratoren einer
// Organisation zu und meldet, OB das gelungen ist.
//
// Rückgabe nil bedeutet: mindestens ein Administrator hat die Nachricht
// bekommen. Konnte keiner erreicht werden, ist das ein Fehler — der Aufrufer
// darf daraus keinen Erfolg machen.
//
// Eine Teilzustellung (einer von drei scheitert) gilt als Erfolg, wird aber
// geloggt: Die Erinnerung hat ihr Ziel erreicht, die kaputte Adresse ist ein
// getrenntes Problem.
//
// Die Funktion nimmt den Benachrichtigungsdienst als Parameter statt ihn aus
// dem Service zu lesen, damit sie ohne Datenbank prüfbar ist.
func deliverAdminEmail(ctx context.Context, notifSvc notifyService, orgID string, admins []string, subject, body string) error {
	if notifSvc == nil {
		return errNoNotifier
	}
	if len(admins) == 0 {
		return errNoAdminRecipients
	}

	var delivered int
	var errs []error
	for _, email := range admins {
		if err := notifSvc.Notify(ctx, notify.Message{
			Title:   subject,
			Body:    body,
			OrgID:   orgID,
			Channel: notify.ChannelEmail,
			Target:  email,
		}); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", email, err))
			continue
		}
		delivered++
	}

	if delivered == 0 {
		return fmt.Errorf("keine Zustellung gelungen: %w", errors.Join(errs...))
	}
	if len(errs) > 0 {
		log.Warn().Errs("fehler", errs).Str("org_id", orgID).Int("zugestellt", delivered).
			Msg("deadline_check: E-Mail nur teilweise zugestellt")
	}
	return nil
}

// authorityYAMLEntry mirrors one entry in authorities.yaml.
type authorityYAMLEntry struct {
	Key        string   `yaml:"key"`
	Name       string   `yaml:"name"`
	Portal     string   `yaml:"portal"`
	Phone      string   `yaml:"phone"`
	SubmitNote string   `yaml:"submit_note"`
	Sectors    []string `yaml:"sectors"`
}

// authorityYAMLFile is the top-level structure of db/seeds/authorities.yaml.
type authorityYAMLFile struct {
	Authorities []authorityYAMLEntry `yaml:"authorities"`
}

var (
	yamlAuthoritiesOnce  sync.Once
	yamlAuthoritiesCache []AuthorityInfo
)

// LoadAuthoritiesFromYAML reads db/seeds/authorities.yaml once and returns the list.
// Falls back to the in-memory directory if the file is not found.
// The seed path is resolved relative to the binary's working directory; in
// production the file is included in the Docker image at /app/db/seeds/authorities.yaml.
func LoadAuthoritiesFromYAML() []AuthorityInfo {
	yamlAuthoritiesOnce.Do(func() {
		candidates := []string{
			"db/seeds/authorities.yaml",
			"/app/db/seeds/authorities.yaml",
		}
		var data []byte
		for _, path := range candidates {
			b, err := os.ReadFile(path)
			if err == nil {
				data = b
				break
			}
		}
		if data == nil {
			log.Warn().Msg("authorities.yaml not found; using in-memory authority directory")
			yamlAuthoritiesCache = ListAllAuthorities()
			return
		}
		var file authorityYAMLFile
		if err := yaml.Unmarshal(data, &file); err != nil {
			log.Error().Err(err).Msg("failed to parse authorities.yaml; using in-memory directory")
			yamlAuthoritiesCache = ListAllAuthorities()
			return
		}
		out := make([]AuthorityInfo, 0, len(file.Authorities))
		for _, e := range file.Authorities {
			out = append(out, AuthorityInfo{
				Name:       e.Name,
				Portal:     e.Portal,
				Phone:      e.Phone,
				SubmitNote: e.SubmitNote,
			})
		}
		yamlAuthoritiesCache = out
	})
	return yamlAuthoritiesCache
}

// --- Incident Register (FR-CK13) ---

func (s *Service) ListIncidents(ctx context.Context, orgID string) ([]Incident, error) {
	incidents, err := s.repo.ListIncidents(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list incidents: %w", err)
	}
	if incidents == nil {
		incidents = []Incident{}
	}
	for i := range incidents {
		incidents[i].DeadlineStatus = computeDeadlineStatus(&incidents[i])
	}
	return incidents, nil
}

func (s *Service) GetIncident(ctx context.Context, orgID, id string) (*Incident, error) {
	inc, err := s.repo.GetIncident(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	inc.DeadlineStatus = computeDeadlineStatus(inc)
	return inc, nil
}

func (s *Service) CreateIncident(ctx context.Context, orgID string, in CreateIncidentInput) (*Incident, error) {
	if in.AffectedSystems == nil {
		in.AffectedSystems = []string{}
	}
	deadlines := computeDeadlines(in.IncidentType, in.DiscoveredAt)
	inc, err := s.repo.CreateIncident(ctx, orgID, in, deadlines)
	if err != nil {
		return nil, err
	}
	inc.DeadlineStatus = computeDeadlineStatus(inc)
	s.triggerWebhook(ctx, orgID, "incident.created", map[string]any{
		"id":       inc.ID,
		"title":    inc.Title,
		"severity": inc.Severity,
		"status":   inc.Status,
		"org_id":   orgID,
	})
	return inc, nil
}

func (s *Service) UpdateIncident(ctx context.Context, orgID, id string, in UpdateIncidentInput) (*Incident, error) {
	if in.AffectedSystems == nil {
		in.AffectedSystems = []string{}
	}
	inc, err := s.repo.UpdateIncident(ctx, orgID, id, in)
	if err != nil {
		return nil, err
	}
	inc.DeadlineStatus = computeDeadlineStatus(inc)
	if in.Status != "" {
		s.triggerWebhook(ctx, orgID, "incident.status_changed", map[string]any{
			"id":       inc.ID,
			"title":    inc.Title,
			"severity": inc.Severity,
			"status":   inc.Status,
			"org_id":   orgID,
		})
	}
	return inc, nil
}

func (s *Service) MarkDeadlineReported(ctx context.Context, orgID, id, deadline string) (*Incident, error) {
	inc, err := s.repo.MarkDeadlineReported(ctx, orgID, id, deadline)
	if err != nil {
		return nil, err
	}
	inc.DeadlineStatus = computeDeadlineStatus(inc)
	return inc, nil
}

// AssessReportability evaluates NIS2 meldepflicht based on a short questionnaire,
// persists the answers, and updates reporting_obligation + notification_authority.
func (s *Service) AssessReportability(ctx context.Context, orgID, incidentID string, in AssessReportabilityInput) (*ReportabilityResult, error) {
	var obligation, explanation string
	switch {
	case in.AffectsEssentialService:
		obligation = "required"
		explanation = "Essenzieller Dienst betroffen — NIS2-Meldepflicht wahrscheinlich (§ 32 BSIG-neu)."
	case in.AffectsExternalData:
		obligation = "unknown"
		explanation = "Externe Kundendaten betroffen, aber kein essenzieller Dienst identifiziert — bitte rechtlich prüfen."
	default:
		obligation = "not_required"
		explanation = "Keine Hinweise auf NIS2-Meldepflicht nach aktuellem Bewertungsstand."
	}

	authority := s.primaryAuthorityForOrg(ctx, orgID)

	answersJSON, err := json.Marshal(in.ReportabilityAnswers)
	if err != nil {
		return nil, fmt.Errorf("marshal reportability answers: %w", err)
	}
	if err := s.repo.UpdateIncidentReportability(ctx, orgID, incidentID, obligation, authority, in.PersonalDataCompromised, answersJSON); err != nil {
		return nil, err
	}
	return &ReportabilityResult{
		Obligation:            obligation,
		GDPRRequired:          in.PersonalDataCompromised,
		NotificationAuthority: authority,
		Explanation:           explanation,
		Answers:               in.ReportabilityAnswers,
	}, nil
}

// CheckOverdueDeadlines iterates all DORA/NIS2 incidents for the given org and
// sends in-app and e-mail notifications for overdue or soon-due deadlines.
// The 12h-before warning is guarded by notified_warn_* flags to prevent repeats.
// It is called by the vaktcomply:incident_deadline_check cron job.
func (s *Service) CheckOverdueDeadlines(ctx context.Context, orgID string) error {
	now := time.Now().UTC()

	// Fetch admin e-mails once per org run (non-fatal if lookup fails).
	adminEmails, _ := s.repo.GetAdminEmails(ctx, orgID)

	// sendEmail stellt an alle Administratoren zu und meldet das Ergebnis.
	// Der Rückgabewert ist der Grund, aus dem der Aufrufer NICHT „sent"
	// loggen darf (R1-19-04).
	sendEmail := func(subject, body string) error {
		return deliverAdminEmail(ctx, s.notifSvc, orgID, adminEmails, subject, body)
	}

	// Check both DORA and NIS2 incident types.
	for _, incType := range []string{"dora", "nis2"} {
		incidents, err := s.repo.ListIncidentsByType(ctx, orgID, incType)
		if err != nil {
			return fmt.Errorf("list %s incidents: %w", incType, err)
		}

		type deadlinePair struct {
			deadline    *time.Time
			reportedAt  *time.Time
			label       string
			warnAlready bool // true if 12h warning already sent
		}

		for i := range incidents {
			inc := &incidents[i]
			pairs := []deadlinePair{
				{inc.Deadline24h, inc.Reported24hAt, "24h", inc.NotifiedWarn24h},
				{inc.Deadline72h, inc.Reported72hAt, "72h", inc.NotifiedWarn72h},
				{inc.Deadline30d, inc.Reported30dAt, "30d", inc.NotifiedWarn30d},
			}
			for _, p := range pairs {
				if p.deadline == nil || p.reportedAt != nil {
					continue
				}
				hoursLeft := p.deadline.Sub(now).Hours()
				if now.After(*p.deadline) {
					// Overdue — in-app notification (sent every cron run until reported).
					var notifTitle, notifType string
					switch incType {
					case "nis2":
						notifTitle = fmt.Sprintf("NIS2-Meldefrist überschritten: %s", inc.Title)
						notifType = "nis2_deadline_overdue"
					default:
						notifTitle = fmt.Sprintf("DORA-Meldefrist überschritten: %s", inc.Title)
						notifType = "dora_deadline_overdue"
					}
					body := fmt.Sprintf(
						"Die %s-Meldefrist für den Vorfall \"%s\" wurde überschritten und ist noch nicht als gemeldet markiert.",
						p.label, inc.Title,
					)
					// Keine Marke — diese Meldung wiederholt sich absichtlich
					// bei jedem Lauf, bis der Vorfall als gemeldet gilt. Der
					// Fehler darf trotzdem nicht unsichtbar sein.
					if err := notify.Send(ctx, s.db, orgID, notifTitle, body, notifType, "vaktcomply"); err != nil {
						log.Error().Err(err).Str("org_id", orgID).Str("incident_id", inc.ID).Str("deadline", p.label).
							Msg("incident_deadline_check: Meldefrist überschritten — In-App-Meldung NICHT geschrieben")
					}
					emailSubj := fmt.Sprintf("[Vakt Comply] %s", notifTitle)
					if err := sendEmail(emailSubj, body); err != nil {
						log.Error().Err(err).Str("org_id", orgID).Str("incident_id", inc.ID).Str("deadline", p.label).
							Msg("incident_deadline_check: Meldefrist überschritten — E-Mail an die Administratoren NICHT versendet")
					} else {
						log.Warn().Str("org_id", orgID).Str("incident_id", inc.ID).Str("deadline", p.label).
							Msg("incident_deadline_check: overdue notification sent")
					}
				} else if hoursLeft <= 12 && !p.warnAlready {
					// 12h-before warning — sent exactly once (guarded by notified_warn_* flag).
					var notifTitle, notifType string
					switch incType {
					case "nis2":
						notifTitle = fmt.Sprintf("NIS2-Meldefrist in %.0fh: %s", hoursLeft, inc.Title)
						notifType = "nis2_deadline_warning"
					default:
						notifTitle = fmt.Sprintf("DORA-Meldefrist in %.0fh: %s", hoursLeft, inc.Title)
						notifType = "dora_deadline_warning"
					}
					body := fmt.Sprintf(
						"Die %s-Meldefrist für den Vorfall \"%s\" läuft in %.0f Stunden ab.",
						p.label, inc.Title, hoursLeft,
					)
					inAppErr := notify.Send(ctx, s.db, orgID, notifTitle, body, notifType, "vaktcomply")
					emailSubj := fmt.Sprintf("[Vakt Comply] %s", notifTitle)
					mailErr := sendEmail(emailSubj, body)
					// R1-W4A-N1: Die Marke schliesst jede Wiederholung aus
					// (die Auswahl prueft notified_warn_*). Hier stand, dass
					// sie bewusst auch ohne erfolgreichen Versand gesetzt
					// wird, weil eine Bindung an die E-Mail bei dauerhaft
					// kaputtem SMTP jede Warnung endlos wiederholen wuerde.
					// Der Einwand stimmt, die Schlussfolgerung nicht: Die
					// Marke gehoert an „mindestens EIN Kanal hat zugestellt".
					// Ein kaputter Mailversand allein wiederholt dann nichts,
					// weil die In-App-Meldung ankam — aber wenn KEIN Kanal
					// erreicht wurde, bleibt die Warnung wiederholbar, statt
					// nach einem einmaligen Ausfall dauerhaft zu verstummen.
					if inAppErr != nil && mailErr != nil {
						log.Error().Str("org_id", orgID).Str("incident_id", inc.ID).Str("deadline", p.label).
							Msg("incident_deadline_check: 12h-Warnung über KEINEN Kanal zugestellt — Marke bleibt ungesetzt, der naechste Lauf versucht es erneut")
					} else if err := s.repo.MarkIncidentWarnNotified(ctx, orgID, inc.ID, p.label); err != nil {
						log.Warn().Err(err).Str("incident_id", inc.ID).Str("deadline", p.label).
							Msg("incident_deadline_check: failed to mark warn notified")
					}
					if inAppErr != nil {
						log.Error().Err(inAppErr).Str("org_id", orgID).Str("incident_id", inc.ID).Str("deadline", p.label).
							Msg("incident_deadline_check: 12h-Warnung — In-App-Meldung NICHT geschrieben")
					}
					if mailErr != nil {
						log.Error().Err(mailErr).Str("org_id", orgID).Str("incident_id", inc.ID).Str("deadline", p.label).
							Msg("incident_deadline_check: 12h-Warnung — E-Mail an die Administratoren NICHT versendet")
					} else {
						log.Info().Str("org_id", orgID).Str("incident_id", inc.ID).Str("deadline", p.label).
							Msg("incident_deadline_check: 12h warning sent")
					}
				}
			}
		}
	}
	return nil
}

// GenerateIncidentReportForm generates a NIS2 Meldungsformular PDF and saves it
// in the ck_incident_reports archive. Returns the archived report and raw PDF bytes.
func (s *Service) GenerateIncidentReportForm(ctx context.Context, orgID, incidentID, reportType, orgName string) (*IncidentReport, []byte, error) {
	inc, err := s.repo.GetIncident(ctx, orgID, incidentID)
	if err != nil {
		return nil, nil, err
	}
	if reportType != "24h" && reportType != "72h" && reportType != "30d" {
		return nil, nil, fmt.Errorf("invalid report_type: %s", reportType)
	}

	pdfBytes, err := GenerateNIS2ReportFormPDF(inc, reportType, orgName)
	if err != nil {
		return nil, nil, fmt.Errorf("generate nis2 report form pdf: %w", err)
	}

	authority := inc.NotificationAuthority
	if authority == "" {
		authority = "BSI"
	}

	meta, _ := json.Marshal(map[string]string{
		"incident_title": inc.Title,
		"report_type":    reportType,
		"authority":      authority,
	})

	report, err := s.repo.SaveIncidentReport(ctx, orgID, incidentID, reportType, authority, pdfBytes, meta)
	if err != nil {
		return nil, nil, err
	}
	return report, pdfBytes, nil
}

// ListIncidentReports returns all archived Meldungsformulare for an incident.
func (s *Service) ListIncidentReports(ctx context.Context, orgID, incidentID string) ([]IncidentReport, error) {
	return s.repo.ListIncidentReports(ctx, orgID, incidentID)
}

// GetIncidentReportPDF returns the stored PDF bytes for a specific report.
func (s *Service) GetIncidentReportPDF(ctx context.Context, orgID, reportID string) ([]byte, error) {
	return s.repo.GetIncidentReportPDF(ctx, orgID, reportID)
}

// GetAuthorityInfo returns submission channel info for a given authority key.
func GetAuthorityInfo(authority string) (AuthorityInfo, bool) {
	info, ok := incidentAuthorityDirectory[authority]
	return info, ok
}

// GetOrgSector returns the sector and federal state configured for the org.
func (s *Service) GetOrgSector(ctx context.Context, orgID string) (*OrgSectorSettings, error) {
	return s.repo.GetOrgSector(ctx, orgID)
}

// UpdateOrgSector sets the org's sector and federal state.
func (s *Service) UpdateOrgSector(ctx context.Context, orgID string, in UpdateOrgSectorInput) (*OrgSectorSettings, error) {
	if err := s.repo.UpdateOrgSector(ctx, orgID, in.Sector, in.FederalState); err != nil {
		return nil, err
	}
	return s.repo.GetOrgSector(ctx, orgID)
}

// GetAuthoritiesForOrg returns the relevant NIS2 authorities for the org's configured sector.
func (s *Service) GetAuthoritiesForOrg(ctx context.Context, orgID string) ([]AuthorityInfo, error) {
	settings, err := s.repo.GetOrgSector(ctx, orgID)
	if err != nil {
		// Fallback to BSI if org lookup fails.
		return []AuthorityInfo{incidentAuthorityDirectory["BSI"]}, nil
	}
	keys, ok := sectorAuthorityMap[settings.Sector]
	if !ok {
		keys = []string{"BSI"}
	}
	var infos []AuthorityInfo
	for _, k := range keys {
		if info, exists := incidentAuthorityDirectory[k]; exists {
			infos = append(infos, info)
		}
	}
	return infos, nil
}

// ListAllAuthorities returns all known reporting authorities.
func ListAllAuthorities() []AuthorityInfo {
	order := []string{"BSI", "BaFin", "BNetzA", "LBA"}
	var all []AuthorityInfo
	for _, k := range order {
		if info, ok := incidentAuthorityDirectory[k]; ok {
			all = append(all, info)
		}
	}
	return all
}

// ClassifyReportingObligation runs the S39-1 3-question wizard, persists the result to
// classification_result JSONB, and also computes + stores reporting_deadlines.
func (s *Service) ClassifyReportingObligation(ctx context.Context, orgID, incidentID string, in ClassifyReportingInput) (*ClassificationResult, error) {
	// Determine obligation level.
	var obligation, reason string
	switch {
	case in.EssentialService:
		obligation = "probably"
		reason = "Essenzieller Dienst betroffen — NIS2-Meldepflicht wahrscheinlich (§ 32 BSIG-neu)."
	case in.CustomerData:
		obligation = "unclear"
		reason = "Kundendaten betroffen, aber kein essenzieller Dienst identifiziert — rechtliche Prüfung empfohlen."
	case in.PersonalData:
		obligation = "unclear"
		reason = "Personenbezogene Daten betroffen — DSGVO-Meldepflicht (72h) prüfen; NIS2-Pflicht hängt vom Dienst ab."
	default:
		obligation = "none"
		reason = "Keine Hinweise auf NIS2-Meldepflicht nach aktuellem Bewertungsstand."
	}

	// Determine authority from org sector.
	authority := s.primaryAuthorityForOrg(ctx, orgID)
	// S39-1 spec: sector "finanz" → "BaFin+BSI"
	settings, _ := s.repo.GetOrgSector(ctx, orgID)
	if settings != nil && settings.Sector == "finance" {
		authority = "BaFin+BSI"
	}
	// Personal data always adds LDA note.
	if in.PersonalData && authority != "" {
		// Keep primary authority, append LDA hint in reason.
		reason += " Datenschutzbehörde (LDA) innerhalb von 72h informieren (DSGVO Art. 33)."
	}

	result := ClassificationResult{
		Obligation: obligation,
		Authority:  authority,
		Reason:     reason,
	}

	if err := s.repo.SaveClassificationResult(ctx, orgID, incidentID, result); err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("classify reporting obligation: %w", err)
	}
	return &result, nil
}

// CheckNIS2ObligationDeadlines checks incidents whose classification_result.obligation = "probably"
// and fires notifications for upcoming or overdue NIS2 deadlines. S39-2.
func (s *Service) CheckNIS2ObligationDeadlines(ctx context.Context, orgID string) error {
	now := time.Now().UTC()

	incidents, err := s.repo.ListNIS2ClassifiedIncidents(ctx, orgID)
	if err != nil {
		return fmt.Errorf("nis2_obligation_check: list incidents: %w", err)
	}

	adminEmails, _ := s.repo.GetAdminEmails(ctx, orgID)
	// Siehe deliverAdminEmail — der Rückgabewert ist der Grund, aus dem der
	// Aufrufer NICHT „sent" loggen darf (R1-19-04).
	sendEmail := func(subject, body string) error {
		return deliverAdminEmail(ctx, s.notifSvc, orgID, adminEmails, subject, body)
	}

	type deadlinePair struct {
		deadline    *time.Time
		reportedAt  *time.Time
		label       string
		warnAlready bool
	}

	for i := range incidents {
		inc := &incidents[i]
		pairs := []deadlinePair{
			{inc.Deadline24h, inc.Reported24hAt, "24h", inc.NotifiedWarn24h},
			{inc.Deadline72h, inc.Reported72hAt, "72h", inc.NotifiedWarn72h},
			{inc.Deadline30d, inc.Reported30dAt, "30d", inc.NotifiedWarn30d},
		}
		for _, p := range pairs {
			if p.deadline == nil || p.reportedAt != nil {
				continue
			}
			hoursLeft := p.deadline.Sub(now).Hours()
			if now.After(*p.deadline) {
				title := fmt.Sprintf("NIS2-Meldefrist überschritten: %s", inc.Title)
				body := fmt.Sprintf(
					"Die %s-Meldefrist für den Vorfall \"%s\" (Meldepflicht wahrscheinlich) ist überschritten und noch nicht als gemeldet markiert.",
					p.label, inc.Title,
				)
				// Keine Marke — wiederholt sich absichtlich bei jedem Lauf.
				if err := notify.Send(ctx, s.db, orgID, title, body, "nis2_obligation_overdue", "vaktcomply"); err != nil {
					log.Error().Err(err).Str("org_id", orgID).Str("incident_id", inc.ID).Str("deadline", p.label).
						Msg("nis2_obligation_check: Meldefrist überschritten — In-App-Meldung NICHT geschrieben")
				}
				if err := sendEmail(fmt.Sprintf("[Vakt Comply] %s", title), body); err != nil {
					log.Error().Err(err).Str("org_id", orgID).Str("incident_id", inc.ID).Str("deadline", p.label).
						Msg("nis2_obligation_check: Meldefrist überschritten — E-Mail an die Administratoren NICHT versendet")
				} else {
					log.Warn().Str("org_id", orgID).Str("incident_id", inc.ID).Str("deadline", p.label).
						Msg("nis2_obligation_check: overdue notification sent")
				}
			} else if hoursLeft <= 12 && !p.warnAlready {
				title := fmt.Sprintf("NIS2-Meldefrist in %.0fh: %s", hoursLeft, inc.Title)
				body := fmt.Sprintf(
					"Die %s-Meldefrist für den Vorfall \"%s\" (Meldepflicht wahrscheinlich) läuft in %.0f Stunden ab.",
					p.label, inc.Title, hoursLeft,
				)
				inAppErr := notify.Send(ctx, s.db, orgID, title, body, "nis2_obligation_warning", "vaktcomply")
				mailErr := sendEmail(fmt.Sprintf("[Vakt Comply] %s", title), body)
				// R1-W4A-N1: Marke nur, wenn mindestens ein Kanal zugestellt
				// hat — Begruendung im gleichlautenden Block in
				// CheckOverdueDeadlines.
				if inAppErr != nil && mailErr != nil {
					log.Error().Str("org_id", orgID).Str("incident_id", inc.ID).Str("deadline", p.label).
						Msg("nis2_obligation_check: 12h-Warnung über KEINEN Kanal zugestellt — Marke bleibt ungesetzt, der naechste Lauf versucht es erneut")
				} else if err := s.repo.MarkIncidentWarnNotified(ctx, orgID, inc.ID, p.label); err != nil {
					log.Warn().Err(err).Str("incident_id", inc.ID).Msg("nis2_obligation_check: failed to mark warn notified")
				}
				if inAppErr != nil {
					log.Error().Err(inAppErr).Str("org_id", orgID).Str("incident_id", inc.ID).Str("deadline", p.label).
						Msg("nis2_obligation_check: 12h-Warnung — In-App-Meldung NICHT geschrieben")
				}
				if mailErr != nil {
					log.Error().Err(mailErr).Str("org_id", orgID).Str("incident_id", inc.ID).Str("deadline", p.label).
						Msg("nis2_obligation_check: 12h-Warnung — E-Mail an die Administratoren NICHT versendet")
				} else {
					log.Info().Str("org_id", orgID).Str("incident_id", inc.ID).Str("deadline", p.label).
						Msg("nis2_obligation_check: 12h warning sent")
				}
			}
		}
	}
	return nil
}

// primaryAuthorityForOrg returns the first authority for the org's sector (used in reportability assessment).
func (s *Service) primaryAuthorityForOrg(ctx context.Context, orgID string) string {
	settings, err := s.repo.GetOrgSector(ctx, orgID)
	if err != nil {
		return "BSI"
	}
	keys, ok := sectorAuthorityMap[settings.Sector]
	if !ok || len(keys) == 0 {
		return "BSI"
	}
	return keys[0]
}

// computeDeadlines calculates absolute deadline timestamps for NIS2 and DORA incident types.
func computeDeadlines(incidentType string, discoveredAt time.Time) map[string]*time.Time {
	result := map[string]*time.Time{"4h": nil, "24h": nil, "72h": nil, "30d": nil}
	switch incidentType {
	case "dora":
		t4h := discoveredAt.Add(4 * time.Hour)
		t24h := discoveredAt.Add(24 * time.Hour)
		t72h := discoveredAt.Add(72 * time.Hour)
		t30d := discoveredAt.AddDate(0, 0, 30)
		result["4h"] = &t4h
		result["24h"] = &t24h
		result["72h"] = &t72h
		result["30d"] = &t30d
	case "nis2":
		t24h := discoveredAt.Add(24 * time.Hour)
		t72h := discoveredAt.Add(72 * time.Hour)
		t30d := discoveredAt.AddDate(0, 0, 30)
		result["24h"] = &t24h
		result["72h"] = &t72h
		result["30d"] = &t30d
	}
	return result
}

// computeDeadlineStatus builds the computed deadline status for a given incident.
func computeDeadlineStatus(inc *Incident) *IncidentDeadlineStatus {
	if inc.Deadline4h == nil && inc.Deadline24h == nil && inc.Deadline72h == nil && inc.Deadline30d == nil {
		return nil
	}
	now := time.Now().UTC()
	status := &IncidentDeadlineStatus{
		Has4h:  inc.Deadline4h != nil,
		Has24h: inc.Deadline24h != nil,
		Has72h: inc.Deadline72h != nil,
		Has30d: inc.Deadline30d != nil,
	}
	if inc.Deadline4h != nil {
		status.D4h = deadlineInfo(inc.Deadline4h, inc.Reported4hAt, now)
	}
	if inc.Deadline24h != nil {
		status.D24h = deadlineInfo(inc.Deadline24h, inc.Reported24hAt, now)
	}
	if inc.Deadline72h != nil {
		status.D72h = deadlineInfo(inc.Deadline72h, inc.Reported72hAt, now)
	}
	if inc.Deadline30d != nil {
		status.D30d = deadlineInfo(inc.Deadline30d, inc.Reported30dAt, now)
	}
	return status
}

func deadlineInfo(deadline, reportedAt *time.Time, now time.Time) *DeadlineInfo {
	info := &DeadlineInfo{
		Deadline:   deadline,
		ReportedAt: reportedAt,
		HoursLeft:  deadline.Sub(now).Hours(),
	}
	if reportedAt != nil {
		info.Status = "done"
	} else if now.After(*deadline) {
		info.Status = "red"
	} else if info.HoursLeft <= 6 {
		info.Status = "yellow"
	} else {
		info.Status = "green"
	}
	return info
}

// UpdateDORADeadlineStatus recomputes the DORA Ampel-Status for all DORA IKT-incidents
// in one org and persists it to dora_deadline_status JSONB. S37-4.
func (s *Service) UpdateDORADeadlineStatus(ctx context.Context, orgID string) error {
	now := time.Now().UTC()
	incidents, err := s.repo.ListIncidentsByType(ctx, orgID, "ikt_dora")
	if err != nil {
		// Fall back to legacy type name "dora".
		incidents, err = s.repo.ListIncidentsByType(ctx, orgID, "dora")
		if err != nil {
			return fmt.Errorf("dora_deadline_status: list incidents: %w", err)
		}
	}

	for i := range incidents {
		inc := &incidents[i]

		// Use first_detected_at if set, otherwise discovered_at.
		detectedAt := inc.DiscoveredAt
		// (first_detected_at is stored as dora_classification["first_detected_at"] in JSONB or deadline_4h-1h)
		// For now derive from existing Deadline24h if set: detectedAt = deadline_24h - 24h.
		if inc.Deadline24h != nil {
			derived := inc.Deadline24h.Add(-24 * time.Hour)
			detectedAt = derived
		}

		type deadlineEntry struct {
			deadline   time.Time
			reportedAt *time.Time
			key        string
		}
		entries := []deadlineEntry{
			{detectedAt.Add(24 * time.Hour), inc.Reported24hAt, "h24"},
			{detectedAt.Add(72 * time.Hour), inc.Reported72hAt, "h72"},
			{detectedAt.Add(30 * 24 * time.Hour), inc.Reported30dAt, "d30"},
		}

		status := make(map[string]string, 3)
		for _, e := range entries {
			if e.reportedAt != nil {
				status[e.key] = "done"
				continue
			}
			hoursLeft := e.deadline.Sub(now).Hours()
			switch {
			case now.After(e.deadline):
				status[e.key] = "red"
			case hoursLeft <= 6:
				status[e.key] = "yellow"
			default:
				status[e.key] = "green"
			}
		}

		if err := s.repo.UpdateIncidentDORADeadlineStatus(ctx, inc.ID, status); err != nil {
			log.Warn().Err(err).Str("incident_id", inc.ID).Msg("dora_deadline_status: update failed")
		}

		// Fire alert when any deadline goes red.
		//
		// L3-01: hier stand notify.Send. Dieser Cron laeuft alle fuenf Minuten
		// (scheduler.go), also 288-mal am Tag; mal drei Fristen sind das 864
		// identische Meldungen pro Tag und Vorfall. SendOnce meldet je Vorfall
		// und Frist genau einmal — der Schluessel steht in
		// user_notifications.module.
		for key, st := range status {
			if st == "red" {
				title := fmt.Sprintf("DORA IKT-Meldefrist überschritten (%s): %s", key, inc.Title)
				body := fmt.Sprintf("Die DORA-Meldefrist (%s) für Vorfall \"%s\" ist überschritten und noch nicht gemeldet.", key, inc.Title)
				// Die Entdopplung steckt in SendOnce selbst (Schluessel in
				// user_notifications.module), nicht in einer Marke — ein
				// Fehlschlag unterdrueckt hier nichts dauerhaft, der naechste
				// Lauf in fuenf Minuten versucht es erneut. Trotzdem geloggt.
				if err := notify.SendOnce(ctx, s.db, orgID, title, body,
					"dora_deadline_overdue", "dora_deadline:"+inc.ID+":"+key); err != nil {
					log.Error().Err(err).Str("org_id", orgID).Str("incident_id", inc.ID).Str("deadline", key).
						Msg("dora_deadline_status: In-App-Meldung NICHT geschrieben")
				}
			}
		}
	}
	return nil
}
