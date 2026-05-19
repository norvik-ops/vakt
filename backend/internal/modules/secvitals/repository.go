package secvitals

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles ComplyKit data access using pgx/v5 directly.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new ComplyKit repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// --- Frameworks ---

// CreateFramework inserts a new framework for an organisation.
func (r *Repository) CreateFramework(ctx context.Context, orgID, name, version string, isBuiltin bool) (*Framework, error) {
	var f Framework
	err := r.db.QueryRow(ctx, `
		INSERT INTO ck_frameworks (org_id, name, version, is_builtin)
		VALUES ($1::uuid, $2, $3, $4)
		RETURNING id::text, org_id::text, name, version, is_builtin, created_at`,
		orgID, name, version, isBuiltin,
	).Scan(&f.ID, &f.OrgID, &f.Name, &f.Version, &f.IsBuiltin, &f.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create framework: %w", err)
	}
	return &f, nil
}

// ListFrameworks returns all frameworks enabled for an organisation.
func (r *Repository) ListFrameworks(ctx context.Context, orgID string) ([]Framework, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, name, version, is_builtin, created_at
		FROM ck_frameworks
		WHERE org_id = $1::uuid
		ORDER BY created_at ASC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list frameworks: %w", err)
	}
	defer rows.Close()

	var frameworks []Framework
	for rows.Next() {
		var f Framework
		if err := rows.Scan(&f.ID, &f.OrgID, &f.Name, &f.Version, &f.IsBuiltin, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan framework: %w", err)
		}
		frameworks = append(frameworks, f)
	}
	return frameworks, rows.Err()
}

// DeleteFramework removes a framework and all its controls/evidence (cascade).
func (r *Repository) DeleteFramework(ctx context.Context, orgID, frameworkID string) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM ck_frameworks WHERE id = $1::uuid AND org_id = $2::uuid`,
		frameworkID, orgID,
	)
	if err != nil {
		return fmt.Errorf("delete framework: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("framework not found")
	}
	return nil
}

// GetFramework returns a single framework by ID within an organisation.
func (r *Repository) GetFramework(ctx context.Context, orgID, frameworkID string) (*Framework, error) {
	var f Framework
	err := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, name, version, is_builtin, created_at
		FROM ck_frameworks
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		frameworkID, orgID,
	).Scan(&f.ID, &f.OrgID, &f.Name, &f.Version, &f.IsBuiltin, &f.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get framework: %w", err)
	}
	return &f, nil
}

// FindFrameworkByName returns a single framework by name within an organisation.
// Returns nil, nil if not found.
func (r *Repository) FindFrameworkByName(ctx context.Context, orgID, name string) (*Framework, error) {
	var f Framework
	err := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, name, version, is_builtin, created_at
		FROM ck_frameworks
		WHERE org_id = $1::uuid AND name = $2
		LIMIT 1`,
		orgID, name,
	).Scan(&f.ID, &f.OrgID, &f.Name, &f.Version, &f.IsBuiltin, &f.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find framework by name: %w", err)
	}
	return &f, nil
}

// ListAllBuiltinFrameworks returns all builtin frameworks across all organisations.
// Used for startup reseeding of controls.
func (r *Repository) ListAllBuiltinFrameworks(ctx context.Context) ([]Framework, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, name, version, is_builtin, created_at
		FROM ck_frameworks
		WHERE is_builtin = true
		ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list all builtin frameworks: %w", err)
	}
	defer rows.Close()

	var frameworks []Framework
	for rows.Next() {
		var f Framework
		if err := rows.Scan(&f.ID, &f.OrgID, &f.Name, &f.Version, &f.IsBuiltin, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan framework: %w", err)
		}
		frameworks = append(frameworks, f)
	}
	return frameworks, rows.Err()
}

// FrameworkExists reports whether a framework with the given name already exists for the org.
func (r *Repository) FrameworkExists(ctx context.Context, orgID, name string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM ck_frameworks WHERE org_id = $1::uuid AND name = $2)`,
		orgID, name,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("framework exists check: %w", err)
	}
	return exists, nil
}

// --- Controls ---

// BulkInsertControls inserts a slice of controls for a framework in a single transaction.
func (r *Repository) BulkInsertControls(ctx context.Context, controls []Control) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, c := range controls {
		_, err := tx.Exec(ctx, `
			INSERT INTO ck_controls (framework_id, org_id, control_id, title, description, domain, evidence_type, weight)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (framework_id, control_id) DO UPDATE
			    SET description = EXCLUDED.description,
			        title       = EXCLUDED.title`,
			c.FrameworkID, c.OrgID, c.ControlID, c.Title, c.Description, c.Domain, c.EvidenceType, c.Weight,
		)
		if err != nil {
			return fmt.Errorf("insert control %s: %w", c.ControlID, err)
		}
	}

	return tx.Commit(ctx)
}

// UpdateControl sets not_applicable, reason, manual_status, and optionally maturity_score on a control.
func (r *Repository) UpdateControl(ctx context.Context, orgID, controlID string, notApplicable bool, reason, manualStatus, owner string, maturityScore *int) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE ck_controls
		SET not_applicable        = $3,
		    not_applicable_reason = NULLIF($4, ''),
		    manual_status         = NULLIF($5, ''),
		    owner                 = NULLIF($6, ''),
		    maturity_score        = COALESCE($7, maturity_score)
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		controlID, orgID, notApplicable, reason, manualStatus, owner, maturityScore,
	)
	if err != nil {
		return fmt.Errorf("update control: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("control not found")
	}
	return nil
}

// ListControls returns all controls for a framework within an organisation.
func (r *Repository) ListControls(ctx context.Context, orgID, frameworkID string) ([]Control, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, framework_id::text, org_id::text, control_id, title,
		       COALESCE(description, ''), domain, evidence_type, weight,
		       not_applicable, COALESCE(not_applicable_reason, ''),
		       COALESCE(manual_status, ''), maturity_score, COALESCE(owner, ''),
		       last_reviewed_at, review_interval_days, next_review_due,
		       last_reviewed_by, review_note
		FROM ck_controls
		WHERE framework_id = $1::uuid AND org_id = $2::uuid
		ORDER BY control_id ASC LIMIT 1000`,
		frameworkID, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list controls: %w", err)
	}
	defer rows.Close()

	var controls []Control
	for rows.Next() {
		var c Control
		var nextReviewDue *time.Time
		if err := rows.Scan(&c.ID, &c.FrameworkID, &c.OrgID, &c.ControlID, &c.Title,
			&c.Description, &c.Domain, &c.EvidenceType, &c.Weight,
			&c.NotApplicable, &c.NotApplicableReason, &c.ManualStatus, &c.MaturityScore, &c.Owner,
			&c.LastReviewedAt, &c.ReviewIntervalDays, &nextReviewDue,
			&c.LastReviewedBy, &c.ReviewNote); err != nil {
			return nil, fmt.Errorf("scan control: %w", err)
		}
		c.NextReviewDue = nextReviewDue
		c.IsReviewOverdue = nextReviewDue != nil && nextReviewDue.Before(time.Now())
		controls = append(controls, c)
	}
	return controls, rows.Err()
}

