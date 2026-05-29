package vaktcomply

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/matharnica/vakt/internal/db"
)

// --- Controls ---

// BulkInsertControls inserts a slice of controls for a framework in a single transaction.
func (r *Repository) BulkInsertControls(ctx context.Context, controls []Control) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op when Commit succeeded
	qtx := r.q.WithTx(tx)

	for _, c := range controls {
		if err := qtx.BulkInsertCKControl(ctx, db.BulkInsertCKControlParams{
			FrameworkID:  c.FrameworkID,
			OrgID:        c.OrgID,
			ControlID:    c.ControlID,
			Title:        c.Title,
			Description:  ckOptText(c.Description),
			Domain:       c.Domain,
			EvidenceType: c.EvidenceType,
			Weight:       int32(c.Weight),
		}); err != nil {
			return fmt.Errorf("insert control %s: %w", c.ControlID, err)
		}
	}

	return tx.Commit(ctx)
}

// UpdateControl sets not_applicable, reason, manual_status, optionally maturity_score, and due_date on a control.
func (r *Repository) UpdateControl(ctx context.Context, orgID, controlID string, notApplicable bool, reason, manualStatus, owner string, maturityScore *int, dueDate *string) error {
	n, err := r.q.UpdateCKControl(ctx, db.UpdateCKControlParams{
		ID:            controlID,
		OrgID:         orgID,
		NotApplicable: notApplicable,
		Reason:        ckOptText(reason),
		ManualStatus:  ckOptText(manualStatus),
		Owner:         ckOptText(owner),
		MaturityScore: ckOptIntPtr(maturityScore),
		DueDate:       ckOptDatePtr(dueDate),
	})
	if err != nil {
		return fmt.Errorf("update control: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListControls returns all controls for a framework within an organisation.
// Uses ListCKControlsPaged with an explicit ceiling of 10 000 rows to prevent
// silent truncation on large BSI-Grundschutz or custom-control sets.
func (r *Repository) ListControls(ctx context.Context, orgID, frameworkID string) ([]Control, error) {
	rows, err := r.q.ListCKControlsPaged(ctx, db.ListCKControlsPagedParams{
		FrameworkID: frameworkID,
		OrgID:       orgID,
		Limit:       10_000,
		Offset:      0,
	})
	if err != nil {
		return nil, fmt.Errorf("list controls: %w", err)
	}
	out := make([]Control, 0, len(rows))
	for _, row := range rows {
		out = append(out, controlFromFields(controlFields{
			ID: row.ID, FrameworkID: row.FrameworkID, OrgID: row.OrgID,
			ControlID: row.ControlID, Title: row.Title, Description: row.Description,
			Domain: row.Domain, EvidenceType: row.EvidenceType, Weight: row.Weight,
			NotApplicable: row.NotApplicable, NotApplicableReason: row.NotApplicableReason,
			ManualStatus: row.ManualStatus, MaturityScore: row.MaturityScore, Owner: row.Owner,
			LastReviewedAt: row.LastReviewedAt, ReviewIntervalDays: row.ReviewIntervalDays,
			NextReviewDue: row.NextReviewDue, LastReviewedBy: row.LastReviewedBy,
			ReviewNote: row.ReviewNote, DueDate: row.DueDate,
		}))
	}
	return out, nil
}

// GetControl returns a single control by its UUID within an organisation.
func (r *Repository) GetControl(ctx context.Context, orgID, controlID string) (*Control, error) {
	row, err := r.q.GetCKControl(ctx, db.GetCKControlParams{ID: controlID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get control: %w", err)
	}
	c := controlFromFields(controlFields{
		ID: row.ID, FrameworkID: row.FrameworkID, OrgID: row.OrgID,
		ControlID: row.ControlID, Title: row.Title, Description: row.Description,
		Domain: row.Domain, EvidenceType: row.EvidenceType, Weight: row.Weight,
		NotApplicable: row.NotApplicable, NotApplicableReason: row.NotApplicableReason,
		ManualStatus: row.ManualStatus, MaturityScore: row.MaturityScore, Owner: row.Owner,
		LastReviewedAt: row.LastReviewedAt, ReviewIntervalDays: row.ReviewIntervalDays,
		NextReviewDue: row.NextReviewDue, LastReviewedBy: row.LastReviewedBy,
		ReviewNote: row.ReviewNote, DueDate: row.DueDate,
	})
	return &c, nil
}

// UpdateSoAMetadata persists the SoA-specific fields for a single control.
func (r *Repository) UpdateSoAMetadata(ctx context.Context, orgID, controlID, justification, implementation, responsible string) error {
	n, err := r.q.UpdateCKControlSoAMetadata(ctx, db.UpdateCKControlSoAMetadataParams{
		ID:             controlID,
		OrgID:          orgID,
		Justification:  ckOptText(justification),
		Implementation: ckOptText(implementation),
		Responsible:    ckOptText(responsible),
	})
	if err != nil {
		return fmt.Errorf("update soa metadata: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListControlsForSoA returns all controls for a framework with SoA metadata and evidence counts,
// ordered by control_id for consistent PDF output.
func (r *Repository) ListControlsForSoA(ctx context.Context, orgID, frameworkID string) ([]SoARow, error) {
	rows, err := r.q.ListCKControlsForSoA(ctx, db.ListCKControlsForSoAParams{FrameworkID: frameworkID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("list controls for soa: %w", err)
	}
	out := make([]SoARow, 0, len(rows))
	for _, row := range rows {
		out = append(out, SoARow{
			ControlID:      row.ControlID,
			Title:          row.Title,
			Domain:         row.Domain,
			Applicable:     row.Applicable,
			Justification:  row.Justification,
			Implementation: row.Implementation,
			Responsible:    row.Responsible,
			ManualStatus:   row.ManualStatus,
			EvidenceCount:  int(row.EvidenceCount),
		})
	}
	return out, nil
}

// GetSoAEntries returns all controls for the org's frameworks with SoA applicability metadata.
func (r *Repository) GetSoAEntries(ctx context.Context, orgID string) ([]SoAEntry, error) {
	rows, err := r.q.ListCKSoAEntries(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("get soa entries: %w", err)
	}
	entries := make([]SoAEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, SoAEntry{
			ControlID:                  row.ControlID,
			FrameworkName:              row.FrameworkName,
			Domain:                     row.Domain,
			Title:                      row.Title,
			Applicable:                 row.Applicable,
			Status:                     row.Status,
			JustificationApplicable:    row.JustYes,
			JustificationNotApplicable: row.JustNo,
		})
	}
	return entries, nil
}

// UpdateSoAApplicability sets the applicability and justification for a control.
func (r *Repository) UpdateSoAApplicability(ctx context.Context, orgID, controlID string, applicable bool, justYes, justNo string) error {
	return r.q.UpdateCKSoAApplicability(ctx, db.UpdateCKSoAApplicabilityParams{
		Applicable: applicable,
		JustYes:    ckOptText(justYes),
		JustNo:     ckOptText(justNo),
		ID:         controlID,
		OrgID:      orgID,
	})
}

// GetUserDisplayName returns the display_name (falling back to email) for a user.
func (r *Repository) GetUserDisplayName(ctx context.Context, userID string) (string, error) {
	return r.q.GetUserDisplayName(ctx, userID)
}

// GetMyTaskControls returns controls owned by a user in an org (by display name).
func (r *Repository) GetMyTaskControls(ctx context.Context, orgID, ownerDisplayName string) ([]MyTask, error) {
	rows, err := r.q.ListCKMyTaskControls(ctx, db.ListCKMyTaskControlsParams{
		OrgID: orgID,
		Owner: ownerDisplayName,
	})
	if err != nil {
		return nil, fmt.Errorf("list my task controls: %w", err)
	}
	tasks := make([]MyTask, 0, len(rows))
	for _, row := range rows {
		tasks = append(tasks, MyTask{
			ID:          row.ID,
			Title:       row.Title,
			Type:        "control",
			Status:      row.ManualStatus,
			FrameworkID: row.FrameworkID,
		})
	}
	return tasks, nil
}

// GetMyTaskRisks returns risks owned by a user in an org (by display name).
func (r *Repository) GetMyTaskRisks(ctx context.Context, orgID, ownerDisplayName string) ([]MyTask, error) {
	rows, err := r.q.ListCKMyTaskRisks(ctx, db.ListCKMyTaskRisksParams{
		OrgID: orgID,
		Owner: ownerDisplayName,
	})
	if err != nil {
		return nil, fmt.Errorf("list my task risks: %w", err)
	}
	tasks := make([]MyTask, 0, len(rows))
	for _, row := range rows {
		tasks = append(tasks, MyTask{
			ID:     row.ID,
			Title:  row.Title,
			Type:   "risk",
			Status: row.Status,
		})
	}
	return tasks, nil
}

// FindControlsByKeywords returns controls whose title or domain matches any of
// the given lowercase keywords. Used by cross-module evidence workers.
func (r *Repository) FindControlsByKeywords(ctx context.Context, orgID string, keywords []string) ([]Control, error) {
	if len(keywords) == 0 {
		return nil, nil
	}
	patterns := make([]string, len(keywords))
	for i, kw := range keywords {
		patterns[i] = "%" + strings.ToLower(kw) + "%"
	}
	rows, err := r.q.FindCKControlsByKeywords(ctx, db.FindCKControlsByKeywordsParams{
		OrgID:    orgID,
		Patterns: patterns,
	})
	if err != nil {
		return nil, fmt.Errorf("find controls by keywords: %w", err)
	}
	controls := make([]Control, 0, len(rows))
	for _, row := range rows {
		controls = append(controls, controlFromFields(controlFields{
			ID: row.ID, FrameworkID: row.FrameworkID, OrgID: row.OrgID,
			ControlID: row.ControlID, Title: row.Title, Description: row.Description,
			Domain: row.Domain, EvidenceType: row.EvidenceType, Weight: row.Weight,
			NotApplicable: row.NotApplicable, NotApplicableReason: row.NotApplicableReason,
			ManualStatus: row.ManualStatus, MaturityScore: row.MaturityScore, Owner: row.Owner,
			LastReviewedAt: row.LastReviewedAt, ReviewIntervalDays: row.ReviewIntervalDays,
			NextReviewDue: row.NextReviewDue, LastReviewedBy: row.LastReviewedBy,
			ReviewNote: row.ReviewNote, DueDate: row.DueDate,
		}))
	}
	return controls, nil
}

// FindPatchControls returns controls whose title or domain mentions patch,
// vulnerability, or update.  Used by the SecPulse auto-evidence worker to
// attach resolved-finding evidence to relevant compliance controls.
func (r *Repository) FindPatchControls(ctx context.Context, orgID string) ([]Control, error) {
	rows, err := r.q.FindCKPatchControls(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("find patch controls: %w", err)
	}
	controls := make([]Control, 0, len(rows))
	for _, row := range rows {
		controls = append(controls, controlFromFields(controlFields{
			ID: row.ID, FrameworkID: row.FrameworkID, OrgID: row.OrgID,
			ControlID: row.ControlID, Title: row.Title, Description: row.Description,
			Domain: row.Domain, EvidenceType: row.EvidenceType, Weight: row.Weight,
			NotApplicable: row.NotApplicable, NotApplicableReason: row.NotApplicableReason,
			ManualStatus: row.ManualStatus, MaturityScore: row.MaturityScore, Owner: row.Owner,
			LastReviewedAt: row.LastReviewedAt, ReviewIntervalDays: row.ReviewIntervalDays,
			NextReviewDue: row.NextReviewDue, LastReviewedBy: row.LastReviewedBy,
			ReviewNote: row.ReviewNote, DueDate: row.DueDate,
		}))
	}
	return controls, nil
}

// --- Control Tasks ---

// controlTaskFromCkControlTasks maps the sqlc row to the domain ControlTask.
func controlTaskFromCkControlTasks(r db.CkControlTasks) ControlTask {
	return ControlTask{
		ID:        r.ID,
		ControlID: r.ControlID,
		OrgID:     r.OrgID,
		Text:      r.Text,
		Completed: r.Completed,
		CreatedAt: ckTsToTime(r.CreatedAt),
		UpdatedAt: ckTsToTime(r.UpdatedAt),
	}
}

func (r *Repository) ListControlTasks(ctx context.Context, orgID, controlID string) ([]ControlTask, error) {
	rows, err := r.q.ListCKControlTasks(ctx, db.ListCKControlTasksParams{ControlID: controlID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("list control tasks: %w", err)
	}
	out := make([]ControlTask, 0, len(rows))
	for _, row := range rows {
		out = append(out, controlTaskFromCkControlTasks(row))
	}
	return out, nil
}

func (r *Repository) CreateControlTask(ctx context.Context, orgID, controlID string, in CreateControlTaskInput) (*ControlTask, error) {
	row, err := r.q.CreateCKControlTask(ctx, db.CreateCKControlTaskParams{
		ControlID: controlID,
		OrgID:     orgID,
		Text:      in.Text,
	})
	if err != nil {
		return nil, fmt.Errorf("create control task: %w", err)
	}
	t := controlTaskFromCkControlTasks(row)
	return &t, nil
}

func (r *Repository) UpdateControlTask(ctx context.Context, orgID, controlID, taskID string, in UpdateControlTaskInput) (*ControlTask, error) {
	row, err := r.q.UpdateCKControlTask(ctx, db.UpdateCKControlTaskParams{
		Completed: in.Completed,
		ID:        taskID,
		ControlID: controlID,
		OrgID:     orgID,
	})
	if err != nil {
		return nil, fmt.Errorf("update control task: %w", err)
	}
	t := controlTaskFromCkControlTasks(row)
	return &t, nil
}

func (r *Repository) DeleteControlTask(ctx context.Context, orgID, controlID, taskID string) error {
	n, err := r.q.DeleteCKControlTask(ctx, db.DeleteCKControlTaskParams{
		ID:        taskID,
		ControlID: controlID,
		OrgID:     orgID,
	})
	if err != nil {
		return fmt.Errorf("delete control task: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("task not found")
	}
	return nil
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
//
// embedded SQL by design — see Sitzung F-Wrap-Up commit. Die UPDATE-Query ist
// dynamisch (interval_expr wechselt zwischen "$5" und "review_interval_days"),
// das wäre in sqlc nur über zwei separate Queries machbar (with/without interval
// override). Da das Verhalten in einer Transaktion atomar bleiben muss und
// sqlc-WithTx-Pattern bereits etabliert ist, wäre die Aufteilung mehr Code
// für wenig Gewinn.
func (r *Repository) RecordControlReview(ctx context.Context, orgID, controlID string, in RecordReviewInput, statusAtReview string) (Control, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Control{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Determine interval: use provided value or keep existing.
	// Use a parameterised placeholder ($5) when a new interval is given to avoid SQL injection
	// via integer interpolation, even though in.ReviewInterval is typed as int.
	var (
		intervalExpr string
		queryArgs    []any
	)
	if in.ReviewInterval > 0 {
		intervalExpr = "$5"
		queryArgs = []any{controlID, orgID, in.ReviewedBy, in.ReviewNote, in.ReviewInterval}
	} else {
		intervalExpr = "review_interval_days"
		queryArgs = []any{controlID, orgID, in.ReviewedBy, in.ReviewNote}
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

	c, err := scanControl(tx.QueryRow(ctx, q, queryArgs...))
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
	rows, err := r.q.ListCKControlReviews(ctx, db.ListCKControlReviewsParams{
		ControlID: controlID,
		OrgID:     orgID,
	})
	if err != nil {
		return nil, fmt.Errorf("list control reviews: %w", err)
	}
	reviews := make([]ControlReview, 0, len(rows))
	for _, row := range rows {
		reviews = append(reviews, ControlReview{
			ID:             row.ID,
			ControlID:      row.ControlID,
			ReviewedBy:     row.ReviewedBy,
			ReviewNote:     row.ReviewNote,
			StatusAtReview: row.StatusAtReview,
			ReviewedAt:     ckTsToTime(row.ReviewedAt),
		})
	}
	return reviews, nil
}

// ListOverdueControls returns controls whose next_review_due is in the past, ordered by urgency.
func (r *Repository) ListOverdueControls(ctx context.Context, orgID string) ([]Control, error) {
	rows, err := r.q.ListCKOverdueControls(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list overdue controls: %w", err)
	}
	controls := make([]Control, 0, len(rows))
	for _, row := range rows {
		controls = append(controls, controlFromFields(controlFields{
			ID: row.ID, FrameworkID: row.FrameworkID, OrgID: row.OrgID,
			ControlID: row.ControlID, Title: row.Title, Description: row.Description,
			Domain: row.Domain, EvidenceType: row.EvidenceType, Weight: row.Weight,
			NotApplicable: row.NotApplicable, NotApplicableReason: row.NotApplicableReason,
			ManualStatus: row.ManualStatus, MaturityScore: row.MaturityScore, Owner: row.Owner,
			LastReviewedAt: row.LastReviewedAt, ReviewIntervalDays: row.ReviewIntervalDays,
			NextReviewDue: row.NextReviewDue, LastReviewedBy: row.LastReviewedBy,
			ReviewNote: row.ReviewNote, DueDate: row.DueDate,
		}))
	}
	return controls, nil
}

// --- Framework Mappings (Story 28.2) ---

func frameworkMappingFromCk(r db.CkFrameworkMappings) FrameworkMapping {
	return FrameworkMapping{
		ID:              r.ID,
		OrgID:           r.OrgID,
		SourceControlID: r.SourceControlID,
		TargetControlID: r.TargetControlID,
		CreatedAt:       ckTsToTime(r.CreatedAt),
	}
}

// CreateMapping inserts a new cross-framework control mapping.
// Returns nil, nil (no error) when the mapping already exists (ON CONFLICT DO NOTHING).
func (r *Repository) CreateMapping(ctx context.Context, orgID, sourceControlID, targetControlID string) (*FrameworkMapping, error) {
	row, err := r.q.CreateCKMapping(ctx, db.CreateCKMappingParams{
		OrgID:           orgID,
		SourceControlID: sourceControlID,
		TargetControlID: targetControlID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			// ON CONFLICT DO NOTHING — mapping already exists, not an error.
			return nil, nil
		}
		return nil, fmt.Errorf("create mapping: %w", err)
	}
	m := frameworkMappingFromCk(db.CkFrameworkMappings(row))
	return &m, nil
}

// ListMappingsByOrg returns all framework mappings for an organisation.
func (r *Repository) ListMappingsByOrg(ctx context.Context, orgID string) ([]FrameworkMapping, error) {
	rows, err := r.q.ListCKMappingsByOrg(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list mappings: %w", err)
	}
	out := make([]FrameworkMapping, 0, len(rows))
	for _, row := range rows {
		out = append(out, frameworkMappingFromCk(db.CkFrameworkMappings(row)))
	}
	return out, nil
}

// DeleteMapping removes a framework mapping by ID within an organisation.
func (r *Repository) DeleteMapping(ctx context.Context, orgID, mappingID string) error {
	n, err := r.q.DeleteCKMapping(ctx, db.DeleteCKMappingParams{ID: mappingID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete mapping: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("mapping not found")
	}
	return nil
}

// GetMappingsBySourceControlIDs returns mappings keyed by source_control_id for a set of source UUIDs.
func (r *Repository) GetMappingsBySourceControlIDs(ctx context.Context, orgID string, sourceIDs []string) (map[string]FrameworkMapping, error) {
	if len(sourceIDs) == 0 {
		return map[string]FrameworkMapping{}, nil
	}
	rows, err := r.q.GetCKMappingsBySourceControlIDs(ctx, db.GetCKMappingsBySourceControlIDsParams{
		OrgID:   orgID,
		Column2: sourceIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("get mappings by source ids: %w", err)
	}
	result := make(map[string]FrameworkMapping, len(rows))
	for _, row := range rows {
		m := frameworkMappingFromCk(db.CkFrameworkMappings(row))
		result[m.SourceControlID] = m
	}
	return result, nil
}

// --- Cross-Framework Mappings (global reference table) ---

// GetMappingsForControl returns all framework controls that map to/from the given control UUID.
// It resolves the global text-code table (ck_framework_control_mappings) to org-specific UUIDs via JOIN.
//
// embedded SQL by design — see Sitzung F-Wrap-Up commit. Diese UNION mit 4-stufigem
// JOIN-Chain (jeweils mit LIKE-Subquery zur Framework-Auflösung) ist sqlc-machbar,
// aber das resultierende Query-File würde ~50 Zeilen Aliase + Casts brauchen und
// der generierte Go-Code würde keine Lesbarkeit gewinnen. Diese Query ist
// stabil seit Sprint 3 und wird höchstens als „read-once" pro Page-Render aufgerufen.
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
	err := r.q.SeedCKGlobalControlMapping(ctx, db.SeedCKGlobalControlMappingParams{
		SourceFramework:   srcFW,
		SourceControlCode: srcCode,
		TargetFramework:   tgtFW,
		TargetControlCode: tgtCode,
		MappingType:       mappingType,
	})
	if err != nil {
		return fmt.Errorf("seed global control mapping %s/%s→%s/%s: %w", srcFW, srcCode, tgtFW, tgtCode, err)
	}
	return nil
}

// --- Risk ↔ Control Links ---

// LinkRiskControl creates a link between a risk and a control within an organisation.
func (r *Repository) LinkRiskControl(ctx context.Context, orgID, riskID, controlID string) error {
	if err := r.q.LinkCKRiskControl(ctx, db.LinkCKRiskControlParams{
		RiskID: riskID, ControlID: controlID, OrgID: orgID,
	}); err != nil {
		return fmt.Errorf("link risk control: %w", err)
	}
	return nil
}

// UnlinkRiskControl removes the link between a risk and a control within an organisation.
func (r *Repository) UnlinkRiskControl(ctx context.Context, orgID, riskID, controlID string) error {
	n, err := r.q.UnlinkCKRiskControl(ctx, db.UnlinkCKRiskControlParams{
		RiskID: riskID, ControlID: controlID, OrgID: orgID,
	})
	if err != nil {
		return fmt.Errorf("unlink risk control: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("link not found")
	}
	return nil
}

// ListRiskControls returns all controls linked to a risk within an organisation.
func (r *Repository) ListRiskControls(ctx context.Context, orgID, riskID string) ([]Control, error) {
	rows, err := r.q.ListCKRiskControls(ctx, db.ListCKRiskControlsParams{RiskID: riskID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("list risk controls: %w", err)
	}
	out := make([]Control, 0, len(rows))
	for _, row := range rows {
		out = append(out, controlFromFields(controlFields{
			ID: row.ID, FrameworkID: row.FrameworkID, OrgID: row.OrgID,
			ControlID: row.ControlID, Title: row.Title, Description: row.Description,
			Domain: row.Domain, EvidenceType: row.EvidenceType, Weight: row.Weight,
			NotApplicable: row.NotApplicable, NotApplicableReason: row.NotApplicableReason,
			ManualStatus: row.ManualStatus, MaturityScore: row.MaturityScore, Owner: row.Owner,
			LastReviewedAt: row.LastReviewedAt, ReviewIntervalDays: row.ReviewIntervalDays,
			NextReviewDue: row.NextReviewDue, LastReviewedBy: row.LastReviewedBy,
			ReviewNote: row.ReviewNote, DueDate: row.DueDate,
		}))
	}
	return out, nil
}

// --- Paginated + cursor control helpers ---

// ListControlsPaged returns a page of controls plus the total count.
func (r *Repository) ListControlsPaged(ctx context.Context, orgID, frameworkID string, offset, limit int) ([]Control, int, error) {
	total, err := r.q.CountCKControls(ctx, db.CountCKControlsParams{
		FrameworkID: frameworkID,
		OrgID:       orgID,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count controls: %w", err)
	}
	rows, err := r.q.ListCKControlsPaged(ctx, db.ListCKControlsPagedParams{
		FrameworkID: frameworkID,
		OrgID:       orgID,
		Limit:       int32(limit),
		Offset:      int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list controls paged: %w", err)
	}
	controls := make([]Control, 0, len(rows))
	for _, row := range rows {
		controls = append(controls, controlFromFields(controlFields{
			ID: row.ID, FrameworkID: row.FrameworkID, OrgID: row.OrgID,
			ControlID: row.ControlID, Title: row.Title, Description: row.Description,
			Domain: row.Domain, EvidenceType: row.EvidenceType, Weight: row.Weight,
			NotApplicable: row.NotApplicable, NotApplicableReason: row.NotApplicableReason,
			ManualStatus: row.ManualStatus, MaturityScore: row.MaturityScore, Owner: row.Owner,
			LastReviewedAt: row.LastReviewedAt, ReviewIntervalDays: row.ReviewIntervalDays,
			NextReviewDue: row.NextReviewDue, LastReviewedBy: row.LastReviewedBy,
			ReviewNote: row.ReviewNote, DueDate: row.DueDate,
		}))
	}
	return controls, int(total), nil
}

// ListControlsCursor returns controls using keyset pagination ordered by (control_id ASC, id ASC).
// Returns limit+1 rows so the caller can detect HasMore.
func (r *Repository) ListControlsCursor(ctx context.Context, orgID, frameworkID string, cursorControlID, cursorID string, limit int) ([]Control, error) {
	const baseQ = `
		SELECT id, framework_id, org_id, control_id, title, description, domain,
		       evidence_type, weight,
		       not_applicable, not_applicable_reason, manual_status, maturity_score, owner,
		       last_reviewed_at, review_interval_days, next_review_due,
		       last_reviewed_by, review_note, due_date
		FROM ck_controls
		WHERE framework_id = $1 AND org_id = $2`

	args := []any{frameworkID, orgID}
	q := baseQ
	n := 3
	if cursorControlID != "" && cursorID != "" {
		args = append(args, cursorControlID, cursorID)
		q += fmt.Sprintf(" AND (control_id > $%d OR (control_id = $%d AND id::text > $%d))", n, n, n+1)
		n += 2
	}
	args = append(args, int32(limit+1))
	q += fmt.Sprintf(" ORDER BY control_id ASC, id ASC LIMIT $%d", n)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list controls cursor: %w", err)
	}
	defer rows.Close()

	var out []Control
	for rows.Next() {
		var f controlFields
		if err := rows.Scan(
			&f.ID, &f.FrameworkID, &f.OrgID, &f.ControlID, &f.Title, &f.Description, &f.Domain,
			&f.EvidenceType, &f.Weight,
			&f.NotApplicable, &f.NotApplicableReason, &f.ManualStatus, &f.MaturityScore, &f.Owner,
			&f.LastReviewedAt, &f.ReviewIntervalDays, &f.NextReviewDue,
			&f.LastReviewedBy, &f.ReviewNote, &f.DueDate,
		); err != nil {
			return nil, fmt.Errorf("scan control cursor row: %w", err)
		}
		out = append(out, controlFromFields(f))
	}
	return out, rows.Err()
}

// BulkUpdateControlStatus sets manual_status for all controls in ids that belong to the org.
func (r *Repository) BulkUpdateControlStatus(ctx context.Context, orgID string, ids []string, status string) error {
	if err := r.q.BulkUpdateCKControlStatus(ctx, db.BulkUpdateCKControlStatusParams{
		ManualStatus: ckOptText(status),
		Ids:          ids,
		OrgID:        orgID,
	}); err != nil {
		return fmt.Errorf("bulk update control status: %w", err)
	}
	return nil
}

// FindControlByCode looks up a control UUID by its text control_id code within an org.
// Returns an empty string if not found.
func (r *Repository) FindControlByCode(ctx context.Context, orgID, code string) (string, error) {
	id, err := r.q.FindCKControlByCode(ctx, db.FindCKControlByCodeParams{
		OrgID:     orgID,
		ControlID: code,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("find control by code %q: %w", code, err)
	}
	return id, nil
}
