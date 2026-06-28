package vaktcomply

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

func (s *Service) GetEvidenceHistory(ctx context.Context, orgID, evidenceID string) ([]EvidenceHistoryEntry, error) {
	return s.repo.ListEvidenceHistory(ctx, orgID, evidenceID)
}

// AddEvidence stores a new evidence item for a control.
func (s *Service) AddEvidence(ctx context.Context, orgID, controlID, userID string, input AddEvidenceInput) (*Evidence, error) {
	// Verify the control belongs to this org.
	if _, err := s.repo.GetControl(ctx, orgID, controlID); err != nil {
		return nil, fmt.Errorf("control not found: %w", err)
	}

	ev, err := s.repo.AddEvidence(ctx, orgID, controlID, userID, input)
	if err != nil {
		return nil, fmt.Errorf("add evidence: %w", err)
	}
	return ev, nil
}

// ListEvidence returns all evidence items for a control.
func (s *Service) ListEvidence(ctx context.Context, orgID, controlID string) ([]Evidence, error) {
	return s.repo.ListEvidence(ctx, orgID, controlID)
}

// ReviewEvidence updates the review status of an evidence item.
// status must be one of: "approved", "rejected".
func (s *Service) ReviewEvidence(ctx context.Context, orgID, evidenceID, reviewerID, status, _ string) error {
	if status != "approved" && status != "rejected" {
		return fmt.Errorf("invalid review status: %s (must be approved or rejected)", status)
	}
	return s.repo.ReviewEvidence(ctx, orgID, evidenceID, reviewerID, status)
}

// GetExpiringEvidenceAll returns evidence expiring within the given number of days, across all frameworks.
func (s *Service) GetExpiringEvidenceAll(ctx context.Context, orgID string, days int) ([]Evidence, error) {
	threshold := time.Now().UTC().AddDate(0, 0, days)
	items, err := s.repo.GetExpiringEvidenceAllFrameworks(ctx, orgID, threshold)
	if err != nil {
		return nil, fmt.Errorf("get expiring evidence all: %w", err)
	}
	if items == nil {
		items = []Evidence{}
	}
	return items, nil
}

// CollectEvidence runs the named collector and stores the result as an evidence item.
func (s *Service) CollectEvidence(ctx context.Context, orgID, controlID, userID string, cfg CollectorConfig) (*Evidence, error) {
	collector, err := GetCollector(cfg.Type)
	if err != nil {
		return nil, err
	}

	data, err := collector.Collect(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("collector %s: %w", cfg.Type, err)
	}

	title := fmt.Sprintf("Auto-collected: %s (%s)", cfg.Type, time.Now().UTC().Format(time.DateOnly))
	return s.repo.AddCollectorEvidence(ctx, orgID, controlID, userID, cfg.Type, title, data)
}

const TaskEvidenceStalenessCheck = "comply:evidence_staleness_check"

// RunStalenessCheck updates evidence_status for every control in the org
// based on evidence age vs evidence_max_age_days.
func (s *Service) RunStalenessCheck(ctx context.Context, orgID string) error {
	n, err := s.repo.UpdateEvidenceStaleness(ctx, orgID)
	if err != nil {
		return fmt.Errorf("evidence staleness check: %w", err)
	}
	log.Info().Str("org_id", orgID).Int("updated", n).Msg("evidence staleness check complete")
	return nil
}

// GetComplianceScore returns the compliance score for an org, counting stale evidence as not-ok.
func (s *Service) GetComplianceScore(ctx context.Context, orgID string) (*ComplianceScore, error) {
	return s.repo.GetComplianceScore(ctx, orgID)
}

// SetControlMaxAge sets the evidence_max_age_days for a control (org override).
func (s *Service) SetControlMaxAge(ctx context.Context, orgID, controlID string, maxAgeDays *int) error {
	return s.repo.SetControlMaxAge(ctx, orgID, controlID, maxAgeDays)
}