// GetControl returns a single control by its UUID within an organisation.
func (r *Repository) GetControl(ctx context.Context, orgID, controlID string) (*Control, error) {
	var c Control
	var nextReviewDue *time.Time
	err := r.db.QueryRow(ctx, `
		SELECT id::text, framework_id::text, org_id::text, control_id, title,
		       COALESCE(description, ''), domain, evidence_type, weight,
		       not_applicable, COALESCE(not_applicable_reason, ''),
		       COALESCE(manual_status, ''), maturity_score, COALESCE(owner, ''),
		       last_reviewed_at, review_interval_days, next_review_due,
		       last_reviewed_by, review_note
		FROM ck_controls
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		controlID, orgID,
	).Scan(&c.ID, &c.FrameworkID, &c.OrgID, &c.ControlID, &c.Title,
		&c.Description, &c.Domain, &c.EvidenceType, &c.Weight,
		&c.NotApplicable, &c.NotApplicableReason, &c.ManualStatus, &c.MaturityScore, &c.Owner,
		&c.LastReviewedAt, &c.ReviewIntervalDays, &nextReviewDue,
		&c.LastReviewedBy, &c.ReviewNote)
	if err != nil {
		return nil, fmt.Errorf("get control: %w", err)
	}
	c.NextReviewDue = nextReviewDue
	c.IsReviewOverdue = nextReviewDue != nil && nextReviewDue.Before(time.Now())
	return &c, nil
}

// UpdateSoAMetadata persists the SoA-specific fields for a single control.
func (r *Repository) UpdateSoAMetadata(ctx context.Context, orgID, controlID, justification, implementation, responsible string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE ck_controls
		SET soa_justification  = NULLIF($3, ''),
		    soa_implementation = NULLIF($4, ''),
		    soa_responsible    = NULLIF($5, '')
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		controlID, orgID, justification, implementation, responsible,
	)
	if err != nil {
		return fmt.Errorf("update soa metadata: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("control not found")
	}
	return nil
}

// ListControlsForSoA returns all controls for a framework with SoA metadata and evidence counts,
// ordered by control_id for consistent PDF output.
func (r *Repository) ListControlsForSoA(ctx context.Context, orgID, frameworkID string) ([]SoARow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT c.control_id, c.title, c.domain,
		       NOT c.not_applicable,
		       COALESCE(c.soa_justification, ''),
		       COALESCE(c.soa_implementation, ''),
		       COALESCE(c.soa_responsible, ''),
		       COALESCE(c.manual_status, ''),
		       COUNT(e.id) FILTER (WHERE e.status IN ('approved', 'pending'))
		FROM ck_controls c
		LEFT JOIN ck_evidence e ON e.control_id = c.id AND e.org_id = c.org_id
		WHERE c.framework_id = $1::uuid AND c.org_id = $2::uuid
		GROUP BY c.id, c.control_id, c.title, c.domain, c.not_applicable,
		         c.soa_justification, c.soa_implementation, c.soa_responsible, c.manual_status
		ORDER BY c.control_id ASC`,
		frameworkID, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list controls for soa: %w", err)
	}
	defer rows.Close()

	var result []SoARow
	for rows.Next() {
		var row SoARow
		if err := rows.Scan(
			&row.ControlID, &row.Title, &row.Domain,
			&row.Applicable,
			&row.Justification, &row.Implementation, &row.Responsible,
			&row.ManualStatus, &row.EvidenceCount,
		); err != nil {
			return nil, fmt.Errorf("scan soa row: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// CountEvidenceByControl returns the number of approved evidence items per control for a framework.
// Result: map[controlUUID]count.
func (r *Repository) CountEvidenceByControl(ctx context.Context, orgID, frameworkID string) (map[string]int, error) {
	rows, err := r.db.Query(ctx, `
		SELECT e.control_id::text, COUNT(*)
		FROM ck_evidence e
		JOIN ck_controls c ON c.id = e.control_id
		WHERE e.org_id = $1::uuid AND c.framework_id = $2::uuid
		  AND e.status IN ('approved', 'pending')
		GROUP BY e.control_id`,
		orgID, frameworkID,
	)
	if err != nil {
		return nil, fmt.Errorf("count evidence by control: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var controlID string
		var count int
		if err := rows.Scan(&controlID, &count); err != nil {
			return nil, fmt.Errorf("scan evidence count: %w", err)
		}
		counts[controlID] = count
	}
	return counts, rows.Err()
}

// GetExpiringEvidence returns evidence items expiring within the given threshold for a framework.
func (r *Repository) GetExpiringEvidence(ctx context.Context, orgID, frameworkID string, threshold time.Time) ([]Evidence, error) {
	rows, err := r.db.Query(ctx, `
		SELECT e.id::text, e.control_id::text, e.org_id::text, e.title,
		       COALESCE(e.description, ''), e.source, COALESCE(e.file_path, ''),
		       COALESCE(e.file_size, 0), e.status, e.version, e.expires_at, e.expiry_notified_at, created_at, updated_at
		FROM ck_evidence e
		JOIN ck_controls c ON c.id = e.control_id
		WHERE e.org_id = $1::uuid AND c.framework_id = $2::uuid
		  AND e.status = 'approved'
		  AND e.expires_at IS NOT NULL AND e.expires_at <= $3`,
		orgID, frameworkID, threshold,
	)
	if err != nil {
		return nil, fmt.Errorf("get expiring evidence: %w", err)
	}
	defer rows.Close()

	var items []Evidence
	for rows.Next() {
		var ev Evidence
		if err := rows.Scan(&ev.ID, &ev.ControlID, &ev.OrgID, &ev.Title,
			&ev.Description, &ev.Source, &ev.FilePath, &ev.FileSize,
			&ev.Status, &ev.Version, &ev.ExpiresAt, &ev.ExpiryNotifiedAt, &ev.CreatedAt, &ev.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan expiring evidence: %w", err)
		}
		items = append(items, ev)
	}
	return items, rows.Err()
}

// GetExpiringEvidenceAllFrameworks returns evidence expiring within threshold across all frameworks for an org.
func (r *Repository) GetExpiringEvidenceAllFrameworks(ctx context.Context, orgID string, threshold time.Time) ([]Evidence, error) {
	rows, err := r.db.Query(ctx, `
		SELECT e.id::text, e.control_id::text, e.org_id::text, e.title,
		       COALESCE(e.description, ''), e.source, COALESCE(e.file_path, ''),
		       COALESCE(e.file_size, 0), e.status, e.version, e.expires_at, e.expiry_notified_at, e.created_at, e.updated_at
		FROM ck_evidence e
		JOIN ck_controls c ON c.id = e.control_id
		WHERE e.org_id = $1::uuid
		  AND e.status = 'approved'
		  AND e.expires_at IS NOT NULL AND e.expires_at <= $2
		ORDER BY e.expires_at ASC`,
		orgID, threshold,
	)
	if err != nil {
		return nil, fmt.Errorf("get expiring evidence all frameworks: %w", err)
	}
	defer rows.Close()

	var items []Evidence
	for rows.Next() {
		var ev Evidence
		if err := rows.Scan(&ev.ID, &ev.ControlID, &ev.OrgID, &ev.Title,
			&ev.Description, &ev.Source, &ev.FilePath, &ev.FileSize,
			&ev.Status, &ev.Version, &ev.ExpiresAt, &ev.ExpiryNotifiedAt, &ev.CreatedAt, &ev.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan expiring evidence: %w", err)
		}
		items = append(items, ev)
	}
	return items, rows.Err()
}

// EvidenceExpiryNotifyRow is a minimal projection used by the expiry notification worker.
type EvidenceExpiryNotifyRow struct {
	ID           string
	OrgID        string
	Title        string
	ControlTitle string
	ExpiresAt    time.Time
}

// GetUnnotifiedExpiringEvidence returns evidence items that expire within the given
// threshold and have not yet had a notification sent (expiry_notified_at IS NULL).
// It joins ck_controls to include the control title in the notification message.
func (r *Repository) GetUnnotifiedExpiringEvidence(ctx context.Context, orgID string, threshold time.Time) ([]EvidenceExpiryNotifyRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT e.id::text, e.org_id::text, e.title, c.title, e.expires_at
		FROM ck_evidence e
		JOIN ck_controls c ON c.id = e.control_id
		WHERE e.org_id = $1::uuid
		  AND e.expires_at IS NOT NULL
		  AND e.expires_at <= $2
		  AND e.expiry_notified_at IS NULL
		ORDER BY e.expires_at ASC`,
		orgID, threshold,
	)
	if err != nil {
		return nil, fmt.Errorf("get unnotified expiring evidence: %w", err)
	}
	defer rows.Close()

	var items []EvidenceExpiryNotifyRow
	for rows.Next() {
		var row EvidenceExpiryNotifyRow
		if err := rows.Scan(&row.ID, &row.OrgID, &row.Title, &row.ControlTitle, &row.ExpiresAt); err != nil {
			return nil, fmt.Errorf("scan unnotified expiring evidence: %w", err)
		}
		items = append(items, row)
	}
	return items, rows.Err()
}

// MarkEvidenceExpiryNotified sets expiry_notified_at = NOW() for the given evidence IDs.
func (r *Repository) MarkEvidenceExpiryNotified(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.db.Exec(ctx, `
		UPDATE ck_evidence
		SET expiry_notified_at = NOW(), updated_at = NOW()
		WHERE id = ANY($1::uuid[])`,
		ids,
	)
	if err != nil {
		return fmt.Errorf("mark evidence expiry notified: %w", err)
	}
	return nil
}

// --- Evidence ---

// AddEvidence inserts a new evidence item for a control.
func (r *Repository) AddEvidence(ctx context.Context, orgID, controlID, userID string, input AddEvidenceInput) (*Evidence, error) {
	var ev Evidence
	err := r.db.QueryRow(ctx, `
		INSERT INTO ck_evidence (control_id, org_id, title, description, source, file_path, file_size, expires_at, uploaded_by)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, NULLIF($6,''), NULLIF($7, 0), $8, $9::uuid)
		RETURNING id::text, control_id::text, org_id::text, title, COALESCE(description,''),
		          source, COALESCE(file_path,''), COALESCE(file_size,0),
		          status, version, expires_at, expiry_notified_at, created_at, updated_at`,
		controlID, orgID, input.Title, input.Description, input.Source,
		input.FilePath, input.FileSize, input.ExpiresAt, userID,
	).Scan(
		&ev.ID, &ev.ControlID, &ev.OrgID, &ev.Title, &ev.Description,
		&ev.Source, &ev.FilePath, &ev.FileSize,
		&ev.Status, &ev.Version, &ev.ExpiresAt, &ev.ExpiryNotifiedAt, &ev.CreatedAt, &ev.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("add evidence: %w", err)
	}
	return &ev, nil
}

// ListEvidence returns all evidence items for a control within an organisation.
func (r *Repository) ListEvidence(ctx context.Context, orgID, controlID string) ([]Evidence, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, control_id::text, org_id::text, title, COALESCE(description,''),
		       source, COALESCE(file_path,''), COALESCE(file_size,0),
		       status, version, expires_at, expiry_notified_at, created_at, updated_at
		FROM ck_evidence
		WHERE control_id = $1::uuid AND org_id = $2::uuid
		ORDER BY created_at DESC`,
		controlID, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list evidence: %w", err)
	}
	defer rows.Close()

	var items []Evidence
	for rows.Next() {
		var ev Evidence
		if err := rows.Scan(
			&ev.ID, &ev.ControlID, &ev.OrgID, &ev.Title, &ev.Description,
			&ev.Source, &ev.FilePath, &ev.FileSize,
			&ev.Status, &ev.Version, &ev.ExpiresAt, &ev.ExpiryNotifiedAt, &ev.CreatedAt, &ev.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan evidence: %w", err)
		}
		items = append(items, ev)
	}
	return items, rows.Err()
}

// ListEvidenceByControls fetches all evidence for the given control IDs in a single query.
// Returns a map[controlID][]Evidence. Controls with no evidence are absent from the map.
func (r *Repository) ListEvidenceByControls(ctx context.Context, orgID string, controlIDs []string) (map[string][]Evidence, error) {
	if len(controlIDs) == 0 {
		return map[string][]Evidence{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT id::text, control_id::text, org_id::text, title, COALESCE(description,''),
		       source, COALESCE(file_path,''), COALESCE(file_size,0),
		       status, version, expires_at, expiry_notified_at, created_at, updated_at
		FROM ck_evidence
		WHERE control_id = ANY($1::uuid[]) AND org_id = $2::uuid
		ORDER BY control_id, created_at DESC`,
		controlIDs, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list evidence by controls: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]Evidence, len(controlIDs))
	for rows.Next() {
		var ev Evidence
		if err := rows.Scan(
			&ev.ID, &ev.ControlID, &ev.OrgID, &ev.Title, &ev.Description,
			&ev.Source, &ev.FilePath, &ev.FileSize,
			&ev.Status, &ev.Version, &ev.ExpiresAt, &ev.ExpiryNotifiedAt, &ev.CreatedAt, &ev.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan evidence batch: %w", err)
		}
		result[ev.ControlID] = append(result[ev.ControlID], ev)
	}
	return result, rows.Err()
}

// GetEvidence returns a single evidence item by ID within an organisation.
func (r *Repository) GetEvidence(ctx context.Context, orgID, evidenceID string) (*Evidence, error) {
	var ev Evidence
	err := r.db.QueryRow(ctx, `
		SELECT id::text, control_id::text, org_id::text, title, COALESCE(description,''),
		       source, COALESCE(file_path,''), COALESCE(file_size,0),
		       status, version, expires_at, expiry_notified_at, created_at, updated_at
		FROM ck_evidence
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		evidenceID, orgID,
	).Scan(
		&ev.ID, &ev.ControlID, &ev.OrgID, &ev.Title, &ev.Description,
		&ev.Source, &ev.FilePath, &ev.FileSize,
		&ev.Status, &ev.Version, &ev.ExpiresAt, &ev.ExpiryNotifiedAt, &ev.CreatedAt, &ev.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get evidence: %w", err)
	}
	return &ev, nil
}

// ListEvidenceHistory returns the audit history for an evidence item, newest first.
func (r *Repository) ListEvidenceHistory(ctx context.Context, orgID, evidenceID string) ([]EvidenceHistoryEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, evidence_id::text,
		       CASE WHEN changed_by IS NULL THEN NULL ELSE changed_by::text END,
		       changed_at,
		       COALESCE(title,''), COALESCE(status,''), COALESCE(change_note,'')
		FROM ck_evidence_history
		WHERE evidence_id = $1::uuid AND org_id = $2::uuid
		ORDER BY changed_at DESC`,
		evidenceID, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list evidence history: %w", err)
	}
	defer rows.Close()

	var items []EvidenceHistoryEntry
	for rows.Next() {
		var e EvidenceHistoryEntry
		if err := rows.Scan(
			&e.ID, &e.EvidenceID, &e.ChangedBy, &e.ChangedAt,
			&e.Title, &e.Status, &e.ChangeNote,
		); err != nil {
			return nil, fmt.Errorf("scan evidence history: %w", err)
		}
		items = append(items, e)
	}
	if items == nil {
		items = []EvidenceHistoryEntry{}
	}
	return items, rows.Err()
}

// ReviewEvidence updates the status and reviewer information on an evidence item.
func (r *Repository) ReviewEvidence(ctx context.Context, orgID, evidenceID, reviewerID, status string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE ck_evidence
		SET status      = $1,
		    reviewed_by = $2::uuid,
		    reviewed_at = NOW(),
		    updated_at  = NOW()
		WHERE id = $3::uuid AND org_id = $4::uuid`,
		status, reviewerID, evidenceID, orgID,
	)
	if err != nil {
		return fmt.Errorf("review evidence: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("evidence not found")
	}
	return nil
}

// AddCollectorEvidence inserts evidence produced by an automated collector.
func (r *Repository) AddCollectorEvidence(ctx context.Context, orgID, controlID, userID, source, title string, data []byte) (*Evidence, error) {
	var ev Evidence
	err := r.db.QueryRow(ctx, `
		INSERT INTO ck_evidence (control_id, org_id, title, source, collector_data, uploaded_by)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6::uuid)
		RETURNING id::text, control_id::text, org_id::text, title, COALESCE(description,''),
		          source, COALESCE(file_path,''), COALESCE(file_size,0),
		          status, version, expires_at, expiry_notified_at, created_at, updated_at`,
		controlID, orgID, title, source, data, userID,
	).Scan(
		&ev.ID, &ev.ControlID, &ev.OrgID, &ev.Title, &ev.Description,
		&ev.Source, &ev.FilePath, &ev.FileSize,
		&ev.Status, &ev.Version, &ev.ExpiresAt, &ev.ExpiryNotifiedAt, &ev.CreatedAt, &ev.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("add collector evidence: %w", err)
	}
	return &ev, nil
}

// --- Auditor links ---

// CreateAuditorLink inserts a new auditor link record.
func (r *Repository) CreateAuditorLink(ctx context.Context, orgID, frameworkID, userID, tokenHash string, expiresAt time.Time, maxUses *int) (*AuditorLink, error) {
	var al AuditorLink
	err := r.db.QueryRow(ctx, `
		INSERT INTO ck_auditor_links (org_id, framework_id, token_hash, created_by, expires_at, max_uses)
		VALUES ($1::uuid, $2::uuid, $3, $4::uuid, $5, $6)
		RETURNING id::text, org_id::text, framework_id::text, created_by::text, expires_at, used_count, max_uses, created_at`,
		orgID, frameworkID, tokenHash, userID, expiresAt, maxUses,
	).Scan(&al.ID, &al.OrgID, &al.FrameworkID, &al.CreatedBy, &al.ExpiresAt, &al.UsedCount, &al.MaxUses, &al.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create auditor link: %w", err)
	}
	return &al, nil
}

// GetAuditorLinkByHash looks up an auditor link by its token hash and validates expiry.
// Returns an error if the link has been revoked.
func (r *Repository) GetAuditorLinkByHash(ctx context.Context, tokenHash string) (*AuditorLink, error) {
	var al AuditorLink
	var revokedAt *time.Time
	err := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, COALESCE(framework_id::text,''), created_by::text,
		       expires_at, used_count, max_uses, created_at, revoked_at
		FROM ck_auditor_links
		WHERE token_hash = $1`,
		tokenHash,
	).Scan(&al.ID, &al.OrgID, &al.FrameworkID, &al.CreatedBy, &al.ExpiresAt, &al.UsedCount, &al.MaxUses, &al.CreatedAt, &revokedAt)
	if err != nil {
		return nil, fmt.Errorf("get auditor link: %w", err)
	}
	if revokedAt != nil {
		return nil, fmt.Errorf("auditor link revoked")
	}
	return &al, nil
}

// GetAuditorLinkByID returns an auditor link by UUID within an organisation.
func (r *Repository) GetAuditorLinkByID(ctx context.Context, orgID, linkID string) (*AuditorLinkListItem, error) {
	var al AuditorLinkListItem
	err := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, COALESCE(framework_id::text,''),
		       COALESCE(label,''), created_by::text, expires_at,
		       last_accessed_at, access_count, revoked_at, created_at
		FROM ck_auditor_links
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		linkID, orgID,
	).Scan(&al.ID, &al.OrgID, &al.FrameworkID, &al.Label,
		&al.CreatedBy, &al.ExpiresAt, &al.LastAccessedAt, &al.AccessCount, &al.RevokedAt, &al.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get auditor link by id: %w", err)
	}
	return &al, nil
}

// ListAuditorLinks returns all non-revoked auditor links for an organisation.
func (r *Repository) ListAuditorLinks(ctx context.Context, orgID string) ([]AuditorLinkListItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, COALESCE(framework_id::text,''),
		       COALESCE(label,''), created_by::text, expires_at,
		       last_accessed_at, access_count, revoked_at, created_at
		FROM ck_auditor_links
		WHERE org_id = $1::uuid
		ORDER BY created_at DESC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list auditor links: %w", err)
	}
	defer rows.Close()

	var links []AuditorLinkListItem
	for rows.Next() {
		var al AuditorLinkListItem
		if err := rows.Scan(&al.ID, &al.OrgID, &al.FrameworkID, &al.Label,
			&al.CreatedBy, &al.ExpiresAt, &al.LastAccessedAt, &al.AccessCount, &al.RevokedAt, &al.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan auditor link: %w", err)
		}
		links = append(links, al)
	}
	return links, rows.Err()
}

// RevokeAuditorLink sets revoked_at on an auditor link, preventing future access.
func (r *Repository) RevokeAuditorLink(ctx context.Context, orgID, linkID string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE ck_auditor_links
		SET revoked_at = NOW()
		WHERE id = $1::uuid AND org_id = $2::uuid AND revoked_at IS NULL`,
		linkID, orgID,
	)
	if err != nil {
		return fmt.Errorf("revoke auditor link: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("auditor link not found or already revoked")
	}
	return nil
}

// UpdateAuditorLinkAccess bumps access_count and sets last_accessed_at.
func (r *Repository) UpdateAuditorLinkAccess(ctx context.Context, linkID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE ck_auditor_links
		SET access_count = access_count + 1,
		    last_accessed_at = NOW()
		WHERE id = $1::uuid`, linkID)
	return err
}

// IncrementAuditorLinkUsage bumps the used_count on an auditor link.
func (r *Repository) IncrementAuditorLinkUsage(ctx context.Context, linkID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE ck_auditor_links SET used_count = used_count + 1 WHERE id = $1::uuid`, linkID)
	return err
}

// FindControlsByKeywords returns controls whose title or domain matches any of
// the given lowercase keywords. Used by cross-module evidence workers.
func (r *Repository) FindControlsByKeywords(ctx context.Context, orgID string, keywords []string) ([]Control, error) {
	if len(keywords) == 0 {
		return nil, nil
	}
	// Build LIKE conditions for each keyword.
	conds := make([]string, 0, len(keywords)*2)
	args := []any{orgID}
	for _, kw := range keywords {
		n := len(args) + 1
		args = append(args, "%"+kw+"%")
		conds = append(conds, fmt.Sprintf("lower(title) LIKE $%d OR lower(domain) LIKE $%d", n, n))
	}
	q := `SELECT id::text, framework_id::text, org_id::text, control_id, title,
		         COALESCE(description,''), domain, COALESCE(evidence_type,''), weight,
		         not_applicable, COALESCE(not_applicable_reason,''),
		         COALESCE(manual_status,''), maturity_score,
		         last_reviewed_at, review_interval_days, next_review_due,
		         last_reviewed_by, review_note
		  FROM ck_controls
		  WHERE org_id = $1::uuid
		    AND not_applicable = false
		    AND (` + strings.Join(conds, " OR ") + `)
		  LIMIT 10`
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("find controls by keywords: %w", err)
	}
	defer rows.Close()
	var controls []Control
	for rows.Next() {
		var c Control
		var nextReviewDue *time.Time
		if err := rows.Scan(&c.ID, &c.FrameworkID, &c.OrgID, &c.ControlID, &c.Title,
			&c.Description, &c.Domain, &c.EvidenceType, &c.Weight,
			&c.NotApplicable, &c.NotApplicableReason, &c.ManualStatus, &c.MaturityScore,
			&c.LastReviewedAt, &c.ReviewIntervalDays, &nextReviewDue,
			&c.LastReviewedBy, &c.ReviewNote); err != nil {
			continue
		}
		c.NextReviewDue = nextReviewDue
		c.IsReviewOverdue = nextReviewDue != nil && nextReviewDue.Before(time.Now())
		controls = append(controls, c)
	}
	return controls, rows.Err()
}

// FindPatchControls returns controls whose title or domain mentions patch,
// vulnerability, or update.  Used by the SecPulse auto-evidence worker to
// attach resolved-finding evidence to relevant compliance controls.
func (r *Repository) FindPatchControls(ctx context.Context, orgID string) ([]Control, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, framework_id::text, org_id::text, control_id, title,
		       COALESCE(description, ''), domain, COALESCE(evidence_type,''), weight,
		       not_applicable, COALESCE(not_applicable_reason,''),
		       COALESCE(manual_status,''), maturity_score,
		       last_reviewed_at, review_interval_days, next_review_due,
		       last_reviewed_by, review_note
		FROM ck_controls
		WHERE org_id = $1::uuid
		  AND (
		    lower(title)  LIKE '%patch%'
		    OR lower(title)  LIKE '%vulnerability%'
		    OR lower(title)  LIKE '%update%'
		    OR lower(domain) LIKE '%patch%'
		  )
		  AND not_applicable = false
		LIMIT 10`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("find patch controls: %w", err)
	}
	defer rows.Close()

	var controls []Control
	for rows.Next() {
		var c Control
		var nextReviewDue *time.Time
		if err := rows.Scan(&c.ID, &c.FrameworkID, &c.OrgID, &c.ControlID, &c.Title,
			&c.Description, &c.Domain, &c.EvidenceType, &c.Weight,
			&c.NotApplicable, &c.NotApplicableReason, &c.ManualStatus, &c.MaturityScore,
			&c.LastReviewedAt, &c.ReviewIntervalDays, &nextReviewDue,
			&c.LastReviewedBy, &c.ReviewNote); err != nil {
			continue
		}
		c.NextReviewDue = nextReviewDue
		c.IsReviewOverdue = nextReviewDue != nil && nextReviewDue.Before(time.Now())
		controls = append(controls, c)
	}
	return controls, rows.Err()
}

// --- Risk Assessment (FR-CK12) ---

func (r *Repository) ListRisks(ctx context.Context, orgID string) ([]Risk, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, title, COALESCE(description,''), COALESCE(category,''),
		       likelihood, impact, risk_score, COALESCE(owner,''), status, treatment,
		       COALESCE(treatment_notes,''),
		       COALESCE(treatment_option,''), COALESCE(treatment_plan,''), COALESCE(treatment_owner,''),
		       treatment_due_date, COALESCE(treatment_status,'pending'),
		       residual_likelihood, residual_impact,
		       created_at, updated_at
		FROM ck_risks
		WHERE org_id = $1::uuid
		ORDER BY risk_score DESC, created_at DESC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list risks: %w", err)
	}
	defer rows.Close()

	var risks []Risk
	for rows.Next() {
		var r2 Risk
		if err := rows.Scan(&r2.ID, &r2.OrgID, &r2.Title, &r2.Description, &r2.Category,
			&r2.Likelihood, &r2.Impact, &r2.RiskScore, &r2.Owner,
			&r2.Status, &r2.Treatment, &r2.TreatmentNotes,
			&r2.TreatmentOption, &r2.TreatmentPlan, &r2.TreatmentOwner,
			&r2.TreatmentDueDate, &r2.TreatmentStatus,
			&r2.ResidualLikelihood, &r2.ResidualImpact,
			&r2.CreatedAt, &r2.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan risk: %w", err)
		}
		risks = append(risks, r2)
	}
	return risks, rows.Err()
}

func (r *Repository) GetRisk(ctx context.Context, orgID, id string) (*Risk, error) {
	var risk Risk
	err := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, title, COALESCE(description,''), COALESCE(category,''),
		       likelihood, impact, risk_score, COALESCE(owner,''), status, treatment,
		       COALESCE(treatment_notes,''),
		       COALESCE(treatment_option,''), COALESCE(treatment_plan,''), COALESCE(treatment_owner,''),
		       treatment_due_date, COALESCE(treatment_status,'pending'),
		       residual_likelihood, residual_impact,
		       created_at, updated_at
		FROM ck_risks WHERE id = $1::uuid AND org_id = $2::uuid`, id, orgID,
	).Scan(&risk.ID, &risk.OrgID, &risk.Title, &risk.Description, &risk.Category,
		&risk.Likelihood, &risk.Impact, &risk.RiskScore, &risk.Owner,
		&risk.Status, &risk.Treatment, &risk.TreatmentNotes,
		&risk.TreatmentOption, &risk.TreatmentPlan, &risk.TreatmentOwner,
		&risk.TreatmentDueDate, &risk.TreatmentStatus,
		&risk.ResidualLikelihood, &risk.ResidualImpact,
		&risk.CreatedAt, &risk.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get risk: %w", err)
	}
	return &risk, nil
}

func (r *Repository) UpdateRisk(ctx context.Context, orgID, id string, in UpdateRiskInput) (*Risk, error) {
	var risk Risk
	err := r.db.QueryRow(ctx, `
		UPDATE ck_risks SET title=$3, description=$4, category=$5, likelihood=$6, impact=$7,
		       owner=$8, status=$9, treatment=$10, treatment_notes=$11, updated_at=now()
		WHERE id=$1::uuid AND org_id=$2::uuid
		RETURNING id::text, org_id::text, title, COALESCE(description,''), COALESCE(category,''),
		          likelihood, impact, risk_score, COALESCE(owner,''), status, treatment,
		          COALESCE(treatment_notes,''),
		          COALESCE(treatment_option,''), COALESCE(treatment_plan,''), COALESCE(treatment_owner,''),
		          treatment_due_date, COALESCE(treatment_status,'pending'),
		          residual_likelihood, residual_impact,
		          created_at, updated_at`,
		id, orgID, in.Title, in.Description, in.Category,
		in.Likelihood, in.Impact, in.Owner, in.Status, in.Treatment, in.TreatmentNotes,
	).Scan(&risk.ID, &risk.OrgID, &risk.Title, &risk.Description, &risk.Category,
		&risk.Likelihood, &risk.Impact, &risk.RiskScore, &risk.Owner,
		&risk.Status, &risk.Treatment, &risk.TreatmentNotes,
		&risk.TreatmentOption, &risk.TreatmentPlan, &risk.TreatmentOwner,
		&risk.TreatmentDueDate, &risk.TreatmentStatus,
		&risk.ResidualLikelihood, &risk.ResidualImpact,
		&risk.CreatedAt, &risk.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update risk: %w", err)
	}
	return &risk, nil
}

// UpdateRiskTreatment patches only the treatment workflow fields for a risk.
func (r *Repository) UpdateRiskTreatment(ctx context.Context, orgID, id string, in UpdateRiskTreatmentInput) (*Risk, error) {
	var dueDateExpr string
	var args []interface{}
	args = append(args, id, orgID, in.TreatmentOption, in.TreatmentPlan, in.TreatmentOwner, in.TreatmentStatus, in.ResidualLikelihood, in.ResidualImpact)

	// dueDateExpr is a hardcoded SQL expression — never user input.
	// Three possible values: a positional param "$9", the column reference "treatment_due_date",
	// or the SQL literal "NULL". No injection risk.
	if in.TreatmentDueDate != nil && *in.TreatmentDueDate != "" {
		dueDateExpr = "$9"
		args = append(args, *in.TreatmentDueDate)
	} else if in.TreatmentDueDate == nil {
		// null explicitly — keep existing
		dueDateExpr = "treatment_due_date"
	} else {
		// empty string — set to NULL
		dueDateExpr = "NULL"
	}

	query := fmt.Sprintf(`
		UPDATE ck_risks
		SET treatment_option=$3, treatment_plan=$4, treatment_owner=$5,
		    treatment_status=$6, residual_likelihood=$7, residual_impact=$8,
		    treatment_due_date=%s, updated_at=now()
		WHERE id=$1::uuid AND org_id=$2::uuid
		RETURNING id::text, org_id::text, title, COALESCE(description,''), COALESCE(category,''),
		          likelihood, impact, risk_score, COALESCE(owner,''), status, treatment,
		          COALESCE(treatment_notes,''),
		          COALESCE(treatment_option,''), COALESCE(treatment_plan,''), COALESCE(treatment_owner,''),
		          treatment_due_date, COALESCE(treatment_status,'pending'),
		          residual_likelihood, residual_impact,
		          created_at, updated_at`, dueDateExpr)

	var risk Risk
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&risk.ID, &risk.OrgID, &risk.Title, &risk.Description, &risk.Category,
		&risk.Likelihood, &risk.Impact, &risk.RiskScore, &risk.Owner,
		&risk.Status, &risk.Treatment, &risk.TreatmentNotes,
		&risk.TreatmentOption, &risk.TreatmentPlan, &risk.TreatmentOwner,
		&risk.TreatmentDueDate, &risk.TreatmentStatus,
		&risk.ResidualLikelihood, &risk.ResidualImpact,
		&risk.CreatedAt, &risk.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update risk treatment: %w", err)
	}
	return &risk, nil
}

func (r *Repository) CreateRisk(ctx context.Context, orgID string, in CreateRiskInput) (*Risk, error) {
	var risk Risk
	err := r.db.QueryRow(ctx, `
		INSERT INTO ck_risks (org_id, title, description, category, likelihood, impact, owner, treatment, treatment_notes)
		VALUES ($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id::text, org_id::text, title, COALESCE(description,''), COALESCE(category,''),
		          likelihood, impact, risk_score, COALESCE(owner,''), status, treatment,
		          COALESCE(treatment_notes,''),
		          COALESCE(treatment_option,''), COALESCE(treatment_plan,''), COALESCE(treatment_owner,''),
		          treatment_due_date, COALESCE(treatment_status,'pending'),
		          residual_likelihood, residual_impact,
		          created_at, updated_at`,
		orgID, in.Title, in.Description, in.Category,
		in.Likelihood, in.Impact, in.Owner, in.Treatment, in.TreatmentNotes,
	).Scan(&risk.ID, &risk.OrgID, &risk.Title, &risk.Description, &risk.Category,
		&risk.Likelihood, &risk.Impact, &risk.RiskScore, &risk.Owner,
		&risk.Status, &risk.Treatment, &risk.TreatmentNotes,
		&risk.TreatmentOption, &risk.TreatmentPlan, &risk.TreatmentOwner,
		&risk.TreatmentDueDate, &risk.TreatmentStatus,
		&risk.ResidualLikelihood, &risk.ResidualImpact,
		&risk.CreatedAt, &risk.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create risk: %w", err)
	}
	return &risk, nil
}

// --- Incident Register (FR-CK13) ---

const incidentSelectCols = `
	id::text, org_id::text, title, COALESCE(description,''), severity, status,
	discovered_at, resolved_at, COALESCE(affected_systems,'{}'), breach_id::text,
	incident_type, reporting_obligation, COALESCE(notification_authority,''),
	deadline_4h, deadline_24h, deadline_72h, deadline_30d,
	reported_4h_at, reported_24h_at, reported_72h_at, reported_30d_at,
	affected_customers, financial_impact_estimate, is_major_incident,
	supplier_id::text,
	COALESCE(notified_warn_24h,false), COALESCE(notified_warn_72h,false), COALESCE(notified_warn_30d,false),
	created_at, updated_at`

