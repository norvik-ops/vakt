package vaktcomply

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/matharnica/vakt/internal/modules/vaktcomply/policy"
	"github.com/rs/zerolog/log"
)

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
		controls[i].Status = policy.ResolveStatus(controls[i])
		if strings.HasPrefix(controls[i].ControlID, "DORA-") {
			if m, ok := policy.DoraISO27001Mapping[controls[i].ControlID]; ok {
				controls[i].ISO27001Mapping = m
			}
		}
	}
	return controls, total, nil
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

// ListControlsCursor returns controls using keyset pagination.
func (s *Service) ListControlsCursor(ctx context.Context, orgID, frameworkID string, cursorControlID, cursorID string, limit int) ([]Control, error) {
	return s.repo.ListControlsCursor(ctx, orgID, frameworkID, cursorControlID, cursorID, limit)
}

type CryptoKey struct {
	ID                   string    `json:"id"`
	OrgID                string    `json:"org_id"`
	Name                 string    `json:"name"`
	KeyType              string    `json:"key_type"`
	Algorithm            string    `json:"algorithm"`
	KeyLength            *int      `json:"key_length,omitempty"`
	Purpose              string    `json:"purpose"`
	Location             string    `json:"location,omitempty"`
	RotationIntervalDays *int      `json:"rotation_interval_days,omitempty"`
	LastRotationDate     *string   `json:"last_rotation_date,omitempty"`
	NextRotationDue      *string   `json:"next_rotation_due,omitempty"`
	ExpiryDate           *string   `json:"expiry_date,omitempty"`
	IsWeakAlgorithm      bool      `json:"is_weak_algorithm"`
	RotationStatus       string    `json:"rotation_status"` // ok | due_soon | overdue | none
	Notes                string    `json:"notes,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// CreateCryptoKeyInput is the validated input for creating a crypto key record.
type CreateCryptoKeyInput struct {
	Name                 string  `json:"name"      validate:"required,max=255"`
	KeyType              string  `json:"key_type"  validate:"required,oneof=symmetric asymmetric certificate hmac signing other"`
	Algorithm            string  `json:"algorithm" validate:"required,max=100"`
	KeyLength            *int    `json:"key_length,omitempty"`
	Purpose              string  `json:"purpose"   validate:"required,max=500"`
	Location             string  `json:"location"`
	RotationIntervalDays *int    `json:"rotation_interval_days,omitempty"`
	LastRotationDate     *string `json:"last_rotation_date,omitempty"`
	ExpiryDate           *string `json:"expiry_date,omitempty"`
	Notes                string  `json:"notes"`
}

var weakAlgorithms = []string{"MD5", "SHA-1", "SHA1", "DES", "3DES", "RC4", "RC2"}
var weakKeyLengths = map[string]int{"RSA": 2048, "DSA": 2048}

// IsWeakAlgorithm detects known-insecure algorithms and key lengths.
func IsWeakAlgorithm(algorithm string, keyLength *int) bool {
	upper := strings.ToUpper(algorithm)
	for _, w := range weakAlgorithms {
		if strings.Contains(upper, strings.ToUpper(w)) {
			return true
		}
	}
	if keyLength != nil {
		for prefix, minLen := range weakKeyLengths {
			if strings.HasPrefix(upper, prefix) && *keyLength < minLen {
				return true
			}
		}
	}
	return false
}

// computeRotationStatus returns ok | due_soon | overdue | none based on next_rotation_due.
func computeRotationStatus(nextRotationDue *string) string {
	if nextRotationDue == nil || *nextRotationDue == "" {
		return "none"
	}
	t, err := time.Parse("2006-01-02", *nextRotationDue)
	if err != nil {
		return "none"
	}
	now := time.Now().UTC()
	if t.Before(now) {
		return "overdue"
	}
	if t.Before(now.Add(30 * 24 * time.Hour)) {
		return "due_soon"
	}
	return "ok"
}

// ListCryptoKeys returns all crypto keys for an org.
func (s *Service) ListCryptoKeys(ctx context.Context, orgID string) ([]CryptoKey, error) {
	keys, err := s.repo.ListCryptoKeys(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list crypto keys: %w", err)
	}
	for i := range keys {
		keys[i].RotationStatus = computeRotationStatus(keys[i].NextRotationDue)
	}
	return keys, nil
}

// CreateCryptoKey creates a new crypto key record.
func (s *Service) CreateCryptoKey(ctx context.Context, orgID string, in CreateCryptoKeyInput) (*CryptoKey, error) {
	weak := IsWeakAlgorithm(in.Algorithm, in.KeyLength)

	var nextDue *string
	if in.RotationIntervalDays != nil && in.LastRotationDate != nil {
		t, err := time.Parse("2006-01-02", *in.LastRotationDate)
		if err == nil {
			nd := t.AddDate(0, 0, *in.RotationIntervalDays).Format("2006-01-02")
			nextDue = &nd
		}
	}

	key, err := s.repo.CreateCryptoKey(ctx, orgID, in, weak, nextDue)
	if err != nil {
		return nil, fmt.Errorf("create crypto key: %w", err)
	}
	key.RotationStatus = computeRotationStatus(key.NextRotationDue)
	return key, nil
}

// RecordKeyRotation records a manual key rotation event.
func (s *Service) RecordKeyRotation(ctx context.Context, orgID, keyID string) (*CryptoKey, error) {
	today := time.Now().UTC().Format("2006-01-02")

	key, err := s.repo.GetCryptoKey(ctx, orgID, keyID)
	if err != nil {
		return nil, fmt.Errorf("get crypto key: %w", err)
	}

	var nextDue *string
	if key.RotationIntervalDays != nil {
		t, _ := time.Parse("2006-01-02", today)
		nd := t.AddDate(0, 0, *key.RotationIntervalDays).Format("2006-01-02")
		nextDue = &nd
	}

	updated, err := s.repo.RecordKeyRotation(ctx, orgID, keyID, today, nextDue)
	if err != nil {
		return nil, fmt.Errorf("record key rotation: %w", err)
	}
	updated.RotationStatus = computeRotationStatus(updated.NextRotationDue)
	return updated, nil
}

// DeleteCryptoKey removes a crypto key record.
func (s *Service) DeleteCryptoKey(ctx context.Context, orgID, keyID string) error {
	return s.repo.DeleteCryptoKey(ctx, orgID, keyID)
}

func (s *Service) CreateAuditorLink(ctx context.Context, orgID, frameworkID, userID string, expiresIn time.Duration, maxUses *int) (string, error) {
	rawToken, tokenHash, err := policy.GenerateToken()
	if err != nil {
		return "", fmt.Errorf("generate auditor token: %w", err)
	}

	expiresAt := time.Now().UTC().Add(expiresIn)
	_, err = s.repo.CreateAuditorLink(ctx, orgID, frameworkID, userID, tokenHash, expiresAt, maxUses)
	if err != nil {
		return "", fmt.Errorf("create auditor link: %w", err)
	}

	return rawToken, nil
}

// ValidateAuditorLink looks up an auditor link by its raw token, increments usage,
// and returns the associated framework.
func (s *Service) ValidateAuditorLink(ctx context.Context, rawToken string) (*Framework, error) {
	sum := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(sum[:])

	al, err := s.repo.GetAuditorLinkByHash(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("invalid auditor link")
	}

	if time.Now().UTC().After(al.ExpiresAt) {
		return nil, fmt.Errorf("auditor link expired")
	}
	if al.MaxUses != nil && al.UsedCount >= *al.MaxUses {
		return nil, fmt.Errorf("auditor link usage limit reached")
	}

	if err := s.repo.IncrementAuditorLinkUsage(ctx, al.ID); err != nil {
		log.Warn().Err(err).Str("link_id", al.ID).Msg("failed to increment auditor link usage")
	}

	return s.repo.GetFramework(ctx, al.OrgID, al.FrameworkID)
}

// validateAuditorToken resolves a raw token to an AuditorLink, enforcing expiry and revocation.
// Returns the internal AuditorLink (not exposed to callers directly).
func (s *Service) validateAuditorToken(ctx context.Context, rawToken string) (*AuditorLink, error) {
	sum := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(sum[:])

	al, err := s.repo.GetAuditorLinkByHash(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("invalid auditor link")
	}
	if time.Now().UTC().After(al.ExpiresAt) {
		return nil, fmt.Errorf("auditor link expired")
	}
	if al.MaxUses != nil && al.UsedCount >= *al.MaxUses {
		return nil, fmt.Errorf("auditor link usage limit reached")
	}
	// Update access tracking (best-effort).
	if err := s.repo.UpdateAuditorLinkAccess(ctx, al.ID); err != nil {
		log.Warn().Err(err).Str("link_id", al.ID).Msg("failed to update auditor link access")
	}
	return al, nil
}

// PreflightAuditorExport validates a token and returns the framework name without
// incrementing the access counter. Used by the handler to set Content-Disposition
// before streaming the ZIP body (ExportAuditorBundle increments on its own call).
func (s *Service) PreflightAuditorExport(ctx context.Context, rawToken string) (string, error) {
	sum := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(sum[:])

	al, err := s.repo.GetAuditorLinkByHash(ctx, tokenHash)
	if err != nil {
		return "", fmt.Errorf("invalid auditor link")
	}
	if time.Now().UTC().After(al.ExpiresAt) {
		return "", fmt.Errorf("auditor link expired")
	}

	fw, err := s.repo.GetFramework(ctx, al.OrgID, al.FrameworkID)
	if err != nil {
		return "", fmt.Errorf("get framework: %w", err)
	}
	return fw.Name, nil
}

// ListAuditorLinks returns all auditor links for the given organisation.
func (s *Service) ListAuditorLinks(ctx context.Context, orgID string) ([]AuditorLinkListItem, error) {
	links, err := s.repo.ListAuditorLinks(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list auditor links: %w", err)
	}
	return links, nil
}

// RevokeAuditorLink marks an auditor link as revoked so it can no longer be used.
func (s *Service) RevokeAuditorLink(ctx context.Context, orgID, linkID string) error {
	if err := s.repo.RevokeAuditorLink(ctx, orgID, linkID); err != nil {
		return fmt.Errorf("revoke auditor link: %w", err)
	}
	return nil
}

// AuditorViewDetailed validates the token and returns the framework, readiness report,
// and each control with its evidence items — for the enhanced auditor portal (E09.2).
func (s *Service) AuditorViewDetailed(ctx context.Context, rawToken string) (*AuditorDetailView, error) {
	al, err := s.validateAuditorToken(ctx, rawToken)
	if err != nil {
		return nil, err
	}

	fw, err := s.repo.GetFramework(ctx, al.OrgID, al.FrameworkID)
	if err != nil {
		return nil, fmt.Errorf("get framework: %w", err)
	}

	controls, err := s.repo.ListControls(ctx, al.OrgID, al.FrameworkID)
	if err != nil {
		return nil, fmt.Errorf("list controls: %w", err)
	}

	evidenceCounts, err := s.repo.CountEvidenceByControl(ctx, al.OrgID, al.FrameworkID)
	if err != nil {
		return nil, fmt.Errorf("count evidence: %w", err)
	}

	report := policy.ComputeReadinessReport(fw, controls, evidenceCounts)

	// Collect all control IDs for a single batch query instead of N per-control queries.
	controlIDs := make([]string, len(controls))
	for i, c := range controls {
		controlIDs[i] = c.ID
	}
	evidenceByControl, err := s.repo.ListEvidenceByControls(ctx, al.OrgID, controlIDs)
	if err != nil {
		return nil, fmt.Errorf("list evidence batch: %w", err)
	}

	withEvidence := make([]ControlWithEvidence, 0, len(controls))
	for i := range controls {
		c := controls[i]
		c.EvidenceCount = evidenceCounts[c.ID]
		c.Status = policy.ResolveStatus(c)

		items := evidenceByControl[c.ID]
		if items == nil {
			items = []Evidence{}
		}
		withEvidence = append(withEvidence, ControlWithEvidence{
			Control:  c,
			Evidence: items,
		})
	}

	return &AuditorDetailView{
		Framework: *fw,
		Report:    report,
		Controls:  withEvidence,
	}, nil
}

// ExportAuditorBundle validates the token and writes a ZIP to w with structure:
//
//	<framework_name>/
//	  <domain>/
//	    <control_code>/
//	      evidence_metadata.json
func (s *Service) ExportAuditorBundle(ctx context.Context, rawToken string, w io.Writer) (string, error) {
	al, err := s.validateAuditorToken(ctx, rawToken)
	if err != nil {
		return "", err
	}

	fw, err := s.repo.GetFramework(ctx, al.OrgID, al.FrameworkID)
	if err != nil {
		return "", fmt.Errorf("get framework: %w", err)
	}

	controls, err := s.repo.ListControls(ctx, al.OrgID, al.FrameworkID)
	if err != nil {
		return "", fmt.Errorf("list controls: %w", err)
	}

	// Batch-load all evidence in one query before writing the ZIP.
	controlIDs := make([]string, len(controls))
	for i, c := range controls {
		controlIDs[i] = c.ID
	}
	evidenceByControl, err := s.repo.ListEvidenceByControls(ctx, al.OrgID, controlIDs)
	if err != nil {
		return "", fmt.Errorf("list evidence batch: %w", err)
	}

	zw := zip.NewWriter(w)
	defer func() { _ = zw.Close() }()

	for i := range controls {
		c := controls[i]
		items := evidenceByControl[c.ID]
		if items == nil {
			items = []Evidence{}
		}

		path := fmt.Sprintf("%s/%s/%s/evidence_metadata.json", fw.Name, c.Domain, c.ControlID)
		f, err := zw.Create(path)
		if err != nil {
			return "", fmt.Errorf("create zip entry %s: %w", path, err)
		}
		meta := EvidenceMetadata{Control: c, Evidence: items}
		if err := json.NewEncoder(f).Encode(meta); err != nil {
			return "", fmt.Errorf("encode metadata for %s: %w", c.ControlID, err)
		}
	}

	if err := zw.Close(); err != nil {
		return "", fmt.Errorf("close zip: %w", err)
	}

	return fw.Name, nil
}

// auditControlEntry groups evidence items under a single control for the audit export.
type auditControlEntry struct {
	ControlID    string // control_id column value, e.g. "A.5.1"
	ControlTitle string
	Evidence     []EvidenceForExport
}

// ExportAuditPackage erstellt ein ZIP-Archiv mit allen Compliance-Nachweisen für ein Framework.
// Die ZIP enthält:
//   - INDEX.pdf    — Übersicht aller Controls mit Status und Evidence-Liste
//   - summary.json — maschinenlesbare Zusammenfassung
//   - evidence/    — Ordner pro Control mit je einer Textdatei pro Evidence
func (s *Service) ExportAuditPackage(ctx context.Context, orgID, frameworkID string) (zipData []byte, filename string, err error) {
	// 1. Load framework metadata.
	fw, err := s.repo.GetFramework(ctx, orgID, frameworkID)
	if err != nil {
		return nil, "", fmt.Errorf("get framework: %w", err)
	}

	// 2. Load org name.
	var orgName string
	orgName = fetchOrgName(ctx, s.db, orgID)
	if orgName == "" {
		orgName = orgID
	}

	// 3. Load all evidence + control metadata in a single query.
	items, err := s.repo.ListEvidenceForFramework(ctx, orgID, frameworkID)
	if err != nil {
		return nil, "", fmt.Errorf("list evidence for framework: %w", err)
	}

	// 4. Build per-control groupings.
	var controlOrder []string
	controlMap := make(map[string]*auditControlEntry)
	evidenceTotal := 0

	for i := range items {
		item := &items[i]
		if _, seen := controlMap[item.ControlID]; !seen {
			controlOrder = append(controlOrder, item.ControlID)
			controlMap[item.ControlID] = &auditControlEntry{
				ControlID:    item.ControlDomain,
				ControlTitle: item.ControlTitle,
			}
		}
		if item.EvidenceID != "" {
			controlMap[item.ControlID].Evidence = append(controlMap[item.ControlID].Evidence, *item)
			evidenceTotal++
		}
	}

	controlsWithEvidence := 0
	for _, ce := range controlMap {
		if len(ce.Evidence) > 0 {
			controlsWithEvidence++
		}
	}
	controlsTotal := len(controlOrder)

	// 5. Generate INDEX.pdf.
	indexPDF, err := GenerateAuditIndexPDF(fw.Name, orgName, controlOrder, controlMap, time.Now())
	if err != nil {
		return nil, "", fmt.Errorf("generate index pdf: %w", err)
	}

	// 6. Build summary.json.
	type summaryJSON struct {
		Framework               string    `json:"framework"`
		Org                     string    `json:"org"`
		ExportedAt              time.Time `json:"exported_at"`
		ControlsTotal           int       `json:"controls_total"`
		ControlsWithEvidence    int       `json:"controls_with_evidence"`
		ControlsWithoutEvidence int       `json:"controls_without_evidence"`
		EvidenceTotal           int       `json:"evidence_total"`
	}
	summaryData, err := json.Marshal(summaryJSON{
		Framework:               fw.Name,
		Org:                     orgName,
		ExportedAt:              time.Now().UTC(),
		ControlsTotal:           controlsTotal,
		ControlsWithEvidence:    controlsWithEvidence,
		ControlsWithoutEvidence: controlsTotal - controlsWithEvidence,
		EvidenceTotal:           evidenceTotal,
	})
	if err != nil {
		return nil, "", fmt.Errorf("marshal summary: %w", err)
	}

	// 7. Assemble ZIP.
	exportDate := time.Now().UTC().Format("2006-01-02")
	safeName := strings.Map(func(r rune) rune {
		if r == ' ' {
			return '-'
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return -1
	}, fw.Name)
	filename = fmt.Sprintf("audit-package-%s-%s.zip", safeName, exportDate)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// INDEX.pdf
	if f, zipErr := zw.Create("INDEX.pdf"); zipErr == nil {
		_, _ = f.Write(indexPDF)
	}

	// summary.json
	if f, zipErr := zw.Create("summary.json"); zipErr == nil {
		_, _ = f.Write(summaryData)
	}

	// evidence/ folder — one .txt file per evidence item.
	for _, ctrlID := range controlOrder {
		ce := controlMap[ctrlID]
		if len(ce.Evidence) == 0 {
			continue
		}
		folderName := auditSanitizePath(ce.ControlID + "-" + ce.ControlTitle)
		for i, ev := range ce.Evidence {
			entryName := fmt.Sprintf("evidence/%s/evidence_%03d.txt", folderName, i+1)
			f, zipErr := zw.Create(entryName)
			if zipErr != nil {
				continue
			}
			_, _ = fmt.Fprintf(f, "Evidence: %s\n", ev.EvidenceTitle)
			_, _ = fmt.Fprintf(f, "Control: %s — %s\n", ce.ControlID, ce.ControlTitle)
			_, _ = fmt.Fprintf(f, "Source: %s\n", ev.EvidenceSource)
			_, _ = fmt.Fprintf(f, "Collected: %s\n", ev.CollectedAt.UTC().Format("2006-01-02 15:04 UTC"))
			if ev.EvidenceDesc != "" {
				_, _ = fmt.Fprintf(f, "\nDescription:\n%s\n", ev.EvidenceDesc)
			}
			if ev.EvidenceFilePath != "" {
				_, _ = fmt.Fprintf(f, "\nFile reference: %s\n", ev.EvidenceFilePath)
			}
		}
	}

	if err := zw.Close(); err != nil {
		return nil, "", fmt.Errorf("close zip: %w", err)
	}
	return buf.Bytes(), filename, nil
}

// auditSanitizePath removes characters unsafe for ZIP entry paths.
func auditSanitizePath(s string) string {
	if len(s) > 60 {
		s = s[:60]
	}
	return strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		}
		return r
	}, s)
}

func needsSeed(templates []Questionnaire) bool {
	return len(templates) == 0
}

// SeedBuiltinQuestionnaires creates the 3 built-in questionnaire templates if they don't exist.
// Idempotent: does nothing if templates are already present.
func (s *Service) SeedBuiltinQuestionnaires(ctx context.Context, orgID string) error {
	isTemplate := true
	existing, err := s.repo.ListQuestionnaires(ctx, orgID, &isTemplate)
	if err != nil {
		return fmt.Errorf("seed questionnaires: list existing: %w", err)
	}
	if !needsSeed(existing) {
		return nil
	}

	type templateDef struct {
		name      string
		questions []string
	}
	templates := []templateDef{
		{
			name: "NIS2 Lieferanten-Assessment",
			questions: []string{
				"Netzwerksicherheit",
				"Zugriffskontrollen",
				"Incident-Response",
				"Backup",
				"Patch-Management",
				"Supply-Chain-Checks",
				"Kryptographie",
				"Physische Sicherheit",
				"Personalschulungen",
				"Auditlogs",
			},
		},
		{
			name: "DORA IKT-Drittanbieter",
			questions: []string{
				"IKT-Risikomanagement",
				"Incident-Klassifizierung",
				"Resilienztests",
				"Drittanbieter-Verträge",
				"Informationsaustausch",
				"Wiederherstellungstests",
				"Aufsichtsmeldung",
				"Kontrollrahmen",
			},
		},
		{
			name: "ISO 27001 Basischeck",
			questions: []string{
				"Asset-Inventar",
				"Risikobehandlung",
				"Zugriffsrechte",
				"Kryptographie",
				"Lieferantensicherheit",
				"Compliance",
				"Awareness",
				"Audit",
				"Business-Continuity",
				"HR-Sicherheit",
				"Physische Kontrollen",
				"Kommunikationssicherheit",
			},
		},
	}

	for _, t := range templates {
		q, err := s.repo.CreateQuestionnaire(ctx, orgID, t.name, "", true)
		if err != nil {
			return fmt.Errorf("seed questionnaire %q: %w", t.name, err)
		}
		for _, text := range t.questions {
			if _, err := s.repo.CreateQuestion(ctx, q.ID, text, "yes_no", nil, true, nil); err != nil {
				return fmt.Errorf("seed question %q: %w", text, err)
			}
		}
	}
	return nil
}

// ListTemplates seeds built-in templates (if needed) then returns all templates.
func (s *Service) ListTemplates(ctx context.Context, orgID string) ([]Questionnaire, error) {
	if err := s.SeedBuiltinQuestionnaires(ctx, orgID); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("seed built-in questionnaires")
	}
	isTemplate := true
	templates, err := s.repo.ListQuestionnaires(ctx, orgID, &isTemplate)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	// Load questions for each template.
	for i := range templates {
		questions, err := s.repo.ListQuestions(ctx, templates[i].ID)
		if err != nil {
			return nil, fmt.Errorf("list template questions: %w", err)
		}
		templates[i].Questions = questions
	}
	return templates, nil
}

// ListQuestionnaires returns questionnaires optionally filtered by is_template.
func (s *Service) ListQuestionnaires(ctx context.Context, orgID string, isTemplate *bool) ([]Questionnaire, error) {
	return s.repo.ListQuestionnaires(ctx, orgID, isTemplate)
}

// GetQuestionnaire returns a single questionnaire with its questions.
func (s *Service) GetQuestionnaire(ctx context.Context, orgID, id string) (*Questionnaire, error) {
	return s.repo.GetQuestionnaire(ctx, orgID, id)
}

// CreateQuestionnaire creates a new questionnaire, cloning from a source if CloneFromID is set.
func (s *Service) CreateQuestionnaire(ctx context.Context, orgID string, in CreateQuestionnaireInput) (*Questionnaire, error) {
	if in.CloneFromID != "" {
		return s.CloneQuestionnaire(ctx, orgID, in.CloneFromID, in.Name)
	}
	return s.repo.CreateQuestionnaire(ctx, orgID, in.Name, in.Description, in.IsTemplate)
}

// CloneQuestionnaire copies a questionnaire and all its questions.
func (s *Service) CloneQuestionnaire(ctx context.Context, orgID, sourceID, name string) (*Questionnaire, error) {
	return s.repo.CloneQuestionnaire(ctx, orgID, sourceID, name)
}

// UpdateQuestionnaire updates questionnaire metadata.
func (s *Service) UpdateQuestionnaire(ctx context.Context, orgID, id string, in UpdateQuestionnaireInput) (*Questionnaire, error) {
	return s.repo.UpdateQuestionnaire(ctx, orgID, id, in.Name, in.Description, in.IsTemplate)
}

// DeleteQuestionnaire removes a questionnaire.
func (s *Service) DeleteQuestionnaire(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteQuestionnaire(ctx, orgID, id)
}

// AddQuestion adds a question to a questionnaire.
// For multiple_choice type, options must be non-empty.
func (s *Service) AddQuestion(ctx context.Context, orgID, questionnaireID string, in CreateQuestionInput) (*Question, error) {
	if in.QuestionType == "multiple_choice" && len(in.Options) == 0 {
		return nil, fmt.Errorf("multiple_choice question requires non-empty options")
	}
	// Verify org owns the questionnaire.
	if _, err := s.repo.GetQuestionnaire(ctx, orgID, questionnaireID); err != nil {
		return nil, fmt.Errorf("questionnaire not found or access denied: %w", err)
	}
	var controlID *string
	if in.ControlID != "" {
		controlID = &in.ControlID
	}
	return s.repo.CreateQuestion(ctx, questionnaireID, in.QuestionText, in.QuestionType, in.Options, in.Required, controlID)
}

// UpdateQuestion updates an existing question.
func (s *Service) UpdateQuestion(ctx context.Context, orgID, questionnaireID, questionID string, in CreateQuestionInput) (*Question, error) {
	if in.QuestionType == "multiple_choice" && len(in.Options) == 0 {
		return nil, fmt.Errorf("multiple_choice question requires non-empty options")
	}
	if _, err := s.repo.GetQuestionnaire(ctx, orgID, questionnaireID); err != nil {
		return nil, fmt.Errorf("questionnaire not found or access denied: %w", err)
	}
	var controlID *string
	if in.ControlID != "" {
		controlID = &in.ControlID
	}
	return s.repo.UpdateQuestion(ctx, questionnaireID, questionID, in.QuestionText, in.QuestionType, in.Options, in.Required, controlID)
}

// DeleteQuestion removes a question from a questionnaire.
func (s *Service) DeleteQuestion(ctx context.Context, orgID, questionnaireID, questionID string) error {
	if _, err := s.repo.GetQuestionnaire(ctx, orgID, questionnaireID); err != nil {
		return fmt.Errorf("questionnaire not found or access denied: %w", err)
	}
	return s.repo.DeleteQuestion(ctx, questionnaireID, questionID)
}

// ReorderQuestions updates the order of questions in a questionnaire.
func (s *Service) ReorderQuestions(ctx context.Context, orgID, questionnaireID string, order []string) error {
	if _, err := s.repo.GetQuestionnaire(ctx, orgID, questionnaireID); err != nil {
		return fmt.Errorf("questionnaire not found or access denied: %w", err)
	}
	return s.repo.ReorderQuestions(ctx, questionnaireID, order)
}

var defaultInterestedParties = []CreateInterestedPartyInput{
	{
		Name:         "Kunden",
		Category:     "customer",
		Requirements: "Vertraulichkeit und Integrität ihrer Daten; Verfügbarkeit der angebotenen Dienste",
	},
	{
		Name:         "Datenschutzbehörde / BSI",
		Category:     "regulator",
		Requirements: "Einhaltung DSGVO, NIS2 und branchenspezifischer Regulierung; Meldepflichten bei Vorfällen",
	},
	{
		Name:         "Geschäftsführung / Eigentümer",
		Category:     "shareholder",
		Requirements: "Risikominimierung, Geschäftskontinuität, Reputationsschutz; Return on Security Investment",
	},
	{
		Name:         "Mitarbeiter",
		Category:     "employee",
		Requirements: "Klare Sicherheitsrichtlinien; sichere Arbeitsumgebung; Datenschutz der eigenen Daten",
	},
	{
		Name:         "Lieferanten und Auftragnehmer",
		Category:     "supplier",
		Requirements: "Klare vertragliche Sicherheitsanforderungen; Auditrechte; Incident-Reporting-Pflichten",
	},
	{
		Name:         "Cyber-Versicherung",
		Category:     "insurer",
		Requirements: "Nachweis angemessener Sicherheitsmaßnahmen; Incident-Reporting innerhalb vereinbarter Fristen",
	},
}

// ListInterestedParties returns all interested parties for the org.
func (s *Service) ListInterestedParties(ctx context.Context, orgID string) ([]InterestedParty, error) {
	return s.repo.ListInterestedParties(ctx, orgID)
}

// CreateInterestedParty persists a new interested party entry.
func (s *Service) CreateInterestedParty(ctx context.Context, orgID string, in CreateInterestedPartyInput) (*InterestedParty, error) {
	return s.repo.CreateInterestedParty(ctx, orgID, in, false)
}

// UpdateInterestedParty modifies an existing entry.
func (s *Service) UpdateInterestedParty(ctx context.Context, orgID, id string, in CreateInterestedPartyInput) (*InterestedParty, error) {
	return s.repo.UpdateInterestedParty(ctx, orgID, id, in)
}

// DeleteInterestedParty removes an entry.
func (s *Service) DeleteInterestedParty(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteInterestedParty(ctx, orgID, id)
}

// SeedDefaultInterestedParties inserts the 6 default stakeholders if the org has none.
// It is idempotent — calling it again when entries exist is a no-op.
func (s *Service) SeedDefaultInterestedParties(ctx context.Context, orgID string) error {
	count, err := s.repo.CountInterestedParties(ctx, orgID)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	for _, tmpl := range defaultInterestedParties {
		if _, err := s.repo.CreateInterestedParty(ctx, orgID, tmpl, true); err != nil {
			return fmt.Errorf("seed interested party %q: %w", tmpl.Name, err)
		}
	}
	return nil
}

// GetClause42Status returns true if Clause 4.2 is considered fulfilled (≥3 entries with requirements).
func (s *Service) GetClause42Status(ctx context.Context, orgID string) (bool, error) {
	return s.repo.CheckClause42Fulfilled(ctx, orgID)
}

// ExportInterestedPartiesPDF generates an audit-ready PDF of all interested parties.
func (s *Service) ExportInterestedPartiesPDF(ctx context.Context, orgID string) ([]byte, error) {
	parties, err := s.repo.ListInterestedParties(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return buildInterestedPartiesPDF(parties)
}

func buildInterestedPartiesPDF(parties []InterestedParty) ([]byte, error) {
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(12, 12, 12)
	pdf.SetAutoPageBreak(true, 12)
	exportedAt := time.Now().UTC()

	pdf.SetFooterFunc(func() {
		pdf.SetY(-10)
		pdf.SetFont("Helvetica", "I", 7)
		pdf.SetTextColor(150, 150, 160)
		pdf.CellFormat(0, 5, fmt.Sprintf("Vakt — Interessierte Parteien (ISO 27001 Clause 4.2) — %s — Seite %d/{nb}", exportedAt.Format("02.01.2006"), pdf.PageNo()), "", 0, "C", false, 0, "")
	})
	pdf.AliasNbPages("{nb}")

	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetTextColor(30, 30, 30)
	pdf.CellFormat(0, 10, "Interessierte Parteien — ISO 27001 Clause 4.2", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(100, 100, 110)
	pdf.CellFormat(0, 6, fmt.Sprintf("Erstellt: %s", exportedAt.Format("02. January 2006")), "", 1, "L", false, 0, "")
	pdf.Ln(4)

	headers := []string{"Name", "Kategorie", "Anforderungen / Erwartungen", "Anliegen / Risiken", "Überprüfung"}
	colW := []float64{50, 35, 85, 70, 27}

	pdf.SetFont("Helvetica", "B", 8)
	pdf.SetFillColor(45, 55, 72)
	pdf.SetTextColor(255, 255, 255)
	for i, h := range headers {
		pdf.CellFormat(colW[i], 6, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	catLabels := map[string]string{
		"customer": "Kunden", "regulator": "Behörde", "employee": "Mitarbeiter",
		"shareholder": "Eigentümer", "supplier": "Lieferant", "insurer": "Versicherung",
		"it_provider": "IT-Dienstleister", "other": "Sonstige",
	}
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(30, 30, 30)

	for _, p := range parties {
		cat := catLabels[p.Category]
		if cat == "" {
			cat = p.Category
		}
		reviewDate := ""
		if p.ReviewDate != nil {
			reviewDate = *p.ReviewDate
		}
		pdf.SetFillColor(255, 255, 255)
		pdf.CellFormat(colW[0], 5, p.Name, "1", 0, "L", false, 0, "")
		pdf.CellFormat(colW[1], 5, cat, "1", 0, "L", false, 0, "")
		pdf.CellFormat(colW[2], 5, policy.Truncate(p.Requirements, 55), "1", 0, "L", false, 0, "")
		pdf.CellFormat(colW[3], 5, policy.Truncate(p.Concerns, 45), "1", 0, "L", false, 0, "")
		pdf.CellFormat(colW[4], 5, reviewDate, "1", 1, "C", false, 0, "")
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