// ComplianceScore holds the aggregated compliance score for an org.
type ComplianceScore struct {
	TotalControls int     `json:"total_controls"`
	OkCount       int     `json:"ok_count"`
	StaleCount    int     `json:"stale_count"`
	MissingCount  int     `json:"missing_count"`
	NACount       int     `json:"na_count"`
	ScorePct      float64 `json:"score_pct"`
	AsOf          string  `json:"as_of"`
}

// DefaultMaxAgeDays returns the recommended evidence max age for a given evidence type.
func DefaultMaxAgeDays(evidenceType string) int {
	defaults := map[string]int{
		"scanner":           7,
		"cloud":             2,
		"identity":          7,
		"phishing":          90,
		"policy":            365,
		"pentest":           365,
		"manual":            180,
		"bcp_test":          365,
		"management_review": 365,
	}
	if d, ok := defaults[evidenceType]; ok {
		return d
	}
	return 180
}

// ListStaleControls returns controls with evidence_status = 'stale'.
func (s *Service) ListStaleControls(ctx context.Context, orgID string) ([]Control, error) {
	return s.repo.ListStaleControls(ctx, orgID)
}

// repository methods referenced from this service are defined in repository_evidence_staleness.go.
var _ = time.Now // keep time import

const TaskBackupFreshnessCheck = "comply:backup_freshness_check"

// frequencyInterval maps a backup frequency to its expected interval.
func frequencyInterval(freq string) time.Duration {
	switch freq {
	case "hourly":
		return time.Hour
	case "weekly":
		return 7 * 24 * time.Hour
	case "monthly":
		return 31 * 24 * time.Hour
	default: // daily
		return 24 * time.Hour
	}
}

// backupStaleness returns on_track|at_risk|overdue for the last successful backup
// relative to its configured frequency. A missing/failed last success is overdue.
func backupStaleness(lastSuccess *time.Time, freq, lastStatus string, now time.Time) string {
	if lastSuccess == nil || lastStatus == "failed" {
		return "overdue"
	}
	interval := frequencyInterval(freq)
	age := now.Sub(*lastSuccess)
	switch {
	case age > 2*interval:
		return "overdue"
	case age > interval:
		return "at_risk"
	default:
		return "on_track"
	}
}

// restoreStaleness returns on_track|at_risk|overdue for the most recent restore
// test relative to restore_max_age_days. A missing test is overdue.
func restoreStaleness(lastTest *time.Time, maxAgeDays int, now time.Time) string {
	if lastTest == nil {
		return "overdue"
	}
	if maxAgeDays <= 0 {
		maxAgeDays = 365
	}
	maxAge := time.Duration(maxAgeDays) * 24 * time.Hour
	age := now.Sub(*lastTest)
	switch {
	case age > maxAge:
		return "overdue"
	case age > time.Duration(float64(maxAge)*0.8):
		return "at_risk"
	default:
		return "on_track"
	}
}

// ── Backup Jobs CRUD (raw SQL, org-scoped) ─────────────────────────────────