func scanIncident(row interface {
	Scan(dest ...any) error
}) (*Incident, error) {
	var inc Incident
	var supplierID *string
	err := row.Scan(
		&inc.ID, &inc.OrgID, &inc.Title, &inc.Description,
		&inc.Severity, &inc.Status, &inc.DiscoveredAt, &inc.ResolvedAt,
		&inc.AffectedSystems, &inc.BreachID,
		&inc.IncidentType, &inc.ReportingObligation, &inc.NotificationAuthority,
		&inc.Deadline4h, &inc.Deadline24h, &inc.Deadline72h, &inc.Deadline30d,
		&inc.Reported4hAt, &inc.Reported24hAt, &inc.Reported72hAt, &inc.Reported30dAt,
		&inc.AffectedCustomers, &inc.FinancialImpactEstimate, &inc.IsMajorIncident,
		&supplierID,
		&inc.NotifiedWarn24h, &inc.NotifiedWarn72h, &inc.NotifiedWarn30d,
		&inc.CreatedAt, &inc.UpdatedAt,
	)
	if supplierID != nil && *supplierID != "" {
		inc.SupplierID = supplierID
	}
	return &inc, err
}

func (r *Repository) ListIncidents(ctx context.Context, orgID string) ([]Incident, error) {
	rows, err := r.db.Query(ctx, `SELECT`+incidentSelectCols+`
		FROM ck_incidents WHERE org_id = $1::uuid ORDER BY discovered_at DESC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list incidents: %w", err)
	}
	defer rows.Close()

	var incidents []Incident
	for rows.Next() {
		inc, err := scanIncident(rows)
		if err != nil {
			return nil, fmt.Errorf("scan incident: %w", err)
		}
		incidents = append(incidents, *inc)
	}
	return incidents, rows.Err()
}

func (r *Repository) GetIncident(ctx context.Context, orgID, id string) (*Incident, error) {
	row := r.db.QueryRow(ctx, `SELECT`+incidentSelectCols+`
		FROM ck_incidents WHERE id = $1::uuid AND org_id = $2::uuid`, id, orgID)
	inc, err := scanIncident(row)
	if err != nil {
		return nil, fmt.Errorf("get incident: %w", err)
	}
	return inc, nil
}

func (r *Repository) UpdateIncident(ctx context.Context, orgID, id string, in UpdateIncidentInput) (*Incident, error) {
	resolvedAt := "NULL"
	if in.Status == "resolved" || in.Status == "closed" {
		resolvedAt = "now()"
	}
	incType := in.IncidentType
	if incType == "" {
		incType = "general"
	}
	obligation := in.ReportingObligation
	if obligation == "" {
		obligation = "unknown"
	}
	row := r.db.QueryRow(ctx, fmt.Sprintf(`
		UPDATE ck_incidents
		SET title=$3, description=$4, severity=$5, status=$6, affected_systems=$7,
		    resolved_at=CASE WHEN status IN ('resolved','closed') THEN resolved_at ELSE %s END,
		    incident_type=$8, reporting_obligation=$9, notification_authority=NULLIF($10,''),
		    affected_customers=$11, financial_impact_estimate=$12, is_major_incident=$13,
		    updated_at=now()
		WHERE id=$1::uuid AND org_id=$2::uuid
		RETURNING`+incidentSelectCols, resolvedAt),
		id, orgID, in.Title, in.Description, in.Severity, in.Status, in.AffectedSystems,
		incType, obligation, in.NotificationAuthority,
		in.AffectedCustomers, in.FinancialImpactEstimate, in.IsMajorIncident,
	)
	inc, err := scanIncident(row)
	if err != nil {
		return nil, fmt.Errorf("update incident: %w", err)
	}
	return inc, nil
}

func (r *Repository) CreateIncident(ctx context.Context, orgID string, in CreateIncidentInput, deadlines map[string]*time.Time) (*Incident, error) {
	incType := in.IncidentType
	if incType == "" {
		incType = "general"
	}
	obligation := in.ReportingObligation
	if obligation == "" {
		obligation = "unknown"
	}
	var d4h, d24h, d72h, d30d *time.Time
	if deadlines != nil {
		d4h = deadlines["4h"]
		d24h = deadlines["24h"]
		d72h = deadlines["72h"]
		d30d = deadlines["30d"]
	}
	row := r.db.QueryRow(ctx, `
		INSERT INTO ck_incidents (
			org_id, title, description, severity, discovered_at, affected_systems, breach_id,
			incident_type, reporting_obligation, notification_authority,
			deadline_4h, deadline_24h, deadline_72h, deadline_30d,
			affected_customers, financial_impact_estimate, is_major_incident
		) VALUES ($1::uuid,$2,$3,$4,$5,$6,$7::uuid,$8,$9,NULLIF($10,''),$11,$12,$13,$14,$15,$16,$17)
		RETURNING`+incidentSelectCols,
		orgID, in.Title, in.Description, in.Severity, in.DiscoveredAt,
		in.AffectedSystems, in.BreachID,
		incType, obligation, in.NotificationAuthority,
		d4h, d24h, d72h, d30d,
		in.AffectedCustomers, in.FinancialImpactEstimate, in.IsMajorIncident,
	)
	inc, err := scanIncident(row)
	if err != nil {
		return nil, fmt.Errorf("create incident: %w", err)
	}
	return inc, nil
}

// ListIncidentsByType returns all non-closed incidents of a specific type for an organisation.
func (r *Repository) ListIncidentsByType(ctx context.Context, orgID, incidentType string) ([]Incident, error) {
	rows, err := r.db.Query(ctx, `SELECT`+incidentSelectCols+`
		FROM ck_incidents WHERE org_id = $1::uuid AND incident_type = $2 AND status NOT IN ('closed') ORDER BY discovered_at DESC`, orgID, incidentType)
	if err != nil {
		return nil, fmt.Errorf("list incidents by type: %w", err)
	}
	defer rows.Close()

	var incidents []Incident
	for rows.Next() {
		inc, err := scanIncident(rows)
		if err != nil {
			return nil, fmt.Errorf("scan incident: %w", err)
		}
		incidents = append(incidents, *inc)
	}
	return incidents, rows.Err()
}

func (r *Repository) MarkDeadlineReported(ctx context.Context, orgID, id, deadline string) (*Incident, error) {
	// col is selected from a hardcoded allowlist — never user input reaches the query.
	col := map[string]string{
		"4h":  "reported_4h_at",
		"24h": "reported_24h_at",
		"72h": "reported_72h_at",
		"30d": "reported_30d_at",
	}[deadline]
	if col == "" {
		return nil, fmt.Errorf("unknown deadline: %s", deadline)
	}
	row := r.db.QueryRow(ctx, fmt.Sprintf(`
		UPDATE ck_incidents SET %s=NOW(), updated_at=NOW()
		WHERE id=$1::uuid AND org_id=$2::uuid
		RETURNING`+incidentSelectCols, col),
		id, orgID,
	)
	inc, err := scanIncident(row)
	if err != nil {
		return nil, fmt.Errorf("mark deadline reported: %w", err)
	}
	return inc, nil
}

// UpdateIncidentReportability stores the questionnaire answers and updates
// reporting_obligation, notification_authority, and gdpr_notification_required.
func (r *Repository) UpdateIncidentReportability(
	ctx context.Context,
	orgID, incidentID, obligation, authority string,
	gdprRequired bool,
	answersJSON []byte,
) error {
	_, err := r.db.Exec(ctx, `
		UPDATE ck_incidents
		SET reporting_obligation        = $3,
		    notification_authority      = $4,
		    gdpr_notification_required  = $5,
		    reportability_answers       = $6,
		    updated_at                  = NOW()
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		incidentID, orgID, obligation, authority, gdprRequired, answersJSON,
	)
	if err != nil {
		return fmt.Errorf("update incident reportability: %w", err)
	}
	return nil
}

// SaveIncidentReport archives a generated Meldungsformular with optional PDF bytes.
func (r *Repository) SaveIncidentReport(ctx context.Context, orgID, incidentID, reportType, authority string, pdfData []byte, metadata []byte) (*IncidentReport, error) {
	var report IncidentReport
	err := r.db.QueryRow(ctx, `
		INSERT INTO ck_incident_reports (org_id, incident_id, report_type, authority, pdf_data, metadata)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6::jsonb)
		RETURNING id::text, org_id::text, incident_id::text, report_type, authority, generated_at`,
		orgID, incidentID, reportType, authority, pdfData, metadata,
	).Scan(&report.ID, &report.OrgID, &report.IncidentID, &report.ReportType, &report.Authority, &report.GeneratedAt)
	if err != nil {
		return nil, fmt.Errorf("save incident report: %w", err)
	}
	return &report, nil
}

// ListIncidentReports returns all archived reports for a given incident.
func (r *Repository) ListIncidentReports(ctx context.Context, orgID, incidentID string) ([]IncidentReport, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, incident_id::text, report_type, authority, generated_at
		FROM ck_incident_reports
		WHERE org_id = $1::uuid AND incident_id = $2::uuid
		ORDER BY generated_at DESC`,
		orgID, incidentID,
	)
	if err != nil {
		return nil, fmt.Errorf("list incident reports: %w", err)
	}
	defer rows.Close()
	var reports []IncidentReport
	for rows.Next() {
		var r IncidentReport
		if err := rows.Scan(&r.ID, &r.OrgID, &r.IncidentID, &r.ReportType, &r.Authority, &r.GeneratedAt); err != nil {
			return nil, fmt.Errorf("scan incident report: %w", err)
		}
		reports = append(reports, r)
	}
	return reports, rows.Err()
}

// GetIncidentReportPDF returns the stored PDF bytes for a report entry.
func (r *Repository) GetIncidentReportPDF(ctx context.Context, orgID, reportID string) ([]byte, error) {
	var pdfData []byte
	err := r.db.QueryRow(ctx, `
		SELECT pdf_data FROM ck_incident_reports
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		reportID, orgID,
	).Scan(&pdfData)
	if err != nil {
		return nil, fmt.Errorf("get incident report pdf: %w", err)
	}
	return pdfData, nil
}

// MarkIncidentWarnNotified sets the notified_warn_* flag for a given deadline
// so the 12h-before warning is only sent once per incident + deadline pair.
func (r *Repository) MarkIncidentWarnNotified(ctx context.Context, orgID, incidentID, deadline string) error {
	// col is selected from a hardcoded allowlist — never user input reaches the query.
	col := map[string]string{
		"24h": "notified_warn_24h",
		"72h": "notified_warn_72h",
		"30d": "notified_warn_30d",
	}[deadline]
	if col == "" {
		return fmt.Errorf("unknown deadline: %s", deadline)
	}
	_, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE ck_incidents SET %s=true, updated_at=NOW()
		WHERE id=$1::uuid AND org_id=$2::uuid`, col),
		incidentID, orgID,
	)
	return err
}

// GetOrgSector returns the sector and federal_state for the given org.
func (r *Repository) GetOrgSector(ctx context.Context, orgID string) (*OrgSectorSettings, error) {
	var s OrgSectorSettings
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(sector,'other'), COALESCE(federal_state,'')
		FROM organizations WHERE id = $1::uuid`, orgID,
	).Scan(&s.Sector, &s.FederalState)
	if err != nil {
		return nil, fmt.Errorf("get org sector: %w", err)
	}
	return &s, nil
}

// UpdateOrgSector sets the sector and federal_state for the given org.
func (r *Repository) UpdateOrgSector(ctx context.Context, orgID, sector, federalState string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE organizations SET sector=$2, federal_state=NULLIF($3,''), updated_at=NOW()
		WHERE id=$1::uuid`, orgID, sector, federalState,
	)
	if err != nil {
		return fmt.Errorf("update org sector: %w", err)
	}
	return nil
}

// GetAdminEmails returns the e-mail addresses of active Admin users for the given org.
func (r *Repository) GetAdminEmails(ctx context.Context, orgID string) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.email
		FROM   org_members om
		JOIN   users u  ON u.id  = om.user_id
		JOIN   roles ro ON ro.id = om.role_id
		WHERE  om.org_id = $1::uuid
		  AND  ro.name   = 'Admin'
		  AND  u.is_active = true`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var emails []string
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			return nil, err
		}
		emails = append(emails, e)
	}
	return emails, rows.Err()
}

// --- Supplier Register (NIS2 Art. 21 / DORA Art. 28) ---

const supplierSelectCols = `
	id::text, org_id::text, name, COALESCE(contact_name,''), COALESCE(contact_email,''),
	COALESCE(service_type,''), criticality, nis2_relevant, dora_relevant,
	contract_end, COALESCE(notes,''), COALESCE(sub_suppliers,'{}'), COALESCE(data_location,''),
	exit_strategy_exists, assessment_status, last_assessment_at, created_at, updated_at`

func scanSupplier(row interface{ Scan(dest ...any) error }) (*Supplier, error) {
	var s Supplier
	err := row.Scan(
		&s.ID, &s.OrgID, &s.Name, &s.ContactName, &s.ContactEmail,
		&s.ServiceType, &s.Criticality, &s.NIS2Relevant, &s.DORARelevant,
		&s.ContractEnd, &s.Notes, &s.SubSuppliers, &s.DataLocation,
		&s.ExitStrategyExists, &s.AssessmentStatus, &s.LastAssessmentAt,
		&s.CreatedAt, &s.UpdatedAt,
	)
	return &s, err
}

func (r *Repository) ListSuppliers(ctx context.Context, orgID string, filter *SupplierFilter) ([]Supplier, error) {
	query := `SELECT` + supplierSelectCols + ` FROM ck_suppliers WHERE org_id = $1::uuid`
	args := []any{orgID}
	n := 2
	if filter != nil {
		if filter.Criticality != "" {
			query += fmt.Sprintf(" AND criticality = $%d", n)
			args = append(args, filter.Criticality)
			n++
		}
		if filter.AssessmentStatus != "" {
			query += fmt.Sprintf(" AND assessment_status = $%d", n)
			args = append(args, filter.AssessmentStatus)
			n++
		}
	}
	query += " ORDER BY name ASC LIMIT 1000"
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list suppliers: %w", err)
	}
	defer rows.Close()
	var suppliers []Supplier
	for rows.Next() {
		s, err := scanSupplier(rows)
		if err != nil {
			return nil, fmt.Errorf("scan supplier: %w", err)
		}
		suppliers = append(suppliers, *s)
	}
	return suppliers, rows.Err()
}

func (r *Repository) GetSupplier(ctx context.Context, orgID, id string) (*Supplier, error) {
	row := r.db.QueryRow(ctx, `SELECT`+supplierSelectCols+`
		FROM ck_suppliers WHERE id = $1::uuid AND org_id = $2::uuid`, id, orgID)
	s, err := scanSupplier(row)
	if err != nil {
		return nil, fmt.Errorf("get supplier: %w", err)
	}
	return s, nil
}

func (r *Repository) CreateSupplier(ctx context.Context, orgID string, in CreateSupplierInput) (*Supplier, error) {
	crit := in.Criticality
	if crit == "" {
		crit = "standard"
	}
	subSuppliers := in.SubSuppliers
	if subSuppliers == nil {
		subSuppliers = []string{}
	}
	assessmentStatus := in.AssessmentStatus
	if assessmentStatus == "" {
		assessmentStatus = "none"
	}
	row := r.db.QueryRow(ctx, `
		INSERT INTO ck_suppliers (org_id, name, contact_name, contact_email, service_type, criticality, nis2_relevant, dora_relevant, contract_end, notes, sub_suppliers, data_location, exit_strategy_exists, assessment_status, last_assessment_at)
		VALUES ($1::uuid,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),$6,$7,$8,$9,NULLIF($10,''),$11,NULLIF($12,''),$13,$14,$15)
		RETURNING`+supplierSelectCols,
		orgID, in.Name, in.ContactName, in.ContactEmail, in.ServiceType,
		crit, in.NIS2Relevant, in.DORARelevant, in.ContractEnd, in.Notes,
		subSuppliers, in.DataLocation, in.ExitStrategyExists,
		assessmentStatus, in.LastAssessmentAt,
	)
	s, err := scanSupplier(row)
	if err != nil {
		return nil, fmt.Errorf("create supplier: %w", err)
	}
	return s, nil
}

func (r *Repository) UpdateSupplier(ctx context.Context, orgID, id string, in UpdateSupplierInput) (*Supplier, error) {
	crit := in.Criticality
	if crit == "" {
		crit = "standard"
	}
	subSuppliers := in.SubSuppliers
	if subSuppliers == nil {
		subSuppliers = []string{}
	}
	assessmentStatus := in.AssessmentStatus
	if assessmentStatus == "" {
		assessmentStatus = "none"
	}
	row := r.db.QueryRow(ctx, `
		UPDATE ck_suppliers
		SET name=$3, contact_name=NULLIF($4,''), contact_email=NULLIF($5,''),
		    service_type=NULLIF($6,''), criticality=$7, nis2_relevant=$8, dora_relevant=$9,
		    contract_end=$10, notes=NULLIF($11,''), sub_suppliers=$12,
		    data_location=NULLIF($13,''), exit_strategy_exists=$14,
		    assessment_status=$15, last_assessment_at=$16, updated_at=now()
		WHERE id=$1::uuid AND org_id=$2::uuid
		RETURNING`+supplierSelectCols,
		id, orgID, in.Name, in.ContactName, in.ContactEmail, in.ServiceType,
		crit, in.NIS2Relevant, in.DORARelevant, in.ContractEnd, in.Notes,
		subSuppliers, in.DataLocation, in.ExitStrategyExists,
		assessmentStatus, in.LastAssessmentAt,
	)
	s, err := scanSupplier(row)
	if err != nil {
		return nil, fmt.Errorf("update supplier: %w", err)
	}
	return s, nil
}

func (r *Repository) DeleteSupplier(ctx context.Context, orgID, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM ck_suppliers WHERE id=$1::uuid AND org_id=$2::uuid`, id, orgID)
	if err != nil {
		return fmt.Errorf("delete supplier: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("supplier not found")
	}
	return nil
}

// LinkSupplierRisk links a risk to a supplier. Idempotent (ON CONFLICT DO NOTHING).
func (r *Repository) LinkSupplierRisk(ctx context.Context, orgID, supplierID, riskID string) error {
	// First verify the supplier belongs to the org.
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM ck_suppliers WHERE id=$1::uuid AND org_id=$2::uuid)`,
		supplierID, orgID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("verify supplier: %w", err)
	}
	if !exists {
		return fmt.Errorf("supplier not found")
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO ck_supplier_risks (supplier_id, risk_id)
		VALUES ($1::uuid, $2::uuid)
		ON CONFLICT DO NOTHING`,
		supplierID, riskID)
	if err != nil {
		return fmt.Errorf("link supplier risk: %w", err)
	}
	return nil
}

// UnlinkSupplierRisk removes a risk link from a supplier.
func (r *Repository) UnlinkSupplierRisk(ctx context.Context, orgID, supplierID, riskID string) error {
	// Verify the supplier belongs to the org.
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM ck_suppliers WHERE id=$1::uuid AND org_id=$2::uuid)`,
		supplierID, orgID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("verify supplier: %w", err)
	}
	if !exists {
		return fmt.Errorf("supplier not found")
	}
	_, err = r.db.Exec(ctx, `
		DELETE FROM ck_supplier_risks WHERE supplier_id=$1::uuid AND risk_id=$2::uuid`,
		supplierID, riskID)
	if err != nil {
		return fmt.Errorf("unlink supplier risk: %w", err)
	}
	return nil
}

