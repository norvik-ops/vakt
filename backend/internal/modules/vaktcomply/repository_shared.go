package vaktcomply

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/matharnica/vakt/internal/db"
)

// --- Collaborative Tasks ---

func taskFromCk(r db.CkTasks) Task {
	return Task{
		ID:            r.ID,
		OrgID:         r.OrgID,
		EntityType:    r.EntityType,
		EntityID:      r.EntityID,
		Title:         r.Title,
		Description:   r.Description,
		AssigneeEmail: r.AssigneeEmail,
		DueDate:       ckDateToTimePtr(r.DueDate),
		Status:        r.Status,
		Priority:      r.Priority,
		CreatedBy:     r.CreatedBy,
		CreatedAt:     ckTsToTime(r.CreatedAt),
		UpdatedAt:     ckTsToTime(r.UpdatedAt),
	}
}

// ListTasks returns all tasks for the given entity, ordered newest first.
func (r *Repository) ListTasks(ctx context.Context, orgID, entityType, entityID string) ([]Task, error) {
	rows, err := r.q.ListCKTasks(ctx, db.ListCKTasksParams{
		OrgID:      orgID,
		EntityType: entityType,
		EntityID:   entityID,
	})
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	out := make([]Task, 0, len(rows))
	for _, row := range rows {
		out = append(out, taskFromCk(row))
	}
	return out, nil
}

// CreateTask inserts a new task and returns the created row.
func (r *Repository) CreateTask(ctx context.Context, orgID, entityType, entityID string, in CreateTaskInput) (Task, error) {
	dueDate := pgtype.Date{}
	if in.DueDate != nil && *in.DueDate != "" {
		t, err := time.Parse("2006-01-02", *in.DueDate)
		if err != nil {
			return Task{}, fmt.Errorf("invalid due_date format (expected YYYY-MM-DD): %w", err)
		}
		dueDate = pgtype.Date{Time: t, Valid: true}
	}
	status := in.Status
	if status == "" {
		status = "open"
	}
	priority := in.Priority
	if priority == "" {
		priority = "medium"
	}
	row, err := r.q.CreateCKTask(ctx, db.CreateCKTaskParams{
		OrgID:         orgID,
		EntityType:    entityType,
		EntityID:      entityID,
		Title:         in.Title,
		Description:   in.Description,
		AssigneeEmail: in.AssigneeEmail,
		DueDate:       dueDate,
		Status:        status,
		Priority:      priority,
	})
	if err != nil {
		return Task{}, fmt.Errorf("create task: %w", err)
	}
	return taskFromCk(row), nil
}

// UpdateTask applies partial updates to a task via COALESCE.
func (r *Repository) UpdateTask(ctx context.Context, orgID, taskID string, in UpdateTaskInput) (Task, error) {
	dueDate := pgtype.Date{}
	if in.DueDate != nil && *in.DueDate != "" {
		t, err := time.Parse("2006-01-02", *in.DueDate)
		if err != nil {
			return Task{}, fmt.Errorf("invalid due_date format (expected YYYY-MM-DD): %w", err)
		}
		dueDate = pgtype.Date{Time: t, Valid: true}
	}
	row, err := r.q.UpdateCKTask(ctx, db.UpdateCKTaskParams{
		ID:            taskID,
		OrgID:         orgID,
		Title:         optTextStrPtr(in.Title),
		Description:   optTextStrPtr(in.Description),
		AssigneeEmail: optTextStrPtr(in.AssigneeEmail),
		DueDate:       dueDate,
		Status:        optTextStrPtr(in.Status),
		Priority:      optTextStrPtr(in.Priority),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Task{}, fmt.Errorf("task not found")
		}
		return Task{}, fmt.Errorf("update task: %w", err)
	}
	return taskFromCk(row), nil
}