func (s *Service) ListBackupJobs(ctx context.Context, orgID string) ([]BackupJob, error) {
	rows, err := s.db.Query(ctx, `
		SELECT j.id::text, j.org_id::text, j.name, j.source, j.destination, j.frequency,
		       j.encrypted, j.last_success_at, j.last_status, j.restore_max_age_days,
		       j.notes, j.created_at, j.updated_at,
		       (SELECT MAX(t.tested_at) FROM ck_backup_restore_tests t
		          WHERE t.job_id = j.id AND t.result <> 'failed') AS last_restore_test_at
		FROM ck_backup_jobs j
		WHERE j.org_id = $1
		ORDER BY j.name ASC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list backup jobs: %w", err)
	}
	defer rows.Close()

	now := time.Now().UTC()
	out := []BackupJob{}
	for rows.Next() {
		var j BackupJob
		var lastRestore *time.Time
		if err := rows.Scan(&j.ID, &j.OrgID, &j.Name, &j.Source, &j.Destination, &j.Frequency,
			&j.Encrypted, &j.LastSuccessAt, &j.LastStatus, &j.RestoreMaxAgeDays,
			&j.Notes, &j.CreatedAt, &j.UpdatedAt, &lastRestore); err != nil {
			return nil, fmt.Errorf("scan backup job: %w", err)
		}
		j.LastRestoreTestAt = lastRestore
		j.BackupStatus = backupStaleness(j.LastSuccessAt, j.Frequency, j.LastStatus, now)
		j.RestoreStatus = restoreStaleness(lastRestore, j.RestoreMaxAgeDays, now)
		out = append(out, j)
	}
	return out, rows.Err()
}

func (s *Service) CreateBackupJob(ctx context.Context, orgID string, in BackupJobInput) (BackupJob, error) {
	if in.RestoreMaxAgeDays == 0 {
		in.RestoreMaxAgeDays = 365
	}
	if in.LastStatus == "" {
		in.LastStatus = "unknown"
	}
	var lastSuccess *time.Time
	if in.LastSuccessAt != "" {
		t, err := time.Parse(time.RFC3339, in.LastSuccessAt)
		if err != nil {
			return BackupJob{}, fmt.Errorf("parse last_success_at: %w", err)
		}
		lastSuccess = &t
	}
	var id string
	err := s.db.QueryRow(ctx, `
		INSERT INTO ck_backup_jobs
		  (org_id, name, source, destination, frequency, encrypted,
		   last_success_at, last_status, restore_max_age_days, notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id::text`,
		orgID, in.Name, in.Source, in.Destination, in.Frequency, in.Encrypted,
		lastSuccess, in.LastStatus, in.RestoreMaxAgeDays, in.Notes).Scan(&id)
	if err != nil {
		return BackupJob{}, fmt.Errorf("create backup job: %w", err)
	}
	return s.GetBackupJob(ctx, orgID, id)
}

func (s *Service) GetBackupJob(ctx context.Context, orgID, id string) (BackupJob, error) {
	jobs, err := s.ListBackupJobs(ctx, orgID)
	if err != nil {
		return BackupJob{}, err
	}
	for _, j := range jobs {
		if j.ID == id {
			return j, nil
		}
	}
	return BackupJob{}, fmt.Errorf("backup job not found")
}

func (s *Service) UpdateBackupJob(ctx context.Context, orgID, id string, in BackupJobInput) (BackupJob, error) {
	if in.RestoreMaxAgeDays == 0 {
		in.RestoreMaxAgeDays = 365
	}
	if in.LastStatus == "" {
		in.LastStatus = "unknown"
	}
	var lastSuccess *time.Time
	if in.LastSuccessAt != "" {
		t, err := time.Parse(time.RFC3339, in.LastSuccessAt)
		if err != nil {
			return BackupJob{}, fmt.Errorf("parse last_success_at: %w", err)
		}
		lastSuccess = &t
	}
	tag, err := s.db.Exec(ctx, `
		UPDATE ck_backup_jobs
		SET name=$3, source=$4, destination=$5, frequency=$6, encrypted=$7,
		    last_success_at=$8, last_status=$9, restore_max_age_days=$10, notes=$11,
		    updated_at=NOW()
		WHERE id=$1 AND org_id=$2`,
		id, orgID, in.Name, in.Source, in.Destination, in.Frequency, in.Encrypted,
		lastSuccess, in.LastStatus, in.RestoreMaxAgeDays, in.Notes)
	if err != nil {
		return BackupJob{}, fmt.Errorf("update backup job: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return BackupJob{}, fmt.Errorf("backup job not found")
	}
	return s.GetBackupJob(ctx, orgID, id)
}

func (s *Service) DeleteBackupJob(ctx context.Context, orgID, id string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM ck_backup_jobs WHERE id=$1 AND org_id=$2`, id, orgID)
	if err != nil {
		return fmt.Errorf("delete backup job: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("backup job not found")
	}
	return nil
}

// ── Restore Tests ──────────────────────────────────────────────────────────

func (s *Service) ListRestoreTests(ctx context.Context, orgID, jobID string) ([]BackupRestoreTest, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id::text, org_id::text, job_id::text, tested_at, result,
		       rto_target_hours, rto_actual_hours, tester, notes, created_at
		FROM ck_backup_restore_tests
		WHERE org_id=$1 AND job_id=$2
		ORDER BY tested_at DESC`, orgID, jobID)
	if err != nil {
		return nil, fmt.Errorf("list restore tests: %w", err)
	}
	defer rows.Close()
	out := []BackupRestoreTest{}
	for rows.Next() {
		var t BackupRestoreTest
		var tested time.Time
		if err := rows.Scan(&t.ID, &t.OrgID, &t.JobID, &tested, &t.Result,
			&t.RTOTargetHours, &t.RTOActualHours, &t.Tester, &t.Notes, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan restore test: %w", err)
		}
		t.TestedAt = tested.Format("2006-01-02")
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Service) CreateRestoreTest(ctx context.Context, orgID, jobID string, in RestoreTestInput) (BackupRestoreTest, error) {
	// Verify the job belongs to the org (defence against cross-org job_id).
	var exists bool
	if err := s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM ck_backup_jobs WHERE id=$1 AND org_id=$2)`,
		jobID, orgID).Scan(&exists); err != nil {
		return BackupRestoreTest{}, fmt.Errorf("verify job: %w", err)
	}
	if !exists {
		return BackupRestoreTest{}, fmt.Errorf("backup job not found")
	}
	tested, err := time.Parse("2006-01-02", in.TestedAt)
	if err != nil {
		return BackupRestoreTest{}, fmt.Errorf("parse tested_at: %w", err)
	}
	var id string
	err = s.db.QueryRow(ctx, `
		INSERT INTO ck_backup_restore_tests
		  (org_id, job_id, tested_at, result, rto_target_hours, rto_actual_hours, tester, notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id::text`,
		orgID, jobID, tested, in.Result, in.RTOTargetHours, in.RTOActualHours, in.Tester, in.Notes).Scan(&id)
	if err != nil {
		return BackupRestoreTest{}, fmt.Errorf("create restore test: %w", err)
	}
	// Sync evidence opportunistically (best-effort).
	if err := s.SyncBackupEvidence(ctx, orgID); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("backup evidence sync after restore test")
	}
	tests, err := s.ListRestoreTests(ctx, orgID, jobID)
	if err != nil {
		return BackupRestoreTest{}, err
	}
	for _, t := range tests {
		if t.ID == id {
			return t, nil
		}
	}
	return BackupRestoreTest{}, fmt.Errorf("restore test not found after insert")
}