// ListSupplierRisks returns all risks linked to the given supplier.
func (r *Repository) ListSupplierRisks(ctx context.Context, orgID, supplierID string) ([]Risk, error) {
	// Verify the supplier belongs to the org.
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM ck_suppliers WHERE id=$1::uuid AND org_id=$2::uuid)`,
		supplierID, orgID).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("verify supplier: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("supplier not found")
	}
	rows, err := r.db.Query(ctx, `
		SELECT r.id::text, r.org_id::text, r.title, COALESCE(r.description,''),
		       COALESCE(r.category,''), r.likelihood, r.impact, r.risk_score,
		       COALESCE(r.owner,''), r.status, r.treatment, COALESCE(r.treatment_notes,''),
		       COALESCE(r.treatment_option,''), COALESCE(r.treatment_plan,''), COALESCE(r.treatment_owner,''),
		       r.treatment_due_date, COALESCE(r.treatment_status,'pending'),
		       r.residual_likelihood, r.residual_impact,
		       r.created_at, r.updated_at
		FROM ck_risks r
		INNER JOIN ck_supplier_risks sr ON sr.risk_id = r.id
		WHERE sr.supplier_id = $1::uuid AND r.org_id = $2::uuid
		ORDER BY r.created_at DESC`,
		supplierID, orgID)
	if err != nil {
		return nil, fmt.Errorf("list supplier risks: %w", err)
	}
	defer rows.Close()
	var risks []Risk
	for rows.Next() {
		var rk Risk
		if err := rows.Scan(
			&rk.ID, &rk.OrgID, &rk.Title, &rk.Description,
			&rk.Category, &rk.Likelihood, &rk.Impact, &rk.RiskScore,
			&rk.Owner, &rk.Status, &rk.Treatment, &rk.TreatmentNotes,
			&rk.TreatmentOption, &rk.TreatmentPlan, &rk.TreatmentOwner,
			&rk.TreatmentDueDate, &rk.TreatmentStatus,
			&rk.ResidualLikelihood, &rk.ResidualImpact,
			&rk.CreatedAt, &rk.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan risk: %w", err)
		}
		risks = append(risks, rk)
	}
	return risks, rows.Err()
}

// ListIncidentsBySupplier returns all incidents linked to a given supplier via supplier_id FK.
func (r *Repository) ListIncidentsBySupplier(ctx context.Context, orgID, supplierID string) ([]Incident, error) {
	rows, err := r.db.Query(ctx, `SELECT`+incidentSelectCols+`
		FROM ck_incidents WHERE org_id = $1::uuid AND supplier_id = $2::uuid ORDER BY discovered_at DESC`, orgID, supplierID)
	if err != nil {
		return nil, fmt.Errorf("list incidents by supplier: %w", err)
	}
	defer rows.Close()

	var incidents []Incident
	for rows.Next() {
		inc, err := scanIncident(rows)
		if err != nil {
			return nil, fmt.Errorf("scan incident: %w", err)
		}
		incidents = append(incidents, *inc)
	}
	return incidents, rows.Err()
}

// --- AI System Inventory (EU AI Act) ---

const aiSystemSelectCols = `
	id::text, org_id::text, name, COALESCE(description,''), COALESCE(provider,''),
	COALESCE(use_case,''), COALESCE(affected_groups,''), autonomy_level,
	in_production_since, status, COALESCE(risk_class,''),
	COALESCE(classification_rationale,''), classified_at, COALESCE(classified_by,''),
	created_at, updated_at`

func scanAISystem(row interface{ Scan(dest ...any) error }) (*AISystem, error) {
	var a AISystem
	err := row.Scan(
		&a.ID, &a.OrgID, &a.Name, &a.Description, &a.Provider,
		&a.UseCase, &a.AffectedGroups, &a.AutonomyLevel,
		&a.InProductionSince, &a.Status, &a.RiskClass,
		&a.ClassificationRationale, &a.ClassifiedAt, &a.ClassifiedBy,
		&a.CreatedAt, &a.UpdatedAt,
	)
	return &a, err
}

func (r *Repository) ListAISystems(ctx context.Context, orgID string, filters AISystemFilters) ([]AISystem, error) {
	query := `SELECT` + aiSystemSelectCols + ` FROM ck_ai_systems WHERE org_id = $1::uuid`
	args := []any{orgID}
	n := 2
	if filters.RiskClass != "" {
		query += fmt.Sprintf(" AND risk_class = $%d", n)
		args = append(args, filters.RiskClass)
		n++
	}
	if filters.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", n)
		args = append(args, filters.Status)
		n++
	}
	query += " ORDER BY name ASC LIMIT 1000"
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list ai systems: %w", err)
	}
	defer rows.Close()
	var systems []AISystem
	for rows.Next() {
		a, err := scanAISystem(rows)
		if err != nil {
			return nil, fmt.Errorf("scan ai system: %w", err)
		}
		systems = append(systems, *a)
	}
	return systems, rows.Err()
}

func (r *Repository) DeleteAISystem(ctx context.Context, orgID, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM ck_ai_systems WHERE id = $1::uuid AND org_id = $2::uuid`, id, orgID)
	if err != nil {
		return fmt.Errorf("delete ai system: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) GetAISystem(ctx context.Context, orgID, id string) (*AISystem, error) {
	row := r.db.QueryRow(ctx, `SELECT`+aiSystemSelectCols+`
		FROM ck_ai_systems WHERE id = $1::uuid AND org_id = $2::uuid`, id, orgID)
	a, err := scanAISystem(row)
	if err != nil {
		return nil, fmt.Errorf("get ai system: %w", err)
	}
	return a, nil
}

func (r *Repository) CreateAISystem(ctx context.Context, orgID string, in CreateAISystemInput) (*AISystem, error) {
	al := in.AutonomyLevel
	if al == "" {
		al = "assistive"
	}
	row := r.db.QueryRow(ctx, `
		INSERT INTO ck_ai_systems (org_id, name, description, provider, use_case, affected_groups, autonomy_level, in_production_since, risk_class, classification_rationale)
		VALUES ($1::uuid,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),NULLIF($6,''),$7,$8,NULLIF($9,''),NULLIF($10,''))
		RETURNING`+aiSystemSelectCols,
		orgID, in.Name, in.Description, in.Provider, in.UseCase, in.AffectedGroups,
		al, in.InProductionSince, in.RiskClass, in.ClassificationRationale,
	)
	a, err := scanAISystem(row)
	if err != nil {
		return nil, fmt.Errorf("create ai system: %w", err)
	}
	return a, nil
}

func (r *Repository) UpdateAISystem(ctx context.Context, orgID, id string, in UpdateAISystemInput) (*AISystem, error) {
	al := in.AutonomyLevel
	if al == "" {
		al = "assistive"
	}
	st := in.Status
	if st == "" {
		st = "under_review"
	}
	var classifiedAt *time.Time
	if in.ClassifiedBy != "" && in.RiskClass != "" {
		now := time.Now()
		classifiedAt = &now
	}
	row := r.db.QueryRow(ctx, `
		UPDATE ck_ai_systems
		SET name=$3, description=NULLIF($4,''), provider=NULLIF($5,''),
		    use_case=NULLIF($6,''), affected_groups=NULLIF($7,''), autonomy_level=$8,
		    in_production_since=$9, status=$10, risk_class=NULLIF($11,''),
		    classification_rationale=NULLIF($12,''),
		    classified_at=COALESCE($13, classified_at), classified_by=NULLIF($14,''),
		    updated_at=now()
		WHERE id=$1::uuid AND org_id=$2::uuid
		RETURNING`+aiSystemSelectCols,
		id, orgID, in.Name, in.Description, in.Provider, in.UseCase, in.AffectedGroups,
		al, in.InProductionSince, st, in.RiskClass, in.ClassificationRationale,
		classifiedAt, in.ClassifiedBy,
	)
	a, err := scanAISystem(row)
	if err != nil {
		return nil, fmt.Errorf("update ai system: %w", err)
	}
	return a, nil
}

// --- Policy Management (FR-CK14) ---

// policySelectCols is the standard SELECT column list for ck_policies including versioning fields (Migration 076).
const policySelectCols = `
	id::text, org_id::text, title, COALESCE(description,''), COALESCE(category,''),
	status, COALESCE(version,''), effective_date, review_date, COALESCE(owner,''),
	created_at, updated_at,
	version_num, COALESCE(version_note,''), COALESCE(last_updated_by,''),
	reviewed_at, to_char(next_review_due, 'YYYY-MM-DD')`

// scanPolicyRow scans a policy from a row using policySelectCols column order.
func scanPolicyRow(row interface{ Scan(dest ...any) error }) (Policy, error) {
	var p Policy
	err := row.Scan(
		&p.ID, &p.OrgID, &p.Title, &p.Description, &p.Category,
		&p.Status, &p.Version, &p.EffectiveDate, &p.ReviewDate, &p.Owner,
		&p.CreatedAt, &p.UpdatedAt,
		&p.VersionNum, &p.VersionNote, &p.LastUpdatedBy,
		&p.ReviewedAt, &p.NextReviewDue,
	)
	return p, err
}

func (r *Repository) ListPolicies(ctx context.Context, orgID string) ([]Policy, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+policySelectCols+`
		FROM ck_policies
		WHERE org_id = $1::uuid
		ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list policies: %w", err)
	}
	defer rows.Close()

	var policies []Policy
	for rows.Next() {
		p, scanErr := scanPolicyRow(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan policy: %w", scanErr)
		}
		policies = append(policies, p)
	}
	return policies, rows.Err()
}

func (r *Repository) GetPolicy(ctx context.Context, orgID, id string) (*Policy, error) {
	row := r.db.QueryRow(ctx, `
		SELECT `+policySelectCols+`
		FROM ck_policies WHERE id = $1::uuid AND org_id = $2::uuid`, id, orgID)
	p, err := scanPolicyRow(row)
	if err != nil {
		return nil, fmt.Errorf("get policy: %w", err)
	}
	return &p, nil
}

// UpdatePolicy snapshots the current policy version into ck_policy_versions, then increments
// version_num and applies the update fields. All steps run in a single transaction.
func (r *Repository) UpdatePolicy(ctx context.Context, orgID, id string, in UpdatePolicyInput) (*Policy, error) {
	versionLabel := in.Version
	if versionLabel == "" {
		versionLabel = "1.0"
	}
	versionNote := ""
	if in.VersionNote != nil {
		versionNote = *in.VersionNote
	}
	updatedBy := ""
	if in.UpdatedBy != nil {
		updatedBy = *in.UpdatedBy
	}
	var nextReviewDue *string
	if in.NextReviewDue != nil && *in.NextReviewDue != "" {
		nextReviewDue = in.NextReviewDue
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Snapshot the current state into ck_policy_versions before updating.
	_, err = tx.Exec(ctx, `
		INSERT INTO ck_policy_versions (org_id, policy_id, version, title, content, status, version_note, updated_by)
		SELECT org_id, id, version_num, title, COALESCE(description,''), status,
		       COALESCE(version_note,''), COALESCE(last_updated_by,'')
		FROM ck_policies
		WHERE id = $1::uuid AND org_id = $2::uuid
		ON CONFLICT (policy_id, version) DO NOTHING`,
		id, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("snapshot policy version: %w", err)
	}

	// Determine whether to refresh reviewed_at (non-empty updatedBy signals a review).
	reviewedAt := "reviewed_at"
	if updatedBy != "" {
		reviewedAt = "now()"
	}

	row := tx.QueryRow(ctx, `
		UPDATE ck_policies
		SET title=$3, description=$4, category=$5, status=$6,
		    version=$7, effective_date=$8, review_date=$9, owner=$10,
		    version_num = version_num + 1,
		    version_note = $11,
		    last_updated_by = $12,
		    reviewed_at = `+reviewedAt+`,
		    next_review_due = $13::date,
		    updated_at = now()
		WHERE id=$1::uuid AND org_id=$2::uuid
		RETURNING `+policySelectCols,
		id, orgID, in.Title, in.Description, in.Category, in.Status,
		versionLabel, in.EffectiveDate, in.ReviewDate, in.Owner,
		versionNote, updatedBy, nextReviewDue,
	)
	p, err := scanPolicyRow(row)
	if err != nil {
		return nil, fmt.Errorf("update policy: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit policy update: %w", err)
	}
	return &p, nil
}

func (r *Repository) CreatePolicy(ctx context.Context, orgID string, in CreatePolicyInput) (*Policy, error) {
	version := in.Version
	if version == "" {
		version = "1.0"
	}
	row := r.db.QueryRow(ctx, `
		INSERT INTO ck_policies (org_id, title, description, category, version, effective_date, review_date, owner)
		VALUES ($1::uuid,$2,$3,$4,$5,$6,$7,$8)
		RETURNING `+policySelectCols,
		orgID, in.Title, in.Description, in.Category, version,
		in.EffectiveDate, in.ReviewDate, in.Owner,
	)
	p, err := scanPolicyRow(row)
	if err != nil {
		return nil, fmt.Errorf("create policy: %w", err)
	}
	return &p, nil
}

// ListPolicyVersions returns all historical version snapshots for a policy, newest first.
func (r *Repository) ListPolicyVersions(ctx context.Context, orgID, policyID string) ([]PolicyVersion, error) {
	rows, err := r.db.Query(ctx, `
		SELECT pv.id::text, pv.org_id::text, pv.policy_id::text, pv.version,
		       pv.title, pv.content, pv.status, pv.version_note, pv.updated_by, pv.created_at
		FROM ck_policy_versions pv
		WHERE pv.policy_id = $1::uuid AND pv.org_id = $2::uuid
		ORDER BY pv.version DESC`,
		policyID, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list policy versions: %w", err)
	}
	defer rows.Close()

	var versions []PolicyVersion
	for rows.Next() {
		var v PolicyVersion
		if err := rows.Scan(&v.ID, &v.OrgID, &v.PolicyID, &v.Version,
			&v.Title, &v.Content, &v.Status, &v.VersionNote, &v.UpdatedBy, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan policy version: %w", err)
		}
		versions = append(versions, v)
	}
	if versions == nil {
		versions = []PolicyVersion{}
	}
	return versions, rows.Err()
}

// GetPolicyVersion returns a single historical version snapshot.
func (r *Repository) GetPolicyVersion(ctx context.Context, orgID, policyID string, version int) (PolicyVersion, error) {
	var v PolicyVersion
	err := r.db.QueryRow(ctx, `
		SELECT pv.id::text, pv.org_id::text, pv.policy_id::text, pv.version,
		       pv.title, pv.content, pv.status, pv.version_note, pv.updated_by, pv.created_at
		FROM ck_policy_versions pv
		WHERE pv.policy_id = $1::uuid AND pv.org_id = $2::uuid AND pv.version = $3`,
		policyID, orgID, version,
	).Scan(&v.ID, &v.OrgID, &v.PolicyID, &v.Version,
		&v.Title, &v.Content, &v.Status, &v.VersionNote, &v.UpdatedBy, &v.CreatedAt)
	if err != nil {
		return PolicyVersion{}, fmt.Errorf("get policy version: %w", err)
	}
	return v, nil
}

// --- Internal Audit Records (FR-CK15) ---

func (r *Repository) ListAuditRecords(ctx context.Context, orgID string) ([]AuditRecord, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, title, COALESCE(scope,''), COALESCE(auditor,''),
		       audit_date, status, COALESCE(findings,''), COALESCE(recommendations,''),
		       created_at, updated_at
		FROM ck_audit_records
		WHERE org_id = $1::uuid
		ORDER BY audit_date DESC LIMIT 1000`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list audit records: %w", err)
	}
	defer rows.Close()

	var records []AuditRecord
	for rows.Next() {
		var rec AuditRecord
		if err := rows.Scan(&rec.ID, &rec.OrgID, &rec.Title, &rec.Scope, &rec.Auditor,
			&rec.AuditDate, &rec.Status, &rec.Findings, &rec.Recommendations,
			&rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan audit record: %w", err)
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

func (r *Repository) GetAuditRecord(ctx context.Context, orgID, id string) (*AuditRecord, error) {
	var rec AuditRecord
	err := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, title, COALESCE(scope,''), COALESCE(auditor,''),
		       audit_date, status, COALESCE(findings,''), COALESCE(recommendations,''),
		       created_at, updated_at
		FROM ck_audit_records WHERE id = $1::uuid AND org_id = $2::uuid`, id, orgID,
	).Scan(&rec.ID, &rec.OrgID, &rec.Title, &rec.Scope, &rec.Auditor,
		&rec.AuditDate, &rec.Status, &rec.Findings, &rec.Recommendations,
		&rec.CreatedAt, &rec.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get audit record: %w", err)
	}
	return &rec, nil
}

func (r *Repository) UpdateAuditRecord(ctx context.Context, orgID, id string, in UpdateAuditRecordInput) (*AuditRecord, error) {
	var rec AuditRecord
	err := r.db.QueryRow(ctx, `
		UPDATE ck_audit_records SET title=$3, scope=$4, auditor=$5, audit_date=$6,
		       status=$7, findings=$8, recommendations=$9, updated_at=now()
		WHERE id=$1::uuid AND org_id=$2::uuid
		RETURNING id::text, org_id::text, title, COALESCE(scope,''), COALESCE(auditor,''),
		          audit_date, status, COALESCE(findings,''), COALESCE(recommendations,''),
		          created_at, updated_at`,
		id, orgID, in.Title, in.Scope, in.Auditor, in.AuditDate,
		in.Status, in.Findings, in.Recommendations,
	).Scan(&rec.ID, &rec.OrgID, &rec.Title, &rec.Scope, &rec.Auditor,
		&rec.AuditDate, &rec.Status, &rec.Findings, &rec.Recommendations,
		&rec.CreatedAt, &rec.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update audit record: %w", err)
	}
	return &rec, nil
}

func (r *Repository) CreateAuditRecord(ctx context.Context, orgID string, in CreateAuditRecordInput) (*AuditRecord, error) {
	var rec AuditRecord
	err := r.db.QueryRow(ctx, `
		INSERT INTO ck_audit_records (org_id, title, scope, auditor, audit_date, findings, recommendations)
		VALUES ($1::uuid,$2,$3,$4,$5,$6,$7)
		RETURNING id::text, org_id::text, title, COALESCE(scope,''), COALESCE(auditor,''),
		          audit_date, status, COALESCE(findings,''), COALESCE(recommendations,''),
		          created_at, updated_at`,
		orgID, in.Title, in.Scope, in.Auditor, in.AuditDate,
		in.Findings, in.Recommendations,
	).Scan(&rec.ID, &rec.OrgID, &rec.Title, &rec.Scope, &rec.Auditor,
		&rec.AuditDate, &rec.Status, &rec.Findings, &rec.Recommendations,
		&rec.CreatedAt, &rec.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create audit record: %w", err)
	}
	return &rec, nil
}

// --- Control Tasks ---

func (r *Repository) ListControlTasks(ctx context.Context, orgID, controlID string) ([]ControlTask, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, control_id::text, org_id::text, text, completed, created_at, updated_at
		FROM ck_control_tasks
		WHERE control_id = $1::uuid AND org_id = $2::uuid
		ORDER BY created_at ASC`,
		controlID, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list control tasks: %w", err)
	}
	defer rows.Close()

	var tasks []ControlTask
	for rows.Next() {
		var t ControlTask
		if err := rows.Scan(&t.ID, &t.ControlID, &t.OrgID, &t.Text, &t.Completed, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan control task: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (r *Repository) CreateControlTask(ctx context.Context, orgID, controlID string, in CreateControlTaskInput) (*ControlTask, error) {
	var t ControlTask
	err := r.db.QueryRow(ctx, `
		INSERT INTO ck_control_tasks (control_id, org_id, text)
		VALUES ($1::uuid, $2::uuid, $3)
		RETURNING id::text, control_id::text, org_id::text, text, completed, created_at, updated_at`,
		controlID, orgID, in.Text,
	).Scan(&t.ID, &t.ControlID, &t.OrgID, &t.Text, &t.Completed, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create control task: %w", err)
	}
	return &t, nil
}

func (r *Repository) UpdateControlTask(ctx context.Context, orgID, controlID, taskID string, in UpdateControlTaskInput) (*ControlTask, error) {
	var t ControlTask
	err := r.db.QueryRow(ctx, `
		UPDATE ck_control_tasks
		SET completed = $1, updated_at = now()
		WHERE id = $2::uuid AND control_id = $3::uuid AND org_id = $4::uuid
		RETURNING id::text, control_id::text, org_id::text, text, completed, created_at, updated_at`,
		in.Completed, taskID, controlID, orgID,
	).Scan(&t.ID, &t.ControlID, &t.OrgID, &t.Text, &t.Completed, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update control task: %w", err)
	}
	return &t, nil
}

func (r *Repository) DeleteControlTask(ctx context.Context, orgID, controlID, taskID string) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM ck_control_tasks
		WHERE id = $1::uuid AND control_id = $2::uuid AND org_id = $3::uuid`,
		taskID, controlID, orgID,
	)
	if err != nil {
		return fmt.Errorf("delete control task: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("task not found")
	}
	return nil
}

// --- Risk ↔ Control Links ---

// LinkRiskControl creates a link between a risk and a control within an organisation.
func (r *Repository) LinkRiskControl(ctx context.Context, orgID, riskID, controlID string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO ck_risk_control_links (risk_id, control_id, org_id)
		VALUES ($1::uuid, $2::uuid, $3::uuid)
		ON CONFLICT DO NOTHING`,
		riskID, controlID, orgID,
	)
	if err != nil {
		return fmt.Errorf("link risk control: %w", err)
	}
	return nil
}

// UnlinkRiskControl removes the link between a risk and a control within an organisation.
func (r *Repository) UnlinkRiskControl(ctx context.Context, orgID, riskID, controlID string) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM ck_risk_control_links
		WHERE risk_id = $1::uuid AND control_id = $2::uuid AND org_id = $3::uuid`,
		riskID, controlID, orgID,
	)
	if err != nil {
		return fmt.Errorf("unlink risk control: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("link not found")
	}
	return nil
}

// --- Resilience Tests (DORA Art. 24-27) ---

const resilienceTestSelectCols = `
	id::text, org_id::text, type, COALESCE(scope,''), COALESCE(provider,''),
	test_date, COALESCE(summary,''), remediation_status, COALESCE(attachment_url,''),
	created_at, updated_at`

func scanResilienceTest(row interface{ Scan(dest ...any) error }) (*ResilienceTest, error) {
	var t ResilienceTest
	err := row.Scan(
		&t.ID, &t.OrgID, &t.Type, &t.Scope, &t.Provider,
		&t.TestDate, &t.Summary, &t.RemediationStatus, &t.AttachmentURL,
		&t.CreatedAt, &t.UpdatedAt,
	)
	return &t, err
}

// ListResilienceTests returns all resilience tests for an organisation, sorted by test_date DESC.
func (r *Repository) ListResilienceTests(ctx context.Context, orgID string) ([]ResilienceTest, error) {
	rows, err := r.db.Query(ctx, `SELECT`+resilienceTestSelectCols+`
		FROM ck_resilience_tests WHERE org_id = $1::uuid ORDER BY test_date DESC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list resilience tests: %w", err)
	}
	defer rows.Close()
	var tests []ResilienceTest
	for rows.Next() {
		t, err := scanResilienceTest(rows)
		if err != nil {
			return nil, fmt.Errorf("scan resilience test: %w", err)
		}
		tests = append(tests, *t)
	}
	return tests, rows.Err()
}

// GetResilienceTest returns a single resilience test by ID within an organisation.
// Returns an error containing "not found" if the test does not exist.
func (r *Repository) GetResilienceTest(ctx context.Context, orgID, id string) (*ResilienceTest, error) {
	row := r.db.QueryRow(ctx, `SELECT`+resilienceTestSelectCols+`
		FROM ck_resilience_tests WHERE id = $1::uuid AND org_id = $2::uuid`, id, orgID)
	t, err := scanResilienceTest(row)
	if err != nil {
		return nil, fmt.Errorf("resilience test not found: %w", err)
	}
	return t, nil
}

// CreateResilienceTest inserts a new resilience test entry and returns it.
func (r *Repository) CreateResilienceTest(ctx context.Context, orgID string, in CreateResilienceTestInput) (*ResilienceTest, error) {
	remStatus := in.RemediationStatus
	if remStatus == "" {
		remStatus = "open"
	}
	row := r.db.QueryRow(ctx, `
		INSERT INTO ck_resilience_tests (org_id, type, scope, provider, test_date, summary, remediation_status)
		VALUES ($1::uuid, $2, NULLIF($3,''), NULLIF($4,''), $5, NULLIF($6,''), $7)
		RETURNING`+resilienceTestSelectCols,
		orgID, in.Type, in.Scope, in.Provider, in.TestDate, in.Summary, remStatus,
	)
	t, err := scanResilienceTest(row)
	if err != nil {
		return nil, fmt.Errorf("create resilience test: %w", err)
	}
	return t, nil
}

// UpdateResilienceTest updates an existing resilience test entry and returns it.
func (r *Repository) UpdateResilienceTest(ctx context.Context, orgID, id string, in UpdateResilienceTestInput) (*ResilienceTest, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE ck_resilience_tests
		SET type=$3, scope=NULLIF($4,''), provider=NULLIF($5,''), test_date=$6,
		    summary=NULLIF($7,''), remediation_status=$8, updated_at=NOW()
		WHERE id=$1::uuid AND org_id=$2::uuid
		RETURNING`+resilienceTestSelectCols,
		id, orgID, in.Type, in.Scope, in.Provider, in.TestDate, in.Summary, in.RemediationStatus,
	)
	t, err := scanResilienceTest(row)
	if err != nil {
		return nil, fmt.Errorf("update resilience test: %w", err)
	}
	return t, nil
}

// DeleteResilienceTest removes a resilience test entry.
func (r *Repository) DeleteResilienceTest(ctx context.Context, orgID, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM ck_resilience_tests WHERE id=$1::uuid AND org_id=$2::uuid`, id, orgID)
	if err != nil {
		return fmt.Errorf("delete resilience test: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("resilience test not found")
	}
	return nil
}

// UpdateResilienceTestAttachment sets the attachment_url on a resilience test entry.
func (r *Repository) UpdateResilienceTestAttachment(ctx context.Context, orgID, id, url string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE ck_resilience_tests SET attachment_url=$3, updated_at=NOW()
		WHERE id=$1::uuid AND org_id=$2::uuid`, id, orgID, url)
	if err != nil {
		return fmt.Errorf("update resilience test attachment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("resilience test not found")
	}
	return nil
}

// --- Framework Mappings (Story 28.2) ---

// CreateMapping inserts a new cross-framework control mapping.
// Returns nil, nil (no error) when the mapping already exists (ON CONFLICT DO NOTHING).
func (r *Repository) CreateMapping(ctx context.Context, orgID, sourceControlID, targetControlID string) (*FrameworkMapping, error) {
	var m FrameworkMapping
	err := r.db.QueryRow(ctx, `
		INSERT INTO ck_framework_mappings (org_id, source_control_id, target_control_id)
		VALUES ($1::uuid, $2::uuid, $3::uuid)
		ON CONFLICT (org_id, source_control_id, target_control_id) DO NOTHING
		RETURNING id::text, org_id::text, source_control_id::text, target_control_id::text, created_at`,
		orgID, sourceControlID, targetControlID,
	).Scan(&m.ID, &m.OrgID, &m.SourceControlID, &m.TargetControlID, &m.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// ON CONFLICT DO NOTHING — mapping already exists, not an error.
			return nil, nil
		}
		return nil, fmt.Errorf("create mapping: %w", err)
	}
	return &m, nil
}

// ListMappingsByOrg returns all framework mappings for an organisation.
func (r *Repository) ListMappingsByOrg(ctx context.Context, orgID string) ([]FrameworkMapping, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, source_control_id::text, target_control_id::text, created_at
		FROM ck_framework_mappings
		WHERE org_id = $1::uuid
		ORDER BY created_at ASC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list mappings: %w", err)
	}
	defer rows.Close()

	var mappings []FrameworkMapping
	for rows.Next() {
		var m FrameworkMapping
		if err := rows.Scan(&m.ID, &m.OrgID, &m.SourceControlID, &m.TargetControlID, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan mapping: %w", err)
		}
		mappings = append(mappings, m)
	}
	return mappings, rows.Err()
}

// DeleteMapping removes a framework mapping by ID within an organisation.
func (r *Repository) DeleteMapping(ctx context.Context, orgID, mappingID string) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM ck_framework_mappings WHERE id = $1::uuid AND org_id = $2::uuid`,
		mappingID, orgID,
	)
	if err != nil {
		return fmt.Errorf("delete mapping: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("mapping not found")
	}
	return nil
}

// GetMappingsBySourceControlIDs returns mappings keyed by source_control_id for a set of source UUIDs.
func (r *Repository) GetMappingsBySourceControlIDs(ctx context.Context, orgID string, sourceIDs []string) (map[string]FrameworkMapping, error) {
	if len(sourceIDs) == 0 {
		return map[string]FrameworkMapping{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, source_control_id::text, target_control_id::text, created_at
		FROM ck_framework_mappings
		WHERE org_id = $1::uuid AND source_control_id = ANY($2::uuid[])`,
		orgID, sourceIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("get mappings by source ids: %w", err)
	}
	defer rows.Close()

	result := make(map[string]FrameworkMapping)
	for rows.Next() {
		var m FrameworkMapping
		if err := rows.Scan(&m.ID, &m.OrgID, &m.SourceControlID, &m.TargetControlID, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan mapping: %w", err)
		}
		result[m.SourceControlID] = m
	}
	return result, rows.Err()
}

// ListRiskControls returns all controls linked to a risk within an organisation.
func (r *Repository) ListRiskControls(ctx context.Context, orgID, riskID string) ([]Control, error) {
	rows, err := r.db.Query(ctx, `
		SELECT c.id::text, c.framework_id::text, c.org_id::text, c.control_id, c.title,
		       COALESCE(c.description, ''), c.domain, c.evidence_type, c.weight,
		       c.not_applicable, COALESCE(c.not_applicable_reason, ''),
		       COALESCE(c.manual_status, ''), c.maturity_score,
		       c.last_reviewed_at, c.review_interval_days, c.next_review_due,
		       c.last_reviewed_by, c.review_note
		FROM ck_controls c
		INNER JOIN ck_risk_control_links l
		       ON l.control_id = c.id AND l.org_id = c.org_id
		WHERE l.risk_id = $1::uuid AND l.org_id = $2::uuid
		ORDER BY c.control_id ASC`,
		riskID, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list risk controls: %w", err)
	}
	defer rows.Close()

	var controls []Control
	for rows.Next() {
		var c Control
		var nextReviewDue *time.Time
		if err := rows.Scan(&c.ID, &c.FrameworkID, &c.OrgID, &c.ControlID, &c.Title,
			&c.Description, &c.Domain, &c.EvidenceType, &c.Weight,
			&c.NotApplicable, &c.NotApplicableReason, &c.ManualStatus, &c.MaturityScore,
			&c.LastReviewedAt, &c.ReviewIntervalDays, &nextReviewDue,
			&c.LastReviewedBy, &c.ReviewNote); err != nil {
			return nil, fmt.Errorf("scan control: %w", err)
		}
		c.NextReviewDue = nextReviewDue
		c.IsReviewOverdue = nextReviewDue != nil && nextReviewDue.Before(time.Now())
		controls = append(controls, c)
	}
	return controls, rows.Err()
}

// --- Cross-Framework Mappings (global reference table) ---

// GetMappingsForControl returns all framework controls that map to/from the given control UUID.
// It resolves the global text-code table (ck_framework_control_mappings) to org-specific UUIDs via JOIN.
func (r *Repository) GetMappingsForControl(ctx context.Context, orgID, controlID string) ([]ControlMapping, error) {
	rows, err := r.db.Query(ctx, `
		SELECT m.id::text, m.source_framework, m.source_control_code,
		       m.target_framework, m.target_control_code, m.mapping_type,
		       c2.id::text, c2.title, f2.name
		FROM ck_framework_control_mappings m
		JOIN ck_controls c1 ON c1.control_id = m.source_control_code
		    AND c1.framework_id IN (
		        SELECT id FROM ck_frameworks
		        WHERE lower(name) LIKE '%' || lower(m.source_framework) || '%'
		          AND org_id = $1::uuid
		    )
		    AND c1.org_id = $1::uuid
		    AND c1.id = $2::uuid
		JOIN ck_controls c2 ON c2.control_id = m.target_control_code
		    AND c2.framework_id IN (
		        SELECT id FROM ck_frameworks
		        WHERE lower(name) LIKE '%' || lower(m.target_framework) || '%'
		          AND org_id = $1::uuid
		    )
		    AND c2.org_id = $1::uuid
		JOIN ck_frameworks f2 ON f2.id = c2.framework_id

		UNION

		SELECT m.id::text, m.target_framework, m.target_control_code,
		       m.source_framework, m.source_control_code, m.mapping_type,
		       c1.id::text, c1.title, f1.name
		FROM ck_framework_control_mappings m
		JOIN ck_controls c2 ON c2.control_id = m.target_control_code
		    AND c2.framework_id IN (
		        SELECT id FROM ck_frameworks
		        WHERE lower(name) LIKE '%' || lower(m.target_framework) || '%'
		          AND org_id = $1::uuid
		    )
		    AND c2.org_id = $1::uuid
		    AND c2.id = $2::uuid
		JOIN ck_controls c1 ON c1.control_id = m.source_control_code
		    AND c1.framework_id IN (
		        SELECT id FROM ck_frameworks
		        WHERE lower(name) LIKE '%' || lower(m.source_framework) || '%'
		          AND org_id = $1::uuid
		    )
		    AND c1.org_id = $1::uuid
		JOIN ck_frameworks f1 ON f1.id = c1.framework_id

		ORDER BY 4, 5`,
		orgID, controlID,
	)
	if err != nil {
		return nil, fmt.Errorf("get control mappings: %w", err)
	}
	defer rows.Close()

	var mappings []ControlMapping
	for rows.Next() {
		var m ControlMapping
		if err := rows.Scan(
			&m.ID, &m.SourceFramework, &m.SourceControlCode,
			&m.TargetFramework, &m.TargetControlCode, &m.MappingType,
			&m.TargetControlID, &m.TargetControlTitle, &m.TargetFrameworkName,
		); err != nil {
			return nil, fmt.Errorf("scan control mapping: %w", err)
		}
		mappings = append(mappings, m)
	}
	return mappings, rows.Err()
}

// SeedGlobalControlMapping inserts a single row into ck_framework_control_mappings,
// silently ignoring duplicates (ON CONFLICT DO NOTHING).
func (r *Repository) SeedGlobalControlMapping(ctx context.Context, srcFW, srcCode, tgtFW, tgtCode, mappingType string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO ck_framework_control_mappings
		  (source_framework, source_control_code, target_framework, target_control_code, mapping_type)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT DO NOTHING`,
		srcFW, srcCode, tgtFW, tgtCode, mappingType,
	)
	if err != nil {
		return fmt.Errorf("seed global control mapping %s/%s→%s/%s: %w", srcFW, srcCode, tgtFW, tgtCode, err)
	}
	return nil
}

// --- Questionnaire Builder (Story 29.2) ---

// CreateQuestionnaire inserts a new questionnaire for an organisation.
func (r *Repository) CreateQuestionnaire(ctx context.Context, orgID, name, description string, isTemplate bool) (*Questionnaire, error) {
	var q Questionnaire
	err := r.db.QueryRow(ctx, `
		INSERT INTO ck_questionnaires (org_id, name, description, is_template)
		VALUES ($1::uuid, $2, $3, $4)
		RETURNING id::text, org_id::text, name, COALESCE(description,''), is_template, created_at, updated_at`,
		orgID, name, description, isTemplate,
	).Scan(&q.ID, &q.OrgID, &q.Name, &q.Description, &q.IsTemplate, &q.CreatedAt, &q.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create questionnaire: %w", err)
	}
	return &q, nil
}

// GetQuestionnaire returns a questionnaire with its questions ordered by order_idx.
func (r *Repository) GetQuestionnaire(ctx context.Context, orgID, id string) (*Questionnaire, error) {
	var q Questionnaire
	err := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, name, COALESCE(description,''), is_template, created_at, updated_at
		FROM ck_questionnaires
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		id, orgID,
	).Scan(&q.ID, &q.OrgID, &q.Name, &q.Description, &q.IsTemplate, &q.CreatedAt, &q.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("questionnaire not found")
		}
		return nil, fmt.Errorf("get questionnaire: %w", err)
	}

	questions, err := r.ListQuestions(ctx, id)
	if err != nil {
		return nil, err
	}
	q.Questions = questions
	return &q, nil
}