// DeleteTask removes a task.
func (r *Repository) DeleteTask(ctx context.Context, orgID, taskID string) error {
	n, err := r.q.DeleteCKTask(ctx, db.DeleteCKTaskParams{ID: taskID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("task not found")
	}
	return nil
}

// ListOverdueTasks returns tasks with due_date in the past that are not done.
func (r *Repository) ListOverdueTasks(ctx context.Context, orgID string) ([]Task, error) {
	rows, err := r.q.ListCKOverdueTasks(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list overdue tasks: %w", err)
	}
	out := make([]Task, 0, len(rows))
	for _, row := range rows {
		out = append(out, taskFromCk(row))
	}
	return out, nil
}

// --- Comments ---

func commentFromCk(r db.CkComments) Comment {
	return Comment{
		ID:          r.ID,
		OrgID:       r.OrgID,
		EntityType:  r.EntityType,
		EntityID:    r.EntityID,
		AuthorEmail: r.AuthorEmail,
		Body:        r.Body,
		CreatedAt:   ckTsToTime(r.CreatedAt),
	}
}

// ListComments returns all comments for an entity ordered chronologically.
func (r *Repository) ListComments(ctx context.Context, orgID, entityType, entityID string) ([]Comment, error) {
	rows, err := r.q.ListCKComments(ctx, db.ListCKCommentsParams{
		OrgID:      orgID,
		EntityType: entityType,
		EntityID:   entityID,
	})
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	out := make([]Comment, 0, len(rows))
	for _, row := range rows {
		out = append(out, commentFromCk(row))
	}
	return out, nil
}

// CreateComment inserts a new comment and returns the created row.
func (r *Repository) CreateComment(ctx context.Context, orgID, entityType, entityID string, in CreateCommentInput) (Comment, error) {
	row, err := r.q.CreateCKComment(ctx, db.CreateCKCommentParams{
		OrgID:       orgID,
		EntityType:  entityType,
		EntityID:    entityID,
		AuthorEmail: in.AuthorEmail,
		Body:        in.Body,
	})
	if err != nil {
		return Comment{}, fmt.Errorf("create comment: %w", err)
	}
	return commentFromCk(row), nil
}

// DeleteComment removes a comment.
func (r *Repository) DeleteComment(ctx context.Context, orgID, commentID string) error {
	n, err := r.q.DeleteCKComment(ctx, db.DeleteCKCommentParams{ID: commentID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("comment not found")
	}
	return nil
}

// --- Resilience Tests (DORA Art. 24-27) ---

func resilienceTestFromCkResilienceTests(r db.CkResilienceTests) ResilienceTest {
	t := ResilienceTest{
		ID:                r.ID,
		OrgID:             r.OrgID,
		Type:              r.Type,
		Scope:             r.Scope.String,
		Provider:          r.Provider.String,
		Summary:           r.Summary.String,
		RemediationStatus: r.RemediationStatus,
		AttachmentURL:     r.AttachmentUrl.String,
		CreatedAt:         ckTsToTime(r.CreatedAt),
		UpdatedAt:         ckTsToTime(r.UpdatedAt),
	}
	if r.TestDate.Valid {
		t.TestDate = r.TestDate.Time
	}
	return t
}

// ListResilienceTests returns all resilience tests for an organisation, sorted by test_date DESC.
func (r *Repository) ListResilienceTests(ctx context.Context, orgID string) ([]ResilienceTest, error) {
	rows, err := r.q.ListCKResilienceTests(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list resilience tests: %w", err)
	}
	out := make([]ResilienceTest, 0, len(rows))
	for _, row := range rows {
		out = append(out, resilienceTestFromCkResilienceTests(row))
	}
	return out, nil
}

// GetResilienceTest returns a single resilience test by ID within an organisation.
// Returns an error containing "not found" if the test does not exist.
func (r *Repository) GetResilienceTest(ctx context.Context, orgID, id string) (*ResilienceTest, error) {
	row, err := r.q.GetCKResilienceTest(ctx, db.GetCKResilienceTestParams{ID: id, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("resilience test not found: %w", err)
	}
	t := resilienceTestFromCkResilienceTests(row)
	return &t, nil
}

// CreateResilienceTest inserts a new resilience test entry and returns it.
func (r *Repository) CreateResilienceTest(ctx context.Context, orgID string, in CreateResilienceTestInput) (*ResilienceTest, error) {
	remStatus := in.RemediationStatus
	if remStatus == "" {
		remStatus = "open"
	}
	row, err := r.q.CreateCKResilienceTest(ctx, db.CreateCKResilienceTestParams{
		OrgID:             orgID,
		Type:              in.Type,
		Scope:             in.Scope,
		Provider:          in.Provider,
		TestDate:          pgtype.Date{Time: in.TestDate, Valid: true},
		Summary:           in.Summary,
		RemediationStatus: remStatus,
	})
	if err != nil {
		return nil, fmt.Errorf("create resilience test: %w", err)
	}
	t := resilienceTestFromCkResilienceTests(row)
	return &t, nil
}

// UpdateResilienceTest updates an existing resilience test entry and returns it.
func (r *Repository) UpdateResilienceTest(ctx context.Context, orgID, id string, in UpdateResilienceTestInput) (*ResilienceTest, error) {
	row, err := r.q.UpdateCKResilienceTest(ctx, db.UpdateCKResilienceTestParams{
		ID:                id,
		OrgID:             orgID,
		Type:              in.Type,
		Scope:             in.Scope,
		Provider:          in.Provider,
		TestDate:          pgtype.Date{Time: in.TestDate, Valid: true},
		Summary:           in.Summary,
		RemediationStatus: in.RemediationStatus,
	})
	if err != nil {
		return nil, fmt.Errorf("update resilience test: %w", err)
	}
	t := resilienceTestFromCkResilienceTests(row)
	return &t, nil
}

// DeleteResilienceTest removes a resilience test entry.
func (r *Repository) DeleteResilienceTest(ctx context.Context, orgID, id string) error {
	n, err := r.q.DeleteCKResilienceTest(ctx, db.DeleteCKResilienceTestParams{ID: id, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete resilience test: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("resilience test not found")
	}
	return nil
}

// UpdateResilienceTestAttachment sets the attachment_url on a resilience test entry.
func (r *Repository) UpdateResilienceTestAttachment(ctx context.Context, orgID, id, url string) error {
	n, err := r.q.UpdateCKResilienceTestAttachment(ctx, db.UpdateCKResilienceTestAttachmentParams{
		ID:            id,
		OrgID:         orgID,
		AttachmentUrl: ckOptText(url),
	})
	if err != nil {
		return fmt.Errorf("update resilience test attachment: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("resilience test not found")
	}
	return nil
}

// --- CAPA (Corrective and Preventive Actions) ---

// capaFromCkCapas maps the sqlc Table-Row to the domain CAPA type.
func capaFromCkCapas(r db.CkCapas) CAPA {
	return CAPA{
		ID:               r.ID,
		OrgID:            r.OrgID,
		SourceType:       r.SourceType,
		SourceID:         r.SourceID,
		Title:            r.Title,
		Description:      r.Description,
		RootCause:        r.RootCause,
		ActionPlan:       r.ActionPlan,
		AssigneeEmail:    r.AssigneeEmail,
		DueDate:          ckDateToTimePtr(r.DueDate),
		Priority:         r.Priority,
		Status:           r.Status,
		VerificationNote: r.VerificationNote,
		ClosedAt:         ckTsToTimePtr(r.ClosedAt),
		CreatedAt:        ckTsToTime(r.CreatedAt),
		UpdatedAt:        ckTsToTime(r.UpdatedAt),
	}
}

// ListCAPAs returns CAPAs for an organisation, optionally filtered by status.
func (r *Repository) ListCAPAs(ctx context.Context, orgID string, statusFilter string) ([]CAPA, error) {
	rows, err := r.q.ListCKCAPAs(ctx, db.ListCKCAPAsParams{
		OrgID:  orgID,
		Status: ckOptText(statusFilter),
	})
	if err != nil {
		return nil, fmt.Errorf("list capas: %w", err)
	}
	out := make([]CAPA, 0, len(rows))
	for _, row := range rows {
		out = append(out, capaFromCkCapas(row))
	}
	return out, nil
}

// ListCAPAsForSource returns CAPAs linked to a specific source (audit/incident/risk).
func (r *Repository) ListCAPAsForSource(ctx context.Context, orgID, sourceType, sourceID string) ([]CAPA, error) {
	rows, err := r.q.ListCKCAPAsForSource(ctx, db.ListCKCAPAsForSourceParams{
		OrgID:      orgID,
		SourceType: sourceType,
		SourceID:   sourceID,
	})
	if err != nil {
		return nil, fmt.Errorf("list capas for source: %w", err)
	}
	out := make([]CAPA, 0, len(rows))
	for _, row := range rows {
		out = append(out, capaFromCkCapas(row))
	}
	return out, nil
}

// GetCAPA returns a single CAPA by ID within an organisation.
func (r *Repository) GetCAPA(ctx context.Context, orgID, capaID string) (CAPA, error) {
	row, err := r.q.GetCKCAPA(ctx, db.GetCKCAPAParams{ID: capaID, OrgID: orgID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CAPA{}, ErrNotFound
		}
		return CAPA{}, fmt.Errorf("get capa: %w", err)
	}
	return capaFromCkCapas(row), nil
}

// CreateCAPA inserts a new CAPA record.
func (r *Repository) CreateCAPA(ctx context.Context, orgID string, in CreateCAPAInput) (CAPA, error) {
	priority := in.Priority
	if priority == "" {
		priority = "medium"
	}
	row, err := r.q.CreateCKCAPA(ctx, db.CreateCKCAPAParams{
		OrgID:         orgID,
		SourceType:    in.SourceType,
		SourceID:      in.SourceID,
		Title:         in.Title,
		Description:   in.Description,
		AssigneeEmail: in.AssigneeEmail,
		DueDate:       ckOptDatePtr(in.DueDate),
		Priority:      priority,
	})
	if err != nil {
		return CAPA{}, fmt.Errorf("create capa: %w", err)
	}
	return capaFromCkCapas(row), nil
}

// UpdateCAPA applies partial updates to a CAPA using COALESCE.
// When status transitions to 'closed', closed_at is set to NOW().
func (r *Repository) UpdateCAPA(ctx context.Context, orgID, capaID string, in UpdateCAPAInput) (CAPA, error) {
	row, err := r.q.UpdateCKCAPA(ctx, db.UpdateCKCAPAParams{
		ID:               capaID,
		OrgID:            orgID,
		Title:            optTextStrPtr(in.Title),
		Description:      optTextStrPtr(in.Description),
		RootCause:        optTextStrPtr(in.RootCause),
		ActionPlan:       optTextStrPtr(in.ActionPlan),
		AssigneeEmail:    optTextStrPtr(in.AssigneeEmail),
		DueDate:          ckOptDatePtr(in.DueDate),
		Priority:         optTextStrPtr(in.Priority),
		Status:           optTextStrPtr(in.Status),
		VerificationNote: optTextStrPtr(in.VerificationNote),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CAPA{}, ErrNotFound
		}
		return CAPA{}, fmt.Errorf("update capa: %w", err)
	}
	return capaFromCkCapas(row), nil
}

// DeleteCAPA removes a CAPA record.
func (r *Repository) DeleteCAPA(ctx context.Context, orgID, capaID string) error {
	n, err := r.q.DeleteCKCAPA(ctx, db.DeleteCKCAPAParams{ID: capaID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete capa: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListCAPAsPaged returns a page of CAPAs plus the total count.
func (r *Repository) ListCAPAsPaged(ctx context.Context, orgID string, statusFilter string, offset, limit int) ([]CAPA, int, error) {
	statusArg := ckOptText(statusFilter)
	total, err := r.q.CountCKCAPAs(ctx, db.CountCKCAPAsParams{OrgID: orgID, Status: statusArg})
	if err != nil {
		return nil, 0, fmt.Errorf("count capas: %w", err)
	}
	rows, err := r.q.ListCKCAPAsPaged(ctx, db.ListCKCAPAsPagedParams{
		OrgID:  orgID,
		Status: statusArg,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list capas paged: %w", err)
	}
	capas := make([]CAPA, 0, len(rows))
	for _, row := range rows {
		capas = append(capas, capaFromCkCapas(row))
	}
	return capas, int(total), nil
}

// BulkUpdateCAPAStatus sets status for all CAPAs in ids that belong to the org.
// Behavior unchanged from original embedded query but jetzt setzt der Query
// auch closed_at = NOW() bei Übergang in 'closed' (Audit-Trail-Konsistenz mit
// UpdateCAPA).
func (r *Repository) BulkUpdateCAPAStatus(ctx context.Context, orgID string, ids []string, status string) error {
	_, err := r.q.BulkUpdateCKCAPAStatus(ctx, db.BulkUpdateCKCAPAStatusParams{
		OrgID:  orgID,
		Status: status,
		Ids:    ids,
	})
	if err != nil {
		return fmt.Errorf("bulk update capa status: %w", err)
	}
	return nil
}

// ListAllOrgs returns the IDs of all organisations.
// Used for cross-org seeding on startup.
func (r *Repository) ListAllOrgs(ctx context.Context) ([]string, error) {
	ids, err := r.q.ListAllOrgIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list all orgs: %w", err)
	}
	return ids, nil
}