// ── Summary + Evidence sync ────────────────────────────────────────────────

func (s *Service) GetBackupSummary(ctx context.Context, orgID string) (BackupSummary, error) {
	jobs, err := s.ListBackupJobs(ctx, orgID)
	if err != nil {
		return BackupSummary{}, err
	}
	var sum BackupSummary
	sum.TotalJobs = len(jobs)
	for _, j := range jobs {
		if j.BackupStatus == "overdue" {
			sum.OverdueBackups++
		}
		if j.RestoreStatus == "overdue" {
			sum.OverdueRestores++
		}
		if j.LastRestoreTestAt != nil {
			sum.TestedJobs++
		}
	}
	return sum, nil
}

// SyncBackupEvidence attaches A.8.13 (and DER.4.A1 once BSI is enabled) evidence
// when the org has at least one backup job with a passed restore test on record.
func (s *Service) SyncBackupEvidence(ctx context.Context, orgID string) error {
	jobs, err := s.ListBackupJobs(ctx, orgID)
	if err != nil {
		return fmt.Errorf("backup evidence: list jobs: %w", err)
	}
	tested := 0
	for _, j := range jobs {
		if j.LastRestoreTestAt != nil && j.RestoreStatus != "overdue" {
			tested++
		}
	}
	if tested == 0 {
		return nil
	}
	title := "Backup-/Restore-Nachweis dokumentiert"
	desc := fmt.Sprintf("%d Backup-Job(s) mit aktuellem Restore-Test dokumentiert (ISO 27001 A.8.13)", tested)
	// ISO 27001:2022 A.8.13 — Information backup.
	s.ensureBackupEvidence(ctx, orgID, "A.8.13", title, desc)
	// BSI DER.4.A1 — once the BSI catalog is enabled in the org (no-op otherwise).
	s.ensureBackupEvidence(ctx, orgID, "BSI-DER.4.A1", title, desc)
	return nil
}