// ListQuestionnaires returns questionnaires for an org, optionally filtered by is_template.
func (r *Repository) ListQuestionnaires(ctx context.Context, orgID string, isTemplate *bool) ([]Questionnaire, error) {
	query := `
		SELECT id::text, org_id::text, name, COALESCE(description,''), is_template, created_at, updated_at
		FROM ck_questionnaires
		WHERE org_id = $1::uuid`
	args := []any{orgID}
	if isTemplate != nil {
		query += ` AND is_template = $2`
		args = append(args, *isTemplate)
	}
	query += ` ORDER BY created_at ASC`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list questionnaires: %w", err)
	}
	defer rows.Close()

	var questionnaires []Questionnaire
	for rows.Next() {
		var q Questionnaire
		if err := rows.Scan(&q.ID, &q.OrgID, &q.Name, &q.Description, &q.IsTemplate, &q.CreatedAt, &q.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan questionnaire: %w", err)
		}
		questionnaires = append(questionnaires, q)
	}
	return questionnaires, rows.Err()
}

// UpdateQuestionnaire updates name/description/is_template of a questionnaire.
func (r *Repository) UpdateQuestionnaire(ctx context.Context, orgID, id, name, description string, isTemplate bool) (*Questionnaire, error) {
	var q Questionnaire
	err := r.db.QueryRow(ctx, `
		UPDATE ck_questionnaires
		SET name = $3, description = $4, is_template = $5, updated_at = NOW()
		WHERE id = $1::uuid AND org_id = $2::uuid
		RETURNING id::text, org_id::text, name, COALESCE(description,''), is_template, created_at, updated_at`,
		id, orgID, name, description, isTemplate,
	).Scan(&q.ID, &q.OrgID, &q.Name, &q.Description, &q.IsTemplate, &q.CreatedAt, &q.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("questionnaire not found")
		}
		return nil, fmt.Errorf("update questionnaire: %w", err)
	}
	return &q, nil
}

// DeleteQuestionnaire removes a questionnaire and its questions (cascade).
func (r *Repository) DeleteQuestionnaire(ctx context.Context, orgID, id string) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM ck_questionnaires WHERE id = $1::uuid AND org_id = $2::uuid`,
		id, orgID,
	)
	if err != nil {
		return fmt.Errorf("delete questionnaire: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("questionnaire not found")
	}
	return nil
}

// scanQuestionRow scans a single question from a QueryRow result.
func scanQuestionRow(row pgx.Row) (Question, error) {
	var q Question
	var optionsJSON []byte
	var controlID *string
	err := row.Scan(
		&q.ID, &q.QuestionnaireID, &q.OrderIdx, &q.QuestionText,
		&q.QuestionType, &optionsJSON, &q.Required, &controlID,
		&q.CreatedAt, &q.UpdatedAt,
	)
	if err != nil {
		return q, err
	}
	if len(optionsJSON) > 0 {
		_ = json.Unmarshal(optionsJSON, &q.Options)
	}
	q.ControlID = controlID
	return q, nil
}

// CreateQuestion inserts a new question into a questionnaire.
func (r *Repository) CreateQuestion(ctx context.Context, questionnaireID, questionText, questionType string, options []string, required bool, controlID *string) (*Question, error) {
	var maxIdx int
	_ = r.db.QueryRow(ctx, `
		SELECT COALESCE(MAX(order_idx)+1, 0) FROM ck_questionnaire_questions WHERE questionnaire_id = $1::uuid`,
		questionnaireID,
	).Scan(&maxIdx)

	var optionsJSON []byte
	if len(options) > 0 {
		var err error
		optionsJSON, err = json.Marshal(options)
		if err != nil {
			return nil, fmt.Errorf("marshal options: %w", err)
		}
	}

	row := r.db.QueryRow(ctx, `
		INSERT INTO ck_questionnaire_questions
		  (questionnaire_id, order_idx, question_text, question_type, options, required, control_id)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7)
		RETURNING id::text, questionnaire_id::text, order_idx, question_text, question_type,
		          options, required, control_id::text, created_at, updated_at`,
		questionnaireID, maxIdx, questionText, questionType, optionsJSON, required, controlID,
	)
	q, err := scanQuestionRow(row)
	if err != nil {
		return nil, fmt.Errorf("create question: %w", err)
	}
	return &q, nil
}

// GetQuestion returns a single question by ID.
func (r *Repository) GetQuestion(ctx context.Context, questionnaireID, questionID string) (*Question, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id::text, questionnaire_id::text, order_idx, question_text, question_type,
		       options, required, control_id::text, created_at, updated_at
		FROM ck_questionnaire_questions
		WHERE id = $1::uuid AND questionnaire_id = $2::uuid`,
		questionID, questionnaireID,
	)
	q, err := scanQuestionRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("question not found")
		}
		return nil, fmt.Errorf("get question: %w", err)
	}
	return &q, nil
}

// UpdateQuestion updates an existing question.
func (r *Repository) UpdateQuestion(ctx context.Context, questionnaireID, questionID, questionText, questionType string, options []string, required bool, controlID *string) (*Question, error) {
	var optionsJSON []byte
	if len(options) > 0 {
		var err error
		optionsJSON, err = json.Marshal(options)
		if err != nil {
			return nil, fmt.Errorf("marshal options: %w", err)
		}
	}

	row := r.db.QueryRow(ctx, `
		UPDATE ck_questionnaire_questions
		SET question_text = $3, question_type = $4, options = $5, required = $6,
		    control_id = $7, updated_at = NOW()
		WHERE id = $1::uuid AND questionnaire_id = $2::uuid
		RETURNING id::text, questionnaire_id::text, order_idx, question_text, question_type,
		          options, required, control_id::text, created_at, updated_at`,
		questionID, questionnaireID, questionText, questionType, optionsJSON, required, controlID,
	)
	q, err := scanQuestionRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("question not found")
		}
		return nil, fmt.Errorf("update question: %w", err)
	}
	return &q, nil
}