// ensureBackupEvidence idempotently attaches collector evidence to a control.
func (s *Service) ensureBackupEvidence(ctx context.Context, orgID, controlCode, title, desc string) {
	controlID, err := s.repo.FindControlByCode(ctx, orgID, controlCode)
	if err != nil || controlID == "" {
		return // control / framework not present — skip silently
	}
	payload := []byte(fmt.Sprintf(`{"source":"backup_evidence","control_code":%q,"description":%q}`, controlCode, desc))
	if _, err := s.repo.AddCollectorEvidence(ctx, orgID, controlID, "", "automated", title, payload); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Str("control", controlCode).Msg("backup evidence sync")
	}
}

// CheckBackupFreshness is the body of the daily Asynq job: it syncs evidence and
// returns the count of overdue items for logging. Reminders are surfaced via the
// dashboard summary; this keeps the write path simple and idempotent.
func (s *Service) CheckBackupFreshness(ctx context.Context, orgID string) (overdue int, err error) {
	sum, err := s.GetBackupSummary(ctx, orgID)
	if err != nil {
		return 0, err
	}
	if err := s.SyncBackupEvidence(ctx, orgID); err != nil {
		return 0, err
	}
	return sum.OverdueBackups + sum.OverdueRestores, nil
}

const TaskBCMEvidenceSync = "comply:bcm_evidence_sync"

// SyncBCMEvidence creates evidence for DER.4 controls when the corresponding
// BCM data is present for the organisation.
func (s *Service) SyncBCMEvidence(ctx context.Context, orgID string) error {
	// DER.4.A4 — BIA erstellt (if ≥1 critical process)
	highCount, err := s.BCM.CountHighCriticalityBIAProcesses(ctx, orgID)
	if err != nil {
		return fmt.Errorf("bcm_evidence_sync: count bia processes: %w", err)
	}
	if highCount > 0 {
		if err := s.ensureBCMEvidence(ctx, orgID, "BSI-DER.4.A4",
			"BIA: Kritische Prozesse dokumentiert",
			fmt.Sprintf("%d kritische Geschäftsprozesse in BIA erfasst (BSI-200-4 DER.4.A4)", highCount),
		); err != nil {
			log.Warn().Err(err).Str("org_id", orgID).Str("control", "DER.4.A4").Msg("bcm_evidence_sync")
		}
	}

	// DER.4.A5 — Notfallkonzept vorhanden (if ≥1 active WAP)
	activeWAPs, err := s.BCM.CountRecoveryPlansActive(ctx, orgID)
	if err != nil {
		return fmt.Errorf("bcm_evidence_sync: count active waps: %w", err)
	}
	if activeWAPs > 0 {
		if err := s.ensureBCMEvidence(ctx, orgID, "BSI-DER.4.A5",
			"Wiederanlaufplan (WAP) vorhanden",
			fmt.Sprintf("%d aktive Wiederanlaufpläne dokumentiert (BSI-200-4 DER.4.A5)", activeWAPs),
		); err != nil {
			log.Warn().Err(err).Str("org_id", orgID).Str("control", "DER.4.A5").Msg("bcm_evidence_sync")
		}
	}

	// DER.4.A6 — Übung durchgeführt (if ≥1 tested WAP in last 12 months)
	testedCount, err := s.BCM.CountRecoveryPlansTested(ctx, orgID)
	if err != nil {
		return fmt.Errorf("bcm_evidence_sync: count tested waps: %w", err)
	}
	if testedCount > 0 {
		if err := s.ensureBCMEvidence(ctx, orgID, "BSI-DER.4.A6",
			"Notfallübung dokumentiert",
			fmt.Sprintf("%d Wiederanlaufpläne in den letzten 12 Monaten getestet (BSI-200-4 DER.4.A6)", testedCount),
		); err != nil {
			log.Warn().Err(err).Str("org_id", orgID).Str("control", "DER.4.A6").Msg("bcm_evidence_sync")
		}
	}

	// DER.4.A8 — Alarmierungsplan/Kontaktverzeichnis vorhanden (if ≥1 contact)
	contactCount, err := s.BCM.CountEmergencyContacts(ctx, orgID)
	if err != nil {
		return fmt.Errorf("bcm_evidence_sync: count emergency contacts: %w", err)
	}
	if contactCount > 0 {
		if err := s.ensureBCMEvidence(ctx, orgID, "BSI-DER.4.A8",
			"Alarmierungsplan/Kontaktverzeichnis gepflegt",
			fmt.Sprintf("%d Notfallkontakte im Alarmierungsplan erfasst (BSI-200-4 DER.4.A8)", contactCount),
		); err != nil {
			log.Warn().Err(err).Str("org_id", orgID).Str("control", "DER.4.A8").Msg("bcm_evidence_sync")
		}
	}

	return nil
}