// DeleteQuestion removes a question.
func (r *Repository) DeleteQuestion(ctx context.Context, questionnaireID, questionID string) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM ck_questionnaire_questions
		WHERE id = $1::uuid AND questionnaire_id = $2::uuid`,
		questionID, questionnaireID,
	)
	if err != nil {
		return fmt.Errorf("delete question: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("question not found")
	}
	return nil
}

// ListQuestions returns all questions for a questionnaire ordered by order_idx.
func (r *Repository) ListQuestions(ctx context.Context, questionnaireID string) ([]Question, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, questionnaire_id::text, order_idx, question_text, question_type,
		       options, required, control_id::text, created_at, updated_at
		FROM ck_questionnaire_questions
		WHERE questionnaire_id = $1::uuid
		ORDER BY order_idx ASC`,
		questionnaireID,
	)
	if err != nil {
		return nil, fmt.Errorf("list questions: %w", err)
	}
	defer rows.Close()

	var questions []Question
	for rows.Next() {
		var q Question
		var optionsJSON []byte
		var controlID *string
		if err := rows.Scan(
			&q.ID, &q.QuestionnaireID, &q.OrderIdx, &q.QuestionText,
			&q.QuestionType, &optionsJSON, &q.Required, &controlID,
			&q.CreatedAt, &q.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan question: %w", err)
		}
		if len(optionsJSON) > 0 {
			_ = json.Unmarshal(optionsJSON, &q.Options)
		}
		q.ControlID = controlID
		questions = append(questions, q)
	}
	return questions, rows.Err()
}

// ReorderQuestions updates order_idx for each question ID in the provided slice using a batch.
func (r *Repository) ReorderQuestions(ctx context.Context, questionnaireID string, order []string) error {
	batch := &pgx.Batch{}
	for i, qID := range order {
		batch.Queue(`
			UPDATE ck_questionnaire_questions
			SET order_idx = $1, updated_at = NOW()
			WHERE id = $2::uuid AND questionnaire_id = $3::uuid`,
			i, qID, questionnaireID,
		)
	}
	results := r.db.SendBatch(ctx, batch)
	defer func() { _ = results.Close() }()
	for range order {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("reorder questions: %w", err)
		}
	}
	return nil
}

// CloneQuestionnaire copies a questionnaire and all its questions with new UUIDs.
func (r *Repository) CloneQuestionnaire(ctx context.Context, orgID, sourceID, name string) (*Questionnaire, error) {
	src, err := r.GetQuestionnaire(ctx, orgID, sourceID)
	if err != nil {
		return nil, fmt.Errorf("clone: get source: %w", err)
	}

	newQ, err := r.CreateQuestionnaire(ctx, orgID, name, src.Description, false)
	if err != nil {
		return nil, fmt.Errorf("clone: create questionnaire: %w", err)
	}

	for _, sq := range src.Questions {
		if _, err := r.CreateQuestion(ctx, newQ.ID, sq.QuestionText, sq.QuestionType, sq.Options, sq.Required, sq.ControlID); err != nil {
			return nil, fmt.Errorf("clone: copy question: %w", err)
		}
	}

	return r.GetQuestionnaire(ctx, orgID, newQ.ID)
}

// --- Supplier Portal Assessments (Story 29.3) ---

// CreateAssessment inserts a new supplier assessment record.
func (r *Repository) CreateAssessment(ctx context.Context, a Assessment) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO ck_supplier_assessments
			(org_id, supplier_id, questionnaire_id, token_hash, expires_at, status)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6)`,
		a.OrgID, a.SupplierID, a.QuestionnaireID, a.TokenHash, a.ExpiresAt, a.Status,
	)
	if err != nil {
		return fmt.Errorf("create assessment: %w", err)
	}
	return nil
}

// GetAssessmentByTokenHash looks up an assessment by its SHA-256 token hash.
func (r *Repository) GetAssessmentByTokenHash(ctx context.Context, hash string) (*Assessment, error) {
	var a Assessment
	err := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, supplier_id::text, questionnaire_id::text,
		       token_hash, expires_at, status, submitted_at, submitted_by_ip, user_agent, created_at
		FROM ck_supplier_assessments
		WHERE token_hash = $1`,
		hash,
	).Scan(
		&a.ID, &a.OrgID, &a.SupplierID, &a.QuestionnaireID,
		&a.TokenHash, &a.ExpiresAt, &a.Status, &a.SubmittedAt,
		&a.SubmittedByIP, &a.UserAgent, &a.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("assessment not found")
		}
		return nil, fmt.Errorf("get assessment by token: %w", err)
	}
	return &a, nil
}

// UpdateAssessmentStatus updates status and optional submission metadata.
// For terminal transitions (submitted/reviewed), the UPDATE is conditional on
// the current status not already being terminal — preventing double-submit races.
func (r *Repository) UpdateAssessmentStatus(ctx context.Context, id, status string, submittedAt *time.Time, submittedByIP, userAgent string) error {
	query := `
		UPDATE ck_supplier_assessments
		SET status = $2, submitted_at = $3, submitted_by_ip = $4, user_agent = $5
		WHERE id = $1::uuid`
	if status == "submitted" || status == "reviewed" {
		query += ` AND status NOT IN ('submitted','reviewed')`
	}

	tag, err := r.db.Exec(ctx, query, id, status, submittedAt, submittedByIP, userAgent)
	if err != nil {
		return fmt.Errorf("update assessment status: %w", err)
	}
	if tag.RowsAffected() == 0 && (status == "submitted" || status == "reviewed") {
		return fmt.Errorf("assessment already submitted")
	}
	return nil
}

// UpsertAnswers upserts a batch of answers using pgx Batch with INSERT ... ON CONFLICT DO UPDATE.
func (r *Repository) UpsertAnswers(ctx context.Context, assessmentID string, answers []AnswerInput) error {
	if len(answers) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, ans := range answers {
		var optionsJSON []byte
		if len(ans.AnswerOptions) > 0 {
			var jsonErr error
			optionsJSON, jsonErr = json.Marshal(ans.AnswerOptions)
			if jsonErr != nil {
				return fmt.Errorf("marshal answer_options: %w", jsonErr)
			}
		}
		batch.Queue(`
			INSERT INTO ck_supplier_answers
				(assessment_id, question_id, answer_text, answer_bool, answer_options, file_url)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5::jsonb, $6)
			ON CONFLICT (assessment_id, question_id) DO UPDATE
			SET answer_text = EXCLUDED.answer_text,
			    answer_bool = EXCLUDED.answer_bool,
			    answer_options = EXCLUDED.answer_options,
			    file_url = EXCLUDED.file_url,
			    updated_at = NOW()`,
			assessmentID, ans.QuestionID, ans.AnswerText, ans.AnswerBool, string(optionsJSON), ans.FileURL,
		)
	}
	results := r.db.SendBatch(ctx, batch)
	defer func() { _ = results.Close() }()
	for range answers {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("upsert answer: %w", err)
		}
	}
	return nil
}

// GetAssessmentWithQuestionnaire returns an assessment joined with its questionnaire and questions.
func (r *Repository) GetAssessmentWithQuestionnaire(ctx context.Context, id string) (*AssessmentWithQuestionnaire, error) {
	var a Assessment
	err := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, supplier_id::text, questionnaire_id::text,
		       token_hash, expires_at, status, submitted_at, submitted_by_ip, user_agent, created_at
		FROM ck_supplier_assessments
		WHERE id = $1::uuid`,
		id,
	).Scan(
		&a.ID, &a.OrgID, &a.SupplierID, &a.QuestionnaireID,
		&a.TokenHash, &a.ExpiresAt, &a.Status, &a.SubmittedAt,
		&a.SubmittedByIP, &a.UserAgent, &a.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("assessment not found")
		}
		return nil, fmt.Errorf("get assessment: %w", err)
	}

	// Fetch questionnaire — use org_id from assessment for ownership check.
	qnr, err := r.GetQuestionnaire(ctx, a.OrgID, a.QuestionnaireID)
	if err != nil {
		return nil, fmt.Errorf("get questionnaire for assessment: %w", err)
	}

	return &AssessmentWithQuestionnaire{
		Assessment:    a,
		Questionnaire: qnr,
	}, nil
}

// ListAssessmentsForSupplier returns all assessments for a given supplier within an org.
func (r *Repository) ListAssessmentsForSupplier(ctx context.Context, orgID, supplierID string) ([]Assessment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, supplier_id::text, questionnaire_id::text,
		       token_hash, expires_at, status, submitted_at, submitted_by_ip, user_agent, created_at
		FROM ck_supplier_assessments
		WHERE org_id = $1::uuid AND supplier_id = $2::uuid
		ORDER BY created_at DESC`,
		orgID, supplierID,
	)
	if err != nil {
		return nil, fmt.Errorf("list assessments: %w", err)
	}
	defer rows.Close()

	var assessments []Assessment
	for rows.Next() {
		var a Assessment
		if err := rows.Scan(
			&a.ID, &a.OrgID, &a.SupplierID, &a.QuestionnaireID,
			&a.TokenHash, &a.ExpiresAt, &a.Status, &a.SubmittedAt,
			&a.SubmittedByIP, &a.UserAgent, &a.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan assessment: %w", err)
		}
		assessments = append(assessments, a)
	}
	return assessments, rows.Err()
}