// ensureBCMEvidence idempotently creates an evidence entry for the given BSI control.
// It uses AddCollectorEvidence which is upsert-safe.
func (s *Service) ensureBCMEvidence(ctx context.Context, orgID, controlCode, title, description string) error {
	controlID, err := s.repo.FindControlByCode(ctx, orgID, controlCode)
	if err != nil || controlID == "" {
		// Control not found — BSI framework may not be enabled; skip silently
		return nil
	}
	payload := []byte(fmt.Sprintf(`{"source":"bcm_evidence_sync","control_code":%q,"description":%q}`, controlCode, description))
	_, err = s.repo.AddCollectorEvidence(ctx, orgID, controlID, "", "automated", title, payload)
	return err
}

func isTLPTOverdue(tests []ResilienceTest, now time.Time) bool {
	threshold := now.AddDate(-3, 0, 0)
	for _, t := range tests {
		if t.Type == "tlpt" && t.TestDate.After(threshold) {
			return false
		}
	}
	return true
}

// ListResilienceTests returns all resilience tests for the organisation, with computed OverdueWarning per entry.
// It also returns whether there is a global TLPT overdue warning.
func (s *Service) ListResilienceTests(ctx context.Context, orgID string) ([]ResilienceTest, bool, error) {
	tests, err := s.repo.ListResilienceTests(ctx, orgID)
	if err != nil {
		return nil, false, fmt.Errorf("list resilience tests: %w", err)
	}
	if tests == nil {
		tests = []ResilienceTest{}
	}
	now := time.Now().UTC()
	threshold := now.AddDate(-3, 0, 0)
	for i := range tests {
		if tests[i].Type == "tlpt" && tests[i].TestDate.Before(threshold) {
			tests[i].OverdueWarning = true
		}
	}
	tlptOverdue := isTLPTOverdue(tests, now)
	return tests, tlptOverdue, nil
}

// GetResilienceTest returns a single resilience test with computed OverdueWarning.
func (s *Service) GetResilienceTest(ctx context.Context, orgID, id string) (*ResilienceTest, error) {
	t, err := s.repo.GetResilienceTest(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	if t.Type == "tlpt" && t.TestDate.Before(time.Now().UTC().AddDate(-3, 0, 0)) {
		t.OverdueWarning = true
	}
	return t, nil
}

// CreateResilienceTest creates a new resilience test entry.
func (s *Service) CreateResilienceTest(ctx context.Context, orgID string, in CreateResilienceTestInput) (*ResilienceTest, error) {
	return s.repo.CreateResilienceTest(ctx, orgID, in)
}

// UpdateResilienceTest updates an existing resilience test entry.
func (s *Service) UpdateResilienceTest(ctx context.Context, orgID, id string, in UpdateResilienceTestInput) (*ResilienceTest, error) {
	t, err := s.repo.UpdateResilienceTest(ctx, orgID, id, in)
	if err != nil {
		return nil, err
	}
	if t.Type == "tlpt" && t.TestDate.Before(time.Now().UTC().AddDate(-3, 0, 0)) {
		t.OverdueWarning = true
	}
	return t, nil
}

// DeleteResilienceTest removes a resilience test entry.
func (s *Service) DeleteResilienceTest(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteResilienceTest(ctx, orgID, id)
}

// AttachResilienceTestFile saves an uploaded file to disk and updates the attachment_url.
// Files are stored at uploadDir/resilience-tests/{id}/{filename}.
func (s *Service) AttachResilienceTestFile(ctx context.Context, orgID, id, uploadDir string, fileBytes []byte, filename string) (*ResilienceTest, error) {
	dir := fmt.Sprintf("%s/resilience-tests/%s", uploadDir, id)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create upload dir: %w", err)
	}
	destPath := fmt.Sprintf("%s/%s", dir, filename)
	if err := os.WriteFile(destPath, fileBytes, 0o640); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}
	if err := s.repo.UpdateResilienceTestAttachment(ctx, orgID, id, destPath); err != nil {
		return nil, fmt.Errorf("update attachment: %w", err)
	}
	return s.GetResilienceTest(ctx, orgID, id)
}

var allowedEvidenceMIME = map[string]bool{
	"application/pdf": true,
	"image/png":       true,
	"image/jpeg":      true,
	"text/plain":      true,
	"text/csv":        true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":       true, // xlsx
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true, // docx
	"application/zip":              true,
	"application/x-zip-compressed": true,
}

// allowedEvidenceExt lists permitted file extensions (lower-case, with dot).
var allowedEvidenceExt = map[string]bool{
	".pdf":  true,
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".txt":  true,
	".csv":  true,
	".xlsx": true,
	".docx": true,
	".zip":  true,
}

const maxEvidenceFileSizeBytes int64 = 50 * 1024 * 1024 // 50 MB

// EvidenceFileService handles storage and retrieval of evidence file attachments.
type EvidenceFileService struct {
	repo      *Repository
	uploadDir string
}

// NewEvidenceFileService creates a new EvidenceFileService.
func NewEvidenceFileService(repo *Repository, uploadDir string) *EvidenceFileService {
	if uploadDir == "" {
		uploadDir = "./data/uploads"
	}
	return &EvidenceFileService{repo: repo, uploadDir: uploadDir}
}