// UpdateSupplierAssessmentStatus sets assessment_status and last_assessment_at on a supplier row.
func (r *Repository) UpdateSupplierAssessmentStatus(ctx context.Context, orgID, supplierID, status string, at *time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE ck_suppliers
		SET assessment_status = $3, last_assessment_at = $4
		WHERE id = $2::uuid AND org_id = $1::uuid`,
		orgID, supplierID, status, at,
	)
	if err != nil {
		return fmt.Errorf("update supplier assessment status: %w", err)
	}
	return nil
}

// --- Assessment Review (Story 29.4) ---

// UpdateAnswerReview sets review_status and rework_note on a single answer.
func (r *Repository) UpdateAnswerReview(ctx context.Context, orgID, assessmentID, answerID, reviewStatus, reworkNote string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE ck_supplier_answers
		SET review_status = $1, rework_note = NULLIF($2,'')
		WHERE id = $3::uuid AND assessment_id = $4::uuid AND org_id = $5::uuid`,
		reviewStatus, reworkNote, answerID, assessmentID, orgID,
	)
	if err != nil {
		return fmt.Errorf("update answer review: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetAnswerWithQuestion fetches a single answer joined with its question (for evidence creation).
func (r *Repository) GetAnswerWithQuestion(ctx context.Context, orgID, answerID string) (*AnswerWithQuestion, error) {
	var a AnswerWithQuestion
	err := r.db.QueryRow(ctx, `
		SELECT sa.id::text, sa.assessment_id::text, sa.org_id::text,
		       sa.question_id::text, qq.question_text,
		       qq.control_id::text,
		       COALESCE(sa.answer_text,''), COALESCE(sa.file_url,''),
		       sa.review_status, sa.rework_note, sa.cert_expiry_date
		FROM ck_supplier_answers sa
		JOIN ck_questionnaire_questions qq ON qq.id = sa.question_id
		WHERE sa.id = $1::uuid AND sa.org_id = $2::uuid`,
		answerID, orgID,
	).Scan(
		&a.AnswerID, &a.AssessmentID, &a.OrgID,
		&a.QuestionID, &a.QuestionText, &a.ControlID,
		&a.AnswerText, &a.FileURL,
		&a.ReviewStatus, &a.ReworkNote, &a.CertExpiryDate,
	)
	if err != nil {
		return nil, fmt.Errorf("get answer with question: %w", err)
	}
	return &a, nil
}

// GetAnswersForAssessment returns all answers for an assessment with question info.
func (r *Repository) GetAnswersForAssessment(ctx context.Context, orgID, assessmentID string) ([]AnswerWithReview, error) {
	rows, err := r.db.Query(ctx, `
		SELECT sa.id::text, qq.question_text,
		       COALESCE(sa.answer_text,''), COALESCE(sa.file_url,''),
		       sa.review_status, sa.rework_note,
		       qq.control_id::text,
		       sa.cert_expiry_date, NULL::text
		FROM ck_supplier_answers sa
		JOIN ck_questionnaire_questions qq ON qq.id = sa.question_id
		WHERE sa.assessment_id = $1::uuid AND sa.org_id = $2::uuid
		ORDER BY qq.sort_order ASC`,
		assessmentID, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("get answers for assessment: %w", err)
	}
	defer rows.Close()
	var results []AnswerWithReview
	for rows.Next() {
		var a AnswerWithReview
		if err := rows.Scan(
			&a.ID, &a.QuestionText,
			&a.AnswerText, &a.FileURL,
			&a.ReviewStatus, &a.ReworkNote,
			&a.ControlID,
			&a.CertExpiryDate, &a.EvidenceID,
		); err != nil {
			return nil, fmt.Errorf("scan answer with review: %w", err)
		}
		results = append(results, a)
	}
	return results, rows.Err()
}

// MarkAssessmentReviewed atomically sets status=reviewed and updates the supplier's assessment_status.
func (r *Repository) MarkAssessmentReviewed(ctx context.Context, orgID, assessmentID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("mark assessment reviewed: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var supplierID string
	err = tx.QueryRow(ctx, `
		UPDATE ck_supplier_assessments
		SET status = 'reviewed', updated_at = now()
		WHERE id = $1::uuid AND org_id = $2::uuid AND status = 'submitted'
		RETURNING supplier_id::text`,
		assessmentID, orgID,
	).Scan(&supplierID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("mark assessment reviewed: update assessment: %w", err)
	}

	now := time.Now().UTC()
	_, err = tx.Exec(ctx, `
		UPDATE ck_suppliers
		SET assessment_status = 'completed', last_assessment_at = $3
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		supplierID, orgID, now,
	)
	if err != nil {
		return fmt.Errorf("mark assessment reviewed: update supplier: %w", err)
	}

	return tx.Commit(ctx)
}

// GetAssessmentsForSupplier returns all assessments for a supplier, newest first.
func (r *Repository) GetAssessmentsForSupplier(ctx context.Context, orgID, supplierID string) ([]Assessment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, supplier_id::text, questionnaire_id::text,
		       token_hash, expires_at, status, submitted_at, submitted_by_ip, user_agent, created_at
		FROM ck_supplier_assessments
		WHERE org_id = $1::uuid AND supplier_id = $2::uuid
		ORDER BY created_at DESC`,
		orgID, supplierID,
	)
	if err != nil {
		return nil, fmt.Errorf("get assessments for supplier: %w", err)
	}
	defer rows.Close()
	var results []Assessment
	for rows.Next() {
		var a Assessment
		if err := rows.Scan(
			&a.ID, &a.OrgID, &a.SupplierID, &a.QuestionnaireID,
			&a.TokenHash, &a.ExpiresAt, &a.Status, &a.SubmittedAt,
			&a.SubmittedByIP, &a.UserAgent, &a.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan assessment: %w", err)
		}
		results = append(results, a)
	}
	return results, rows.Err()
}

// FindExpiringCerts returns certificate answers whose cert_expiry_date is on or before the threshold.
func (r *Repository) FindExpiringCerts(ctx context.Context, orgID string, before time.Time) ([]CertExpiryWarning, error) {
	rows, err := r.db.Query(ctx, `
		SELECT sa.id::text, s.id::text, s.name,
		       qq.question_text, sa.cert_expiry_date, COALESCE(sa.file_url,'')
		FROM ck_supplier_answers sa
		JOIN ck_questionnaire_questions qq ON qq.id = sa.question_id
		JOIN ck_supplier_assessments asm ON asm.id = sa.assessment_id
		JOIN ck_suppliers s ON s.id = asm.supplier_id
		WHERE sa.org_id = $1::uuid
		  AND sa.cert_expiry_date IS NOT NULL
		  AND sa.cert_expiry_date <= $2
		  AND sa.file_url IS NOT NULL AND sa.file_url != ''`,
		orgID, before,
	)
	if err != nil {
		return nil, fmt.Errorf("find expiring certs: %w", err)
	}
	defer rows.Close()
	var results []CertExpiryWarning
	for rows.Next() {
		var w CertExpiryWarning
		if err := rows.Scan(
			&w.AnswerID, &w.SupplierID, &w.SupplierName,
			&w.QuestionText, &w.CertExpiryDate, &w.FileURL,
		); err != nil {
			return nil, fmt.Errorf("scan cert expiry warning: %w", err)
		}
		results = append(results, w)
	}
	return results, rows.Err()
}

// InsertAIClassification saves a new classification event and returns its ID.
func (r *Repository) InsertAIClassification(ctx context.Context, orgID, systemID string, in ClassifyAISystemInput) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		INSERT INTO ck_ai_classifications (org_id, ai_system_id, risk_class, rationale, classified_by, wizard_answers)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6)
		RETURNING id::text`,
		orgID, systemID, in.RiskClass, in.Rationale, in.ClassifiedBy, in.WizardAnswers,
	).Scan(&id)
	return id, err
}

// UpdateAISystemClassification sets the denormalized classification fields on the AI system row.
func (r *Repository) UpdateAISystemClassification(ctx context.Context, orgID, systemID string, in ClassifyAISystemInput) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE ck_ai_systems
		SET risk_class = $3, classification_rationale = $4, classified_by = $5,
		    classified_at = NOW(), status = 'approved', updated_at = NOW()
		WHERE id = $2::uuid AND org_id = $1::uuid`,
		orgID, systemID, in.RiskClass, in.Rationale, in.ClassifiedBy,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListAIClassifications returns the classification history for an AI system, newest first.
func (r *Repository) ListAIClassifications(ctx context.Context, orgID, systemID string) ([]AIClassification, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, ai_system_id::text, risk_class,
		       COALESCE(rationale,''), COALESCE(classified_by,''), wizard_answers, classified_at
		FROM ck_ai_classifications
		WHERE org_id = $1::uuid AND ai_system_id = $2::uuid
		ORDER BY classified_at DESC`,
		orgID, systemID,
	)
	if err != nil {
		return nil, fmt.Errorf("list ai classifications: %w", err)
	}
	defer rows.Close()
	var results []AIClassification
	for rows.Next() {
		var c AIClassification
		var wa []byte
		if err := rows.Scan(&c.ID, &c.OrgID, &c.AISystemID, &c.RiskClass,
			&c.Rationale, &c.ClassifiedBy, &wa, &c.ClassifiedAt); err != nil {
			return nil, fmt.Errorf("scan ai classification: %w", err)
		}
		if len(wa) > 0 {
			_ = json.Unmarshal(wa, &c.WizardAnswers)
		}
		results = append(results, c)
	}
	return results, rows.Err()
}

// UpsertAIDocumentation inserts or updates (creates a new version) the technical documentation for an AI system.
// Each save creates a new version row; returns the saved document.
func (r *Repository) UpsertAIDocumentation(ctx context.Context, orgID, systemID string, in UpsertAIDocumentationInput) (*AIDocumentation, error) {
	var nextVer int
	_ = r.db.QueryRow(ctx,
		`SELECT COALESCE(MAX(version),0)+1 FROM ck_ai_documentation WHERE org_id=$1::uuid AND ai_system_id=$2::uuid`,
		orgID, systemID,
	).Scan(&nextVer)

	var doc AIDocumentation
	err := r.db.QueryRow(ctx, `
		INSERT INTO ck_ai_documentation
		  (org_id, ai_system_id, version, system_description, intended_purpose,
		   training_data, data_quality, performance_metrics, system_limits,
		   risk_management, human_oversight, logging_audit_trail, authored_by, status)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id::text, org_id::text, ai_system_id::text, version,
		  COALESCE(system_description,''), COALESCE(intended_purpose,''),
		  COALESCE(training_data,''), COALESCE(data_quality,''),
		  COALESCE(performance_metrics,''), COALESCE(system_limits,''),
		  COALESCE(risk_management,''), COALESCE(human_oversight,''),
		  COALESCE(logging_audit_trail,''), COALESCE(authored_by,''), status,
		  created_at, updated_at`,
		orgID, systemID, nextVer,
		in.SystemDescription, in.IntendedPurpose,
		in.TrainingData, in.DataQuality, in.PerformanceMetrics, in.SystemLimits,
		in.RiskManagement, in.HumanOversight, in.LoggingAuditTrail, in.AuthoredBy,
		func() string {
			if in.Status == "" {
				return "draft"
			}
			return in.Status
		}(),
	).Scan(
		&doc.ID, &doc.OrgID, &doc.AISystemID, &doc.Version,
		&doc.SystemDescription, &doc.IntendedPurpose,
		&doc.TrainingData, &doc.DataQuality, &doc.PerformanceMetrics, &doc.SystemLimits,
		&doc.RiskManagement, &doc.HumanOversight, &doc.LoggingAuditTrail,
		&doc.AuthoredBy, &doc.Status, &doc.CreatedAt, &doc.UpdatedAt,
	)
	return &doc, err
}

// GetLatestAIDocumentation returns the most recent documentation version for an AI system.
func (r *Repository) GetLatestAIDocumentation(ctx context.Context, orgID, systemID string) (*AIDocumentation, error) {
	var doc AIDocumentation
	err := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, ai_system_id::text, version,
		  COALESCE(system_description,''), COALESCE(intended_purpose,''),
		  COALESCE(training_data,''), COALESCE(data_quality,''),
		  COALESCE(performance_metrics,''), COALESCE(system_limits,''),
		  COALESCE(risk_management,''), COALESCE(human_oversight,''),
		  COALESCE(logging_audit_trail,''), COALESCE(authored_by,''), status,
		  created_at, updated_at
		FROM ck_ai_documentation
		WHERE org_id=$1::uuid AND ai_system_id=$2::uuid
		ORDER BY version DESC LIMIT 1`,
		orgID, systemID,
	).Scan(
		&doc.ID, &doc.OrgID, &doc.AISystemID, &doc.Version,
		&doc.SystemDescription, &doc.IntendedPurpose,
		&doc.TrainingData, &doc.DataQuality, &doc.PerformanceMetrics, &doc.SystemLimits,
		&doc.RiskManagement, &doc.HumanOversight, &doc.LoggingAuditTrail,
		&doc.AuthoredBy, &doc.Status, &doc.CreatedAt, &doc.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// ListAIDocumentationVersions returns all versions of a system's documentation, newest first.
func (r *Repository) ListAIDocumentationVersions(ctx context.Context, orgID, systemID string) ([]AIDocumentation, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, ai_system_id::text, version,
		  COALESCE(system_description,''), COALESCE(intended_purpose,''),
		  COALESCE(training_data,''), COALESCE(data_quality,''),
		  COALESCE(performance_metrics,''), COALESCE(system_limits,''),
		  COALESCE(risk_management,''), COALESCE(human_oversight,''),
		  COALESCE(logging_audit_trail,''), COALESCE(authored_by,''), status,
		  created_at, updated_at
		FROM ck_ai_documentation
		WHERE org_id=$1::uuid AND ai_system_id=$2::uuid
		ORDER BY version DESC`,
		orgID, systemID,
	)
	if err != nil {
		return nil, fmt.Errorf("list ai doc versions: %w", err)
	}
	defer rows.Close()
	var results []AIDocumentation
	for rows.Next() {
		var doc AIDocumentation
		if err := rows.Scan(
			&doc.ID, &doc.OrgID, &doc.AISystemID, &doc.Version,
			&doc.SystemDescription, &doc.IntendedPurpose,
			&doc.TrainingData, &doc.DataQuality, &doc.PerformanceMetrics, &doc.SystemLimits,
			&doc.RiskManagement, &doc.HumanOversight, &doc.LoggingAuditTrail,
			&doc.AuthoredBy, &doc.Status, &doc.CreatedAt, &doc.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ai doc: %w", err)
		}
		results = append(results, doc)
	}
	return results, rows.Err()
}

// GetEUAIActStats returns aggregate counts needed for the EU AI Act dashboard.
func (r *Repository) GetEUAIActStats(ctx context.Context, orgID string) (total int, byRisk map[string]int, byStatus map[string]int, withoutDocs int, err error) {
	byRisk = map[string]int{}
	byStatus = map[string]int{}

	rows, err := r.db.Query(ctx,
		`SELECT COALESCE(risk_class,'unclassified'), status FROM ck_ai_systems WHERE org_id=$1::uuid`,
		orgID,
	)
	if err != nil {
		return 0, nil, nil, 0, fmt.Errorf("get eu ai act stats: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var rc, st string
		if err := rows.Scan(&rc, &st); err != nil {
			return 0, nil, nil, 0, err
		}
		total++
		byRisk[rc]++
		byStatus[st]++
	}
	if err := rows.Err(); err != nil {
		return 0, nil, nil, 0, err
	}

	// Count systems that have no entry in ck_ai_documentation.
	err = r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM ck_ai_systems s
		 WHERE s.org_id=$1::uuid
		   AND NOT EXISTS (SELECT 1 FROM ck_ai_documentation d WHERE d.ai_system_id=s.id)`,
		orgID,
	).Scan(&withoutDocs)
	return total, byRisk, byStatus, withoutDocs, err
}

// EvidenceForExport is a flattened view of evidence joined with its control,
// used exclusively by the audit-package ZIP generator.
type EvidenceForExport struct {
	ControlID        string
	ControlTitle     string
	ControlDomain    string // e.g. "A.5" from the control code
	EvidenceID       string
	EvidenceTitle    string
	EvidenceSource   string // 'manual', 'github', 'aws', etc.
	EvidenceDesc     string
	EvidenceFilePath string
	CollectedAt      time.Time
}

// ListEvidenceForFramework returns all evidence for all controls of a framework
// joined with control metadata. Controls without evidence are included with
// empty evidence fields so every control appears in the index PDF.
func (r *Repository) ListEvidenceForFramework(ctx context.Context, orgID, frameworkID string) ([]EvidenceForExport, error) {
	rows, err := r.db.Query(ctx, `
		SELECT c.id::text, c.title, COALESCE(c.control_id,''),
		       COALESCE(e.id::text,''), COALESCE(e.title,''), COALESCE(e.source,''),
		       COALESCE(e.description,''), COALESCE(e.file_path,''),
		       COALESCE(e.created_at, NOW())
		FROM ck_controls c
		LEFT JOIN ck_evidence e ON e.control_id = c.id AND e.org_id = $1::uuid
		WHERE c.framework_id = $2::uuid AND c.org_id = $1::uuid
		ORDER BY c.control_id ASC, e.created_at DESC`,
		orgID, frameworkID,
	)
	if err != nil {
		return nil, fmt.Errorf("list evidence for framework: %w", err)
	}
	defer rows.Close()

	var result []EvidenceForExport
	for rows.Next() {
		var row EvidenceForExport
		if err := rows.Scan(
			&row.ControlID, &row.ControlTitle, &row.ControlDomain,
			&row.EvidenceID, &row.EvidenceTitle, &row.EvidenceSource,
			&row.EvidenceDesc, &row.EvidenceFilePath,
			&row.CollectedAt,
		); err != nil {
			return nil, fmt.Errorf("scan evidence for export: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// --- Maßnahmen-Katalog (control measures) ---

// ListMeasures returns all measures for a control within an organisation, ordered by step_order.
func (r *Repository) ListMeasures(ctx context.Context, orgID, controlID string) ([]ControlMeasure, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, control_id::text, org_id::text, title, description,
		       difficulty, step_order, is_builtin, created_at
		FROM ck_control_measures
		WHERE org_id = $1::uuid AND control_id = $2::uuid
		ORDER BY step_order ASC, created_at ASC`,
		orgID, controlID,
	)
	if err != nil {
		return nil, fmt.Errorf("list measures: %w", err)
	}
	defer rows.Close()

	var measures []ControlMeasure
	for rows.Next() {
		var m ControlMeasure
		if err := rows.Scan(&m.ID, &m.ControlID, &m.OrgID, &m.Title, &m.Description,
			&m.Difficulty, &m.StepOrder, &m.IsBuiltin, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan measure: %w", err)
		}
		measures = append(measures, m)
	}
	return measures, rows.Err()
}

// CreateMeasure inserts a new measure for a control.
func (r *Repository) CreateMeasure(ctx context.Context, orgID, controlID string, in CreateMeasureInput) (ControlMeasure, error) {
	var m ControlMeasure
	err := r.db.QueryRow(ctx, `
		INSERT INTO ck_control_measures (control_id, org_id, title, description, difficulty, step_order, is_builtin)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, false)
		RETURNING id::text, control_id::text, org_id::text, title, description,
		          difficulty, step_order, is_builtin, created_at`,
		controlID, orgID, in.Title, in.Description, in.Difficulty, in.StepOrder,
	).Scan(&m.ID, &m.ControlID, &m.OrgID, &m.Title, &m.Description,
		&m.Difficulty, &m.StepOrder, &m.IsBuiltin, &m.CreatedAt)
	if err != nil {
		return ControlMeasure{}, fmt.Errorf("create measure: %w", err)
	}
	return m, nil
}

// UpdateMeasure updates editable fields of a measure.
func (r *Repository) UpdateMeasure(ctx context.Context, orgID, measureID string, in UpdateMeasureInput) (ControlMeasure, error) {
	var m ControlMeasure
	err := r.db.QueryRow(ctx, `
		UPDATE ck_control_measures
		SET title       = COALESCE($3, title),
		    description = COALESCE($4, description),
		    difficulty  = COALESCE($5, difficulty),
		    step_order  = COALESCE($6, step_order)
		WHERE id = $1::uuid AND org_id = $2::uuid
		RETURNING id::text, control_id::text, org_id::text, title, description,
		          difficulty, step_order, is_builtin, created_at`,
		measureID, orgID, in.Title, in.Description, in.Difficulty, in.StepOrder,
	).Scan(&m.ID, &m.ControlID, &m.OrgID, &m.Title, &m.Description,
		&m.Difficulty, &m.StepOrder, &m.IsBuiltin, &m.CreatedAt)
	if err != nil {
		return ControlMeasure{}, fmt.Errorf("update measure: %w", err)
	}
	return m, nil
}

// DeleteMeasure removes a non-builtin measure by ID.
func (r *Repository) DeleteMeasure(ctx context.Context, orgID, measureID string) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM ck_control_measures
		WHERE id = $1::uuid AND org_id = $2::uuid AND is_builtin = false`,
		measureID, orgID,
	)
	if err != nil {
		return fmt.Errorf("delete measure: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("measure not found or is builtin")
	}
	return nil
}

// SeedMeasuresForControl inserts builtin measures for a control, skipping duplicates by title.
func (r *Repository) SeedMeasuresForControl(ctx context.Context, orgID, controlID string, measures []CreateMeasureInput) error {
	for i, m := range measures {
		_, err := r.db.Exec(ctx, `
			INSERT INTO ck_control_measures (control_id, org_id, title, description, difficulty, step_order, is_builtin)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, true)
			ON CONFLICT DO NOTHING`,
			controlID, orgID, m.Title, m.Description, m.Difficulty, i,
		)
		if err != nil {
			return fmt.Errorf("seed measure %q: %w", m.Title, err)
		}
	}
	return nil
}

// FindControlByCode looks up a control UUID by its text control_id code within an org.
// Returns an empty string if not found.
func (r *Repository) FindControlByCode(ctx context.Context, orgID, code string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		SELECT id::text FROM ck_controls
		WHERE org_id = $1::uuid AND control_id = $2
		LIMIT 1`,
		orgID, code,
	).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("find control by code %q: %w", code, err)
	}
	return id, nil
}

// ListAllOrgs returns the IDs of all organisations.
// Used for cross-org seeding on startup.
func (r *Repository) ListAllOrgs(ctx context.Context) ([]string, error) {
	rows, err := r.db.Query(ctx, `SELECT id::text FROM organizations ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list all orgs: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan org id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// --- CAPA (Corrective and Preventive Actions) ---

// capaColumns is the canonical SELECT column list for CAPA rows.
const capaColumns = `id::text, org_id::text, source_type, source_id,
	title, description, root_cause, action_plan, assignee_email,
	due_date, priority, status, verification_note, closed_at, created_at, updated_at`

func scanCAPA(row interface {
	Scan(dest ...any) error
}) (CAPA, error) {
	var c CAPA
	err := row.Scan(
		&c.ID, &c.OrgID, &c.SourceType, &c.SourceID,
		&c.Title, &c.Description, &c.RootCause, &c.ActionPlan, &c.AssigneeEmail,
		&c.DueDate, &c.Priority, &c.Status, &c.VerificationNote, &c.ClosedAt, &c.CreatedAt, &c.UpdatedAt,
	)
	return c, err
}

// ListCAPAs returns CAPAs for an organisation, optionally filtered by status.
func (r *Repository) ListCAPAs(ctx context.Context, orgID string, statusFilter string) ([]CAPA, error) {
	query := `SELECT ` + capaColumns + ` FROM ck_capas WHERE org_id = $1::uuid`
	args := []any{orgID}
	if statusFilter != "" {
		query += " AND status = $2"
		args = append(args, statusFilter)
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list capas: %w", err)
	}
	defer rows.Close()

	var capas []CAPA
	for rows.Next() {
		c, err := scanCAPA(rows)
		if err != nil {
			return nil, fmt.Errorf("scan capa: %w", err)
		}
		capas = append(capas, c)
	}
	return capas, rows.Err()
}

// ListCAPAsForSource returns CAPAs linked to a specific source (audit/incident/risk).
func (r *Repository) ListCAPAsForSource(ctx context.Context, orgID, sourceType, sourceID string) ([]CAPA, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+capaColumns+` FROM ck_capas
		WHERE org_id = $1::uuid AND source_type = $2 AND source_id = $3
		ORDER BY created_at DESC`,
		orgID, sourceType, sourceID,
	)
	if err != nil {
		return nil, fmt.Errorf("list capas for source: %w", err)
	}
	defer rows.Close()

	var capas []CAPA
	for rows.Next() {
		c, err := scanCAPA(rows)
		if err != nil {
			return nil, fmt.Errorf("scan capa: %w", err)
		}
		capas = append(capas, c)
	}
	return capas, rows.Err()
}

// GetCAPA returns a single CAPA by ID within an organisation.
func (r *Repository) GetCAPA(ctx context.Context, orgID, capaID string) (CAPA, error) {
	row := r.db.QueryRow(ctx, `
		SELECT `+capaColumns+` FROM ck_capas
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		capaID, orgID,
	)
	c, err := scanCAPA(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CAPA{}, ErrNotFound
		}
		return CAPA{}, fmt.Errorf("get capa: %w", err)
	}
	return c, nil
}

// CreateCAPA inserts a new CAPA record.
func (r *Repository) CreateCAPA(ctx context.Context, orgID string, in CreateCAPAInput) (CAPA, error) {
	priority := in.Priority
	if priority == "" {
		priority = "medium"
	}

	var dueDateExpr interface{}
	if in.DueDate != nil && *in.DueDate != "" {
		dueDateExpr = *in.DueDate
	}

	row := r.db.QueryRow(ctx, `
		INSERT INTO ck_capas
			(org_id, source_type, source_id, title, description, assignee_email, due_date, priority)
		VALUES
			($1::uuid, $2, $3, $4, $5, $6, $7::date, $8)
		RETURNING `+capaColumns,
		orgID, in.SourceType, in.SourceID, in.Title, in.Description,
		in.AssigneeEmail, dueDateExpr, priority,
	)
	c, err := scanCAPA(row)
	if err != nil {
		return CAPA{}, fmt.Errorf("create capa: %w", err)
	}
	return c, nil
}

// UpdateCAPA applies partial updates to a CAPA using COALESCE.
// When status transitions to 'closed', closed_at is set to NOW().
func (r *Repository) UpdateCAPA(ctx context.Context, orgID, capaID string, in UpdateCAPAInput) (CAPA, error) {
	var dueDateStr *string
	if in.DueDate != nil && *in.DueDate != "" {
		dueDateStr = in.DueDate
	}

	row := r.db.QueryRow(ctx, `
		UPDATE ck_capas SET
			title             = COALESCE($3, title),
			description       = COALESCE($4, description),
			root_cause        = COALESCE($5, root_cause),
			action_plan       = COALESCE($6, action_plan),
			assignee_email    = COALESCE($7, assignee_email),
			due_date          = CASE WHEN $8::text IS NOT NULL THEN $8::date ELSE due_date END,
			priority          = COALESCE($9, priority),
			status            = COALESCE($10, status),
			verification_note = COALESCE($11, verification_note),
			closed_at         = CASE WHEN $10 = 'closed' AND closed_at IS NULL THEN NOW() ELSE closed_at END,
			updated_at        = NOW()
		WHERE id = $1::uuid AND org_id = $2::uuid
		RETURNING `+capaColumns,
		capaID, orgID,
		in.Title, in.Description, in.RootCause, in.ActionPlan, in.AssigneeEmail,
		dueDateStr, in.Priority, in.Status, in.VerificationNote,
	)
	c, err := scanCAPA(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CAPA{}, ErrNotFound
		}
		return CAPA{}, fmt.Errorf("update capa: %w", err)
	}
	return c, nil
}

// DeleteCAPA removes a CAPA record.
func (r *Repository) DeleteCAPA(ctx context.Context, orgID, capaID string) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM ck_capas WHERE id = $1::uuid AND org_id = $2::uuid`,
		capaID, orgID,
	)
	if err != nil {
		return fmt.Errorf("delete capa: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Collaborative Tasks ---

// scanTask scans a single task row from the provided row scanner.
func scanTask(s interface {
	Scan(dest ...any) error
}) (Task, error) {
	var t Task
	err := s.Scan(
		&t.ID, &t.OrgID, &t.EntityType, &t.EntityID,
		&t.Title, &t.Description, &t.AssigneeEmail, &t.DueDate,
		&t.Status, &t.Priority, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt,
	)
	return t, err
}

// ListTasks returns all tasks for the given entity, ordered newest first.
func (r *Repository) ListTasks(ctx context.Context, orgID, entityType, entityID string) ([]Task, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, entity_type, entity_id::text,
		       title, description, assignee_email, due_date,
		       status, priority, created_by, created_at, updated_at
		FROM ck_tasks
		WHERE org_id = $1::uuid AND entity_type = $2 AND entity_id = $3::uuid
		ORDER BY created_at DESC`,
		orgID, entityType, entityID,
	)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// CreateTask inserts a new task and returns the created row.
func (r *Repository) CreateTask(ctx context.Context, orgID, entityType, entityID string, in CreateTaskInput) (Task, error) {
	var dueDate *time.Time
	if in.DueDate != nil && *in.DueDate != "" {
		t, err := time.Parse("2006-01-02", *in.DueDate)
		if err != nil {
			return Task{}, fmt.Errorf("invalid due_date format (expected YYYY-MM-DD): %w", err)
		}
		dueDate = &t
	}
	status := in.Status
	if status == "" {
		status = "open"
	}
	priority := in.Priority
	if priority == "" {
		priority = "medium"
	}
	row := r.db.QueryRow(ctx, `
		INSERT INTO ck_tasks
		  (org_id, entity_type, entity_id, title, description, assignee_email, due_date, status, priority)
		VALUES ($1::uuid, $2, $3::uuid, $4, $5, $6, $7, $8, $9)
		RETURNING id::text, org_id::text, entity_type, entity_id::text,
		          title, description, assignee_email, due_date,
		          status, priority, created_by, created_at, updated_at`,
		orgID, entityType, entityID, in.Title, in.Description, in.AssigneeEmail, dueDate, status, priority,
	)
	t, err := scanTask(row)
	if err != nil {
		return Task{}, fmt.Errorf("create task: %w", err)
	}
	return t, nil
}

// UpdateTask applies partial updates to a task via COALESCE.
func (r *Repository) UpdateTask(ctx context.Context, orgID, taskID string, in UpdateTaskInput) (Task, error) {
	var dueDate *time.Time
	if in.DueDate != nil && *in.DueDate != "" {
		t, err := time.Parse("2006-01-02", *in.DueDate)
		if err != nil {
			return Task{}, fmt.Errorf("invalid due_date format (expected YYYY-MM-DD): %w", err)
		}
		dueDate = &t
	}
	row := r.db.QueryRow(ctx, `
		UPDATE ck_tasks
		SET title          = COALESCE($3, title),
		    description    = COALESCE($4, description),
		    assignee_email = COALESCE($5, assignee_email),
		    due_date       = COALESCE($6, due_date),
		    status         = COALESCE($7, status),
		    priority       = COALESCE($8, priority),
		    updated_at     = NOW()
		WHERE id = $1::uuid AND org_id = $2::uuid
		RETURNING id::text, org_id::text, entity_type, entity_id::text,
		          title, description, assignee_email, due_date,
		          status, priority, created_by, created_at, updated_at`,
		taskID, orgID, in.Title, in.Description, in.AssigneeEmail, dueDate, in.Status, in.Priority,
	)
	t, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Task{}, fmt.Errorf("task not found")
		}
		return Task{}, fmt.Errorf("update task: %w", err)
	}
	return t, nil
}

// DeleteTask removes a task.
func (r *Repository) DeleteTask(ctx context.Context, orgID, taskID string) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM ck_tasks WHERE id = $1::uuid AND org_id = $2::uuid`,
		taskID, orgID,
	)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("task not found")
	}
	return nil
}

// ListOverdueTasks returns tasks with due_date in the past that are not done.
func (r *Repository) ListOverdueTasks(ctx context.Context, orgID string) ([]Task, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, entity_type, entity_id::text,
		       title, description, assignee_email, due_date,
		       status, priority, created_by, created_at, updated_at
		FROM ck_tasks
		WHERE org_id = $1::uuid AND due_date < NOW() AND status != 'done'
		ORDER BY due_date ASC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list overdue tasks: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan overdue task: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// --- Comments ---

// ListComments returns all comments for an entity ordered chronologically.
func (r *Repository) ListComments(ctx context.Context, orgID, entityType, entityID string) ([]Comment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, entity_type, entity_id::text,
		       author_email, body, created_at
		FROM ck_comments
		WHERE org_id = $1::uuid AND entity_type = $2 AND entity_id = $3::uuid
		ORDER BY created_at ASC`,
		orgID, entityType, entityID,
	)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.OrgID, &c.EntityType, &c.EntityID,
			&c.AuthorEmail, &c.Body, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

// CreateComment inserts a new comment and returns the created row.
func (r *Repository) CreateComment(ctx context.Context, orgID, entityType, entityID string, in CreateCommentInput) (Comment, error) {
	var c Comment
	err := r.db.QueryRow(ctx, `
		INSERT INTO ck_comments (org_id, entity_type, entity_id, author_email, body)
		VALUES ($1::uuid, $2, $3::uuid, $4, $5)
		RETURNING id::text, org_id::text, entity_type, entity_id::text,
		          author_email, body, created_at`,
		orgID, entityType, entityID, in.AuthorEmail, in.Body,
	).Scan(&c.ID, &c.OrgID, &c.EntityType, &c.EntityID, &c.AuthorEmail, &c.Body, &c.CreatedAt)
	if err != nil {
		return Comment{}, fmt.Errorf("create comment: %w", err)
	}
	return c, nil
}

// DeleteComment removes a comment.
func (r *Repository) DeleteComment(ctx context.Context, orgID, commentID string) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM ck_comments WHERE id = $1::uuid AND org_id = $2::uuid`,
		commentID, orgID,
	)
	if err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("comment not found")
	}
	return nil
}

// --- Evidence Files (Migration 074) ---

// CreateEvidenceFile inserts a new evidence file record.
func (r *Repository) CreateEvidenceFile(ctx context.Context, f EvidenceFile) (EvidenceFile, error) {
	var out EvidenceFile
	var evidenceID *string
	if f.EvidenceID != "" {
		evidenceID = &f.EvidenceID
	}
	err := r.db.QueryRow(ctx, `
		INSERT INTO ck_evidence_files
		  (org_id, evidence_id, control_id, original_name, stored_name, mime_type, size_bytes, uploaded_by)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8)
		RETURNING id::text, org_id::text,
		          COALESCE(evidence_id::text, ''), control_id::text,
		          original_name, stored_name, mime_type, size_bytes, uploaded_by, created_at`,
		f.OrgID, evidenceID, f.ControlID,
		f.OriginalName, f.StoredName, f.MimeType, f.SizeBytes, f.UploadedBy,
	).Scan(
		&out.ID, &out.OrgID, &out.EvidenceID, &out.ControlID,
		&out.OriginalName, &out.StoredName, &out.MimeType, &out.SizeBytes, &out.UploadedBy, &out.CreatedAt,
	)
	if err != nil {
		return EvidenceFile{}, fmt.Errorf("create evidence file: %w", err)
	}
	return out, nil
}

// ListEvidenceFiles returns all files attached to an evidence record.
func (r *Repository) ListEvidenceFiles(ctx context.Context, orgID, evidenceID string) ([]EvidenceFile, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text,
		       COALESCE(evidence_id::text, ''), control_id::text,
		       original_name, stored_name, mime_type, size_bytes, uploaded_by, created_at
		FROM ck_evidence_files
		WHERE org_id = $1::uuid AND evidence_id = $2::uuid
		ORDER BY created_at DESC`,
		orgID, evidenceID,
	)
	if err != nil {
		return nil, fmt.Errorf("list evidence files: %w", err)
	}
	defer rows.Close()
	var items []EvidenceFile
	for rows.Next() {
		var f EvidenceFile
		if err := rows.Scan(
			&f.ID, &f.OrgID, &f.EvidenceID, &f.ControlID,
			&f.OriginalName, &f.StoredName, &f.MimeType, &f.SizeBytes, &f.UploadedBy, &f.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan evidence file: %w", err)
		}
		items = append(items, f)
	}
	return items, rows.Err()
}

// ListEvidenceFilesByControl returns all files attached to any evidence for a given control.
func (r *Repository) ListEvidenceFilesByControl(ctx context.Context, orgID, controlID string) ([]EvidenceFile, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text,
		       COALESCE(evidence_id::text, ''), control_id::text,
		       original_name, stored_name, mime_type, size_bytes, uploaded_by, created_at
		FROM ck_evidence_files
		WHERE org_id = $1::uuid AND control_id = $2::uuid
		ORDER BY created_at DESC`,
		orgID, controlID,
	)
	if err != nil {
		return nil, fmt.Errorf("list evidence files by control: %w", err)
	}
	defer rows.Close()
	var items []EvidenceFile
	for rows.Next() {
		var f EvidenceFile
		if err := rows.Scan(
			&f.ID, &f.OrgID, &f.EvidenceID, &f.ControlID,
			&f.OriginalName, &f.StoredName, &f.MimeType, &f.SizeBytes, &f.UploadedBy, &f.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan evidence file: %w", err)
		}
		items = append(items, f)
	}
	return items, rows.Err()
}

// GetEvidenceFile returns a single evidence file by ID within an organisation.
func (r *Repository) GetEvidenceFile(ctx context.Context, orgID, fileID string) (EvidenceFile, error) {
	var f EvidenceFile
	err := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text,
		       COALESCE(evidence_id::text, ''), control_id::text,
		       original_name, stored_name, mime_type, size_bytes, uploaded_by, created_at
		FROM ck_evidence_files
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		fileID, orgID,
	).Scan(
		&f.ID, &f.OrgID, &f.EvidenceID, &f.ControlID,
		&f.OriginalName, &f.StoredName, &f.MimeType, &f.SizeBytes, &f.UploadedBy, &f.CreatedAt,
	)
	if err != nil {
		return EvidenceFile{}, fmt.Errorf("get evidence file: %w", err)
	}
	return f, nil
}

// DeleteEvidenceFile removes an evidence file record and returns its metadata for disk deletion.
func (r *Repository) DeleteEvidenceFile(ctx context.Context, orgID, fileID string) (EvidenceFile, error) {
	var f EvidenceFile
	err := r.db.QueryRow(ctx, `
		DELETE FROM ck_evidence_files
		WHERE id = $1::uuid AND org_id = $2::uuid
		RETURNING id::text, org_id::text,
		          COALESCE(evidence_id::text, ''), control_id::text,
		          original_name, stored_name, mime_type, size_bytes, uploaded_by, created_at`,
		fileID, orgID,
	).Scan(
		&f.ID, &f.OrgID, &f.EvidenceID, &f.ControlID,
		&f.OriginalName, &f.StoredName, &f.MimeType, &f.SizeBytes, &f.UploadedBy, &f.CreatedAt,
	)
	if err != nil {
		return EvidenceFile{}, fmt.Errorf("delete evidence file: %w", err)
	}
	return f, nil
}

// --- Control Review Cycles (Migration 075) ---

// scanControl is a helper that scans the standard control SELECT columns including review fields.
func scanControl(row interface {
	Scan(dest ...any) error
}) (Control, error) {
	var c Control
	var nextReviewDue *time.Time
	err := row.Scan(
		&c.ID, &c.FrameworkID, &c.OrgID, &c.ControlID, &c.Title,
		&c.Description, &c.Domain, &c.EvidenceType, &c.Weight,
		&c.NotApplicable, &c.NotApplicableReason, &c.ManualStatus, &c.MaturityScore,
		&c.LastReviewedAt, &c.ReviewIntervalDays, &nextReviewDue,
		&c.LastReviewedBy, &c.ReviewNote,
	)
	if err != nil {
		return Control{}, err
	}
	c.NextReviewDue = nextReviewDue
	c.IsReviewOverdue = nextReviewDue != nil && nextReviewDue.Before(time.Now())
	return c, nil
}

// RecordControlReview records a review event for a control:
//   - Updates last_reviewed_at, review_interval_days, last_reviewed_by, review_note on ck_controls.
//   - Inserts a row into ck_control_reviews.
//   - Returns the updated control.
func (r *Repository) RecordControlReview(ctx context.Context, orgID, controlID string, in RecordReviewInput, statusAtReview string) (Control, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Control{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Determine interval: use provided value or keep existing.
	intervalExpr := "review_interval_days"
	if in.ReviewInterval > 0 {
		intervalExpr = fmt.Sprintf("%d", in.ReviewInterval)
	}

	q := fmt.Sprintf(`
		UPDATE ck_controls
		SET last_reviewed_at      = NOW(),
		    review_interval_days  = %s,
		    last_reviewed_by      = $3,
		    review_note           = $4
		WHERE id = $1::uuid AND org_id = $2::uuid
		RETURNING id::text, framework_id::text, org_id::text, control_id, title,
		          COALESCE(description,''), domain, evidence_type, weight,
		          not_applicable, COALESCE(not_applicable_reason,''),
		          COALESCE(manual_status,''), maturity_score,
		          last_reviewed_at, review_interval_days, next_review_due,
		          last_reviewed_by, review_note`, intervalExpr)

	c, err := scanControl(tx.QueryRow(ctx, q, controlID, orgID, in.ReviewedBy, in.ReviewNote))
	if err != nil {
		return Control{}, fmt.Errorf("update control for review: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO ck_control_reviews (org_id, control_id, reviewed_by, review_note, status_at_review)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5)`,
		orgID, controlID, in.ReviewedBy, in.ReviewNote, statusAtReview,
	)
	if err != nil {
		return Control{}, fmt.Errorf("insert control review: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Control{}, fmt.Errorf("commit review tx: %w", err)
	}
	return c, nil
}

// ListControlReviews returns the review history for a control, newest first.
func (r *Repository) ListControlReviews(ctx context.Context, orgID, controlID string) ([]ControlReview, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, control_id::text, reviewed_by, review_note, status_at_review, reviewed_at
		FROM ck_control_reviews
		WHERE control_id = $1::uuid AND org_id = $2::uuid
		ORDER BY reviewed_at DESC`,
		controlID, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list control reviews: %w", err)
	}
	defer rows.Close()

	var reviews []ControlReview
	for rows.Next() {
		var rv ControlReview
		if err := rows.Scan(&rv.ID, &rv.ControlID, &rv.ReviewedBy, &rv.ReviewNote, &rv.StatusAtReview, &rv.ReviewedAt); err != nil {
			return nil, fmt.Errorf("scan control review: %w", err)
		}
		reviews = append(reviews, rv)
	}
	return reviews, rows.Err()
}

// ListOverdueControls returns controls whose next_review_due is in the past, ordered by urgency.
func (r *Repository) ListOverdueControls(ctx context.Context, orgID string) ([]Control, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, framework_id::text, org_id::text, control_id, title,
		       COALESCE(description,''), domain, evidence_type, weight,
		       not_applicable, COALESCE(not_applicable_reason,''),
		       COALESCE(manual_status,''), maturity_score,
		       last_reviewed_at, review_interval_days, next_review_due,
		       last_reviewed_by, review_note
		FROM ck_controls
		WHERE org_id = $1::uuid
		  AND next_review_due IS NOT NULL
		  AND next_review_due < NOW()
		ORDER BY next_review_due ASC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list overdue controls: %w", err)
	}
	defer rows.Close()

	var controls []Control
	for rows.Next() {
		c, err := scanControl(rows)
		if err != nil {
			return nil, fmt.Errorf("scan overdue control: %w", err)
		}
		controls = append(controls, c)
	}
	return controls, rows.Err()
}

// --- Paginated list helpers (used by pagination-aware handlers) ---

// ListControlsPaged returns a page of controls plus the total count.
func (r *Repository) ListControlsPaged(ctx context.Context, orgID, frameworkID string, offset, limit int) ([]Control, int, error) {
	var total int
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM ck_controls WHERE framework_id = $1::uuid AND org_id = $2::uuid`,
		frameworkID, orgID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count controls: %w", err)
	}

	rows, err := r.db.Query(ctx, `
		SELECT id::text, framework_id::text, org_id::text, control_id, title,
		       COALESCE(description, ''), domain, evidence_type, weight,
		       not_applicable, COALESCE(not_applicable_reason, ''),
		       COALESCE(manual_status, ''), maturity_score,
		       last_reviewed_at, review_interval_days, next_review_due,
		       last_reviewed_by, review_note
		FROM ck_controls
		WHERE framework_id = $1::uuid AND org_id = $2::uuid
		ORDER BY control_id ASC
		LIMIT $3 OFFSET $4`,
		frameworkID, orgID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list controls paged: %w", err)
	}
	defer rows.Close()

	var controls []Control
	for rows.Next() {
		var c Control
		var nextReviewDue *time.Time
		if err := rows.Scan(&c.ID, &c.FrameworkID, &c.OrgID, &c.ControlID, &c.Title,
			&c.Description, &c.Domain, &c.EvidenceType, &c.Weight,
			&c.NotApplicable, &c.NotApplicableReason, &c.ManualStatus, &c.MaturityScore,
			&c.LastReviewedAt, &c.ReviewIntervalDays, &nextReviewDue,
			&c.LastReviewedBy, &c.ReviewNote); err != nil {
			return nil, 0, fmt.Errorf("scan control paged: %w", err)
		}
		c.NextReviewDue = nextReviewDue
		c.IsReviewOverdue = nextReviewDue != nil && nextReviewDue.Before(time.Now())
		controls = append(controls, c)
	}
	return controls, total, rows.Err()
}

// ListRisksPaged returns a page of risks plus the total count.
func (r *Repository) ListRisksPaged(ctx context.Context, orgID string, offset, limit int) ([]Risk, int, error) {
	var total int
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM ck_risks WHERE org_id = $1::uuid`, orgID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count risks: %w", err)
	}

	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, title, COALESCE(description,''), COALESCE(category,''),
		       likelihood, impact, risk_score, COALESCE(owner,''), status, treatment,
		       COALESCE(treatment_notes,''),
		       COALESCE(treatment_option,''), COALESCE(treatment_plan,''), COALESCE(treatment_owner,''),
		       treatment_due_date, COALESCE(treatment_status,'pending'),
		       residual_likelihood, residual_impact,
		       created_at, updated_at
		FROM ck_risks
		WHERE org_id = $1::uuid
		ORDER BY risk_score DESC, created_at DESC
		LIMIT $2 OFFSET $3`, orgID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list risks paged: %w", err)
	}
	defer rows.Close()

	var risks []Risk
	for rows.Next() {
		var r2 Risk
		if err := rows.Scan(&r2.ID, &r2.OrgID, &r2.Title, &r2.Description, &r2.Category,
			&r2.Likelihood, &r2.Impact, &r2.RiskScore, &r2.Owner,
			&r2.Status, &r2.Treatment, &r2.TreatmentNotes,
			&r2.TreatmentOption, &r2.TreatmentPlan, &r2.TreatmentOwner,
			&r2.TreatmentDueDate, &r2.TreatmentStatus,
			&r2.ResidualLikelihood, &r2.ResidualImpact,
			&r2.CreatedAt, &r2.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan risk paged: %w", err)
		}
		risks = append(risks, r2)
	}
	return risks, total, rows.Err()
}

// ListIncidentsPaged returns a page of incidents plus the total count.
func (r *Repository) ListIncidentsPaged(ctx context.Context, orgID string, offset, limit int) ([]Incident, int, error) {
	var total int
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM ck_incidents WHERE org_id = $1::uuid`, orgID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count incidents: %w", err)
	}

	rows, err := r.db.Query(ctx, `SELECT`+incidentSelectCols+`
		FROM ck_incidents WHERE org_id = $1::uuid ORDER BY discovered_at DESC
		LIMIT $2 OFFSET $3`, orgID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list incidents paged: %w", err)
	}
	defer rows.Close()

	var incidents []Incident
	for rows.Next() {
		inc, err := scanIncident(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan incident paged: %w", err)
		}
		incidents = append(incidents, *inc)
	}
	return incidents, total, rows.Err()
}

// ListPoliciesPaged returns a page of policies plus the total count.
func (r *Repository) ListPoliciesPaged(ctx context.Context, orgID string, offset, limit int) ([]Policy, int, error) {
	var total int
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM ck_policies WHERE org_id = $1::uuid`, orgID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count policies: %w", err)
	}

	rows, err := r.db.Query(ctx, `
		SELECT `+policySelectCols+`
		FROM ck_policies
		WHERE org_id = $1::uuid
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, orgID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list policies paged: %w", err)
	}
	defer rows.Close()

	var policies []Policy
	for rows.Next() {
		p, scanErr := scanPolicyRow(rows)
		if scanErr != nil {
			return nil, 0, fmt.Errorf("scan policy paged: %w", scanErr)
		}
		policies = append(policies, p)
	}
	return policies, total, rows.Err()
}

// ListCAPAsPaged returns a page of CAPAs plus the total count.
func (r *Repository) ListCAPAsPaged(ctx context.Context, orgID string, statusFilter string, offset, limit int) ([]CAPA, int, error) {
	where := "org_id = $1::uuid"
	countArgs := []any{orgID}
	dataArgs := []any{orgID}
	argN := 2

	if statusFilter != "" {
		where += fmt.Sprintf(" AND status = $%d", argN)
		countArgs = append(countArgs, statusFilter)
		dataArgs = append(dataArgs, statusFilter)
		argN++
	}

	var total int
	if err := r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM ck_capas WHERE %s`, where), countArgs...,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count capas: %w", err)
	}

	dataArgs = append(dataArgs, limit, offset)
	query := fmt.Sprintf(`SELECT `+capaColumns+` FROM ck_capas WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, argN, argN+1)

	rows, err := r.db.Query(ctx, query, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list capas paged: %w", err)
	}
	defer rows.Close()

	var capas []CAPA
	for rows.Next() {
		c, err := scanCAPA(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan capa paged: %w", err)
		}
		capas = append(capas, c)
	}
	return capas, total, rows.Err()
}

// --- Score History ---

// InsertScoreSnapshot inserts a compliance score snapshot for an organisation.
// frameworkID is optional (pass empty string for the org-wide snapshot).
func (r *Repository) InsertScoreSnapshot(ctx context.Context, orgID string, frameworkID *string, score float64, total, implemented int) error {
	if frameworkID != nil && *frameworkID == "" {
		frameworkID = nil
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO ck_score_history (org_id, framework_id, score, controls_total, controls_implemented)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5)`,
		orgID, frameworkID, score, total, implemented,
	)
	return err
}

// ScoreHistoryEntry is a single data point for the score trend chart.
type ScoreHistoryEntry struct {
	Date              string  `json:"date"`
	Score             float64 `json:"score"`
	ControlsTotal     int     `json:"controls_total"`
	ControlsImplemented int   `json:"controls_implemented"`
}

// GetScoreHistory returns aggregated daily score history for an organisation.
// framework_id is nil to query the org-wide score. Days is the look-back window.
func (r *Repository) GetScoreHistory(ctx context.Context, orgID string, days int) ([]ScoreHistoryEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			TO_CHAR(recorded_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') AS date,
			MAX(score) AS score,
			MAX(controls_total) AS controls_total,
			MAX(controls_implemented) AS controls_implemented
		FROM ck_score_history
		WHERE org_id = $1::uuid
		  AND framework_id IS NULL
		  AND recorded_at >= NOW() - ($2 || ' days')::INTERVAL
		GROUP BY date
		ORDER BY date ASC`,
		orgID, days,
	)
	if err != nil {
		return nil, fmt.Errorf("get score history: %w", err)
	}
	defer rows.Close()

	var entries []ScoreHistoryEntry
	for rows.Next() {
		var e ScoreHistoryEntry
		if err := rows.Scan(&e.Date, &e.Score, &e.ControlsTotal, &e.ControlsImplemented); err != nil {
			return nil, fmt.Errorf("scan score history: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// BulkUpdateControlStatus sets manual_status for all controls in ids that belong to the org.
func (r *Repository) BulkUpdateControlStatus(ctx context.Context, orgID string, ids []string, status string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE ck_controls SET manual_status = $3, updated_at = NOW()
		WHERE id = ANY($2::uuid[]) AND org_id = $1::uuid`,
		orgID, ids, status,
	)
	if err != nil {
		return fmt.Errorf("bulk update control status: %w", err)
	}
	return nil
}

// BulkUpdateCAPAStatus sets status for all CAPAs in ids that belong to the org.
func (r *Repository) BulkUpdateCAPAStatus(ctx context.Context, orgID string, ids []string, status string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE ck_capas SET status = $3, updated_at = NOW()
		WHERE id = ANY($2::uuid[]) AND org_id = $1::uuid`,
		orgID, ids, status,
	)
	if err != nil {
		return fmt.Errorf("bulk update capa status: %w", err)
	}
	return nil
}