// Upload validates, stores, and records a new evidence file.
// evidenceID may be empty when a file is attached directly to a control without a parent evidence record.
func (s *EvidenceFileService) Upload(
	ctx context.Context,
	orgID, controlID, evidenceID, uploaderID string,
	file multipart.File,
	header *multipart.FileHeader,
) (EvidenceFile, error) {
	// Size check
	if header.Size > maxEvidenceFileSizeBytes {
		return EvidenceFile{}, fmt.Errorf("file too large: max 50 MB")
	}

	// Extension check
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedEvidenceExt[ext] {
		return EvidenceFile{}, fmt.Errorf("file type not allowed: %s", ext)
	}

	// MIME sniff from actual content — never trust the client-supplied Content-Type header.
	sniffBuf := make([]byte, 512)
	n, _ := file.Read(sniffBuf)
	detected := http.DetectContentType(sniffBuf[:n])
	// Strip parameters (e.g. "text/plain; charset=utf-8" → "text/plain")
	if idx := strings.Index(detected, ";"); idx != -1 {
		detected = strings.TrimSpace(detected[:idx])
	}
	if !allowedEvidenceMIME[detected] {
		return EvidenceFile{}, fmt.Errorf("MIME type not allowed: %s", detected)
	}
	// Seek back so io.Copy writes the full file, not just bytes after the sniff window.
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return EvidenceFile{}, fmt.Errorf("seek file: %w", err)
	}

	// Build destination path
	storedName := uuid.New().String() + ext
	dir := filepath.Join(s.uploadDir, "evidence", orgID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return EvidenceFile{}, fmt.Errorf("create upload dir: %w", err)
	}
	destPath := filepath.Join(dir, storedName)

	// Write file to disk
	dst, err := os.Create(destPath)
	if err != nil {
		return EvidenceFile{}, fmt.Errorf("create file: %w", err)
	}
	defer func() { _ = dst.Close() }()

	if _, err := io.Copy(dst, file); err != nil {
		_ = os.Remove(destPath)
		return EvidenceFile{}, fmt.Errorf("write file: %w", err)
	}

	// Insert DB record
	rec := EvidenceFile{
		OrgID:        orgID,
		EvidenceID:   evidenceID,
		ControlID:    controlID,
		OriginalName: header.Filename,
		StoredName:   storedName,
		MimeType:     detected,
		SizeBytes:    header.Size,
		UploadedBy:   uploaderID,
	}
	out, err := s.repo.CreateEvidenceFile(ctx, rec)
	if err != nil {
		_ = os.Remove(destPath)
		return EvidenceFile{}, fmt.Errorf("record evidence file: %w", err)
	}

	out.DownloadURL = "/api/v1/vaktcomply/evidence-files/" + out.ID + "/download"
	return out, nil
}

// Download returns the file metadata and full disk path for streaming.
func (s *EvidenceFileService) Download(ctx context.Context, orgID, fileID string) (EvidenceFile, string, error) {
	f, err := s.repo.GetEvidenceFile(ctx, orgID, fileID)
	if err != nil {
		return EvidenceFile{}, "", fmt.Errorf("get evidence file: %w", err)
	}
	diskPath := filepath.Join(s.uploadDir, "evidence", orgID, f.StoredName)
	f.DownloadURL = "/api/v1/vaktcomply/evidence-files/" + f.ID + "/download"
	return f, diskPath, nil
}

// Delete removes the DB record and the associated file from disk.
func (s *EvidenceFileService) Delete(ctx context.Context, orgID, fileID string) error {
	f, err := s.repo.DeleteEvidenceFile(ctx, orgID, fileID)
	if err != nil {
		return fmt.Errorf("delete evidence file record: %w", err)
	}
	diskPath := filepath.Join(s.uploadDir, "evidence", orgID, f.StoredName)
	if err := os.Remove(diskPath); err != nil && !os.IsNotExist(err) {
		// Log but don't fail — the DB record is already gone.
		_ = err
	}
	return nil
}

// ListForEvidence returns all files attached to a specific evidence record.
func (s *EvidenceFileService) ListForEvidence(ctx context.Context, orgID, evidenceID string) ([]EvidenceFile, error) {
	items, err := s.repo.ListEvidenceFiles(ctx, orgID, evidenceID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].DownloadURL = "/api/v1/vaktcomply/evidence-files/" + items[i].ID + "/download"
	}
	return items, nil
}

// ListForControl returns all files attached to any evidence under a given control.
func (s *EvidenceFileService) ListForControl(ctx context.Context, orgID, controlID string) ([]EvidenceFile, error) {
	items, err := s.repo.ListEvidenceFilesByControl(ctx, orgID, controlID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].DownloadURL = "/api/v1/vaktcomply/evidence-files/" + items[i].ID + "/download"
	}
	return items, nil
}
