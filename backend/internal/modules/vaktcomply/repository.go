package vaktcomply

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/matharnica/vakt/internal/db"
	"github.com/matharnica/vakt/internal/modules/vaktcomply/policy"
	"github.com/rs/zerolog/log"
)

type Repository struct {
	*policy.Repository
	db *pgxpool.Pool
	q  *db.Queries
}

// NewRepository creates a new ComplyKit repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{Repository: policy.NewRepository(pool), db: pool, q: db.New(pool)}
}

// ckTsToTime converts pgtype.Timestamptz to time.Time (zero on NULL).
func ckTsToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

// ckTsToTimePtr converts pgtype.Timestamptz to *time.Time (nil on NULL).
func ckTsToTimePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	tm := t.Time
	return &tm
}

// ckDateToTimePtr converts pgtype.Date to *time.Time (nil on NULL).
func ckDateToTimePtr(d pgtype.Date) *time.Time {
	if !d.Valid {
		return nil
	}
	tm := d.Time
	return &tm
}

// ckOptText: empty string -> invalid pgtype.Text (NULL in DB).
func ckOptText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// ckOptIntPtr: nil -> invalid pgtype.Int4 (NULL in DB).
func ckOptIntPtr(i *int) pgtype.Int4 {
	if i == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*i), Valid: true}
}

// ckOptUUIDFromStr converts a string to pgtype.UUID; empty -> invalid.
func ckOptUUIDFromStr(s string) pgtype.UUID {
	if s == "" {
		return pgtype.UUID{}
	}
	var u pgtype.UUID
	_ = u.Scan(s)
	return u
}

// ckOptTsPtr converts *time.Time to pgtype.Timestamptz; nil -> invalid.
func ckOptTsPtr(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// ckOptDatePtr: nil string ptr -> invalid; "YYYY-MM-DD" string -> pgtype.Date.
func ckOptDatePtr(s *string) pgtype.Date {
	if s == nil || *s == "" {
		return pgtype.Date{}
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: t, Valid: true}
}

// incidentFields holds all columns shared between every Incident-returning
// sqlc query. ADR-0013: one mapper handles all Row-types.
type incidentFields struct {
	ID, OrgID, Title, Description, Severity, Status           string
	DiscoveredAt, ResolvedAt                                  pgtype.Timestamptz
	AffectedSystems                                           []string
	BreachID                                                  pgtype.UUID
	IncidentType, ReportingObligation                         string
	NotificationAuthority                                     pgtype.Text
	Deadline4h, Deadline24h, Deadline72h, Deadline30d         pgtype.Timestamptz
	Reported4hAt, Reported24hAt, Reported72hAt, Reported30dAt pgtype.Timestamptz
	AffectedCustomers                                         pgtype.Int4
	FinancialImpactEstimate                                   pgtype.Text
	IsMajorIncident                                           bool
	SupplierID                                                pgtype.UUID
	NotifiedWarn24h, NotifiedWarn72h, NotifiedWarn30d         bool
	CreatedAt, UpdatedAt                                      pgtype.Timestamptz
}

func uuidPtrFromPgtype(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	s := u.String()
	return &s
}

func incidentFromFields(f incidentFields) Incident {
	return Incident{
		ID:                      f.ID,
		OrgID:                   f.OrgID,
		Title:                   f.Title,
		Description:             f.Description,
		Severity:                f.Severity,
		Status:                  f.Status,
		DiscoveredAt:            ckTsToTime(f.DiscoveredAt),
		ResolvedAt:              ckTsToTimePtr(f.ResolvedAt),
		AffectedSystems:         f.AffectedSystems,
		BreachID:                uuidPtrFromPgtype(f.BreachID),
		IncidentType:            f.IncidentType,
		ReportingObligation:     f.ReportingObligation,
		NotificationAuthority:   f.NotificationAuthority.String,
		Deadline4h:              ckTsToTimePtr(f.Deadline4h),
		Deadline24h:             ckTsToTimePtr(f.Deadline24h),
		Deadline72h:             ckTsToTimePtr(f.Deadline72h),
		Deadline30d:             ckTsToTimePtr(f.Deadline30d),
		Reported4hAt:            ckTsToTimePtr(f.Reported4hAt),
		Reported24hAt:           ckTsToTimePtr(f.Reported24hAt),
		Reported72hAt:           ckTsToTimePtr(f.Reported72hAt),
		Reported30dAt:           ckTsToTimePtr(f.Reported30dAt),
		AffectedCustomers:       intPtrFromInt4(f.AffectedCustomers),
		FinancialImpactEstimate: textPtrOrNil(f.FinancialImpactEstimate),
		IsMajorIncident:         f.IsMajorIncident,
		SupplierID:              uuidPtrFromPgtype(f.SupplierID),
		NotifiedWarn24h:         f.NotifiedWarn24h,
		NotifiedWarn72h:         f.NotifiedWarn72h,
		NotifiedWarn30d:         f.NotifiedWarn30d,
		CreatedAt:               ckTsToTime(f.CreatedAt),
		UpdatedAt:               ckTsToTime(f.UpdatedAt),
	}
}

func textPtrOrNil(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

// riskFields collects all columns shared between every Risk-returning sqlc
// query. ADR-0013: centralise mapping in one helper.
type riskFields struct {
	ID, OrgID, Title, Description, Category  string
	Likelihood, Impact                       int16
	RiskScore                                pgtype.Int2
	Owner, Status, Treatment, TreatmentNotes string
	TreatmentOption                          pgtype.Text
	TreatmentPlan, TreatmentOwner            string
	TreatmentDueDate                         pgtype.Date
	TreatmentStatus                          string
	ResidualLikelihood                       pgtype.Int4
	ResidualImpact                           pgtype.Int4
	// S61-4 residual fields (Migration 164) — nil when populated via sqlc (query not regenerated)
	InherentLikelihood          pgtype.Int4
	InherentImpact              pgtype.Int4
	RiskAcceptedBy              pgtype.UUID
	RiskAcceptedAt              pgtype.Timestamptz
	RiskAcceptanceJustification pgtype.Text
	CreatedAt, UpdatedAt        pgtype.Timestamptz
}

func intPtrFromInt4(v pgtype.Int4) *int {
	if !v.Valid {
		return nil
	}
	i := int(v.Int32)
	return &i
}

func riskFromFields(f riskFields) Risk {
	r := Risk{
		ID:                 f.ID,
		OrgID:              f.OrgID,
		Title:              f.Title,
		Description:        f.Description,
		Category:           f.Category,
		Likelihood:         int(f.Likelihood),
		Impact:             int(f.Impact),
		RiskScore:          int(f.RiskScore.Int16),
		Owner:              f.Owner,
		Status:             f.Status,
		Treatment:          f.Treatment,
		TreatmentNotes:     f.TreatmentNotes,
		TreatmentOption:    f.TreatmentOption.String,
		TreatmentPlan:      f.TreatmentPlan,
		TreatmentOwner:     f.TreatmentOwner,
		TreatmentDueDate:   ckDateToTimePtr(f.TreatmentDueDate),
		TreatmentStatus:    f.TreatmentStatus,
		ResidualLikelihood: intPtrFromInt4(f.ResidualLikelihood),
		ResidualImpact:     intPtrFromInt4(f.ResidualImpact),
		// S61-4 residual fields
		InherentLikelihood: intPtrFromInt4(f.InherentLikelihood),
		InherentImpact:     intPtrFromInt4(f.InherentImpact),
		CreatedAt:          ckTsToTime(f.CreatedAt),
		UpdatedAt:          ckTsToTime(f.UpdatedAt),
	}
	if f.RiskAcceptedBy.Valid {
		s := f.RiskAcceptedBy.Bytes
		str := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
			s[0:4], s[4:6], s[6:8], s[8:10], s[10:16])
		r.RiskAcceptedBy = &str
	}
	r.RiskAcceptedAt = ckTsToTimePtr(f.RiskAcceptedAt)
	if f.RiskAcceptanceJustification.Valid {
		r.RiskAcceptanceJustification = f.RiskAcceptanceJustification.String
	}
	r.ComputeScores()
	return r
}

// evidenceFields is the union of columns returned by all Evidence-returning
// sqlc queries (Add/List/GetExpiring). Identical shape, so one container.
type evidenceFields struct {
	ID               string
	ControlID        pgtype.UUID
	OrgID            string
	Title            string
	Description      pgtype.Text
	Source           string
	FilePath         pgtype.Text
	FileSize         pgtype.Int8
	Status           string
	Version          int32
	ExpiresAt        pgtype.Timestamptz
	ExpiryNotifiedAt pgtype.Timestamptz
	CreatedAt        pgtype.Timestamptz
	UpdatedAt        pgtype.Timestamptz
}

func evidenceFromFields(f evidenceFields) Evidence {
	var controlID string
	if f.ControlID.Valid {
		controlID = f.ControlID.String()
	}
	return Evidence{
		ID:               f.ID,
		ControlID:        controlID,
		OrgID:            f.OrgID,
		Title:            f.Title,
		Description:      f.Description.String,
		Source:           f.Source,
		FilePath:         f.FilePath.String,
		FileSize:         f.FileSize.Int64,
		Status:           f.Status,
		Version:          int(f.Version),
		ExpiresAt:        ckTsToTimePtr(f.ExpiresAt),
		ExpiryNotifiedAt: ckTsToTimePtr(f.ExpiryNotifiedAt),
		CreatedAt:        ckTsToTime(f.CreatedAt),
		UpdatedAt:        ckTsToTime(f.UpdatedAt),
	}
}

// optTextStrPtr converts *string to pgtype.Text (nil -> invalid, *"" -> valid empty).
func optTextStrPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// ckOptUUIDFromPtr converts *string to pgtype.UUID; nil/empty -> invalid.
func ckOptUUIDFromPtr(s *string) pgtype.UUID {
	if s == nil || *s == "" {
		return pgtype.UUID{}
	}
	return ckOptUUIDFromStr(*s)
}

// uuidStringFromPgtype returns the UUID as string ("" when invalid).
func uuidStringFromPgtype(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return u.String()
}

// policyDateFromTimePtr converts *time.Time -> pgtype.Date.
func policyDateFromTimePtr(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *t, Valid: true}
}

func uuidStringPtr(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	s := uuidStringFromPgtype(u)
	return &s
}

// milestoneFromRow maps a sqlc milestone row (shared column layout) to the
// AuditMilestone domain type. today is pre-computed by the caller.
func milestoneFromRow(
	id, orgID string,
	frameworkID pgtype.UUID,
	title string,
	description pgtype.Text,
	milestoneDate, milestoneType, status string,
	createdBy pgtype.UUID,
	createdAt, updatedAt pgtype.Timestamptz,
	today time.Time,
) AuditMilestone {
	return AuditMilestone{
		ID:            id,
		OrgID:         orgID,
		FrameworkID:   uuidStringPtr(frameworkID),
		Title:         title,
		Description:   description.String,
		MilestoneDate: milestoneDate,
		MilestoneType: milestoneType,
		Status:        status,
		CreatedBy:     uuidStringPtr(createdBy),
		CreatedAt:     ckTsToTime(createdAt),
		UpdatedAt:     ckTsToTime(updatedAt),
		DaysRemaining: computeDaysRemaining(milestoneDate, today),
	}
}

// milestoneFromListRow maps a ListCKMilestonesRow to AuditMilestone.
func milestoneFromListRow(r db.ListCKMilestonesRow, today time.Time) AuditMilestone {
	return milestoneFromRow(
		r.ID, r.OrgID, r.FrameworkID,
		r.Title, r.Description,
		r.MilestoneDate, r.MilestoneType, r.Status,
		r.CreatedBy, r.CreatedAt, r.UpdatedAt,
		today,
	)
}

// milestoneFromGetRow maps a GetCKMilestoneRow to AuditMilestone.
func milestoneFromGetRow(r db.GetCKMilestoneRow, today time.Time) AuditMilestone {
	return milestoneFromRow(
		r.ID, r.OrgID, r.FrameworkID,
		r.Title, r.Description,
		r.MilestoneDate, r.MilestoneType, r.Status,
		r.CreatedBy, r.CreatedAt, r.UpdatedAt,
		today,
	)
}

// milestoneFromCreateRow maps a CreateCKMilestoneRow to AuditMilestone.
func milestoneFromCreateRow(r db.CreateCKMilestoneRow, today time.Time) AuditMilestone {
	return milestoneFromRow(
		r.ID, r.OrgID, r.FrameworkID,
		r.Title, r.Description,
		r.MilestoneDate, r.MilestoneType, r.Status,
		r.CreatedBy, r.CreatedAt, r.UpdatedAt,
		today,
	)
}

// milestoneFromUpdateRow maps an UpdateCKMilestoneRow to AuditMilestone.
func milestoneFromUpdateRow(r db.UpdateCKMilestoneRow, today time.Time) AuditMilestone {
	return milestoneFromRow(
		r.ID, r.OrgID, r.FrameworkID,
		r.Title, r.Description,
		r.MilestoneDate, r.MilestoneType, r.Status,
		r.CreatedBy, r.CreatedAt, r.UpdatedAt,
		today,
	)
}

// milestoneFromNextRow maps a NextCKMilestoneRow to AuditMilestone.
func milestoneFromNextRow(r db.NextCKMilestoneRow, today time.Time) AuditMilestone {
	return milestoneFromRow(
		r.ID, r.OrgID, r.FrameworkID,
		r.Title, r.Description,
		r.MilestoneDate, r.MilestoneType, r.Status,
		r.CreatedBy, r.CreatedAt, r.UpdatedAt,
		today,
	)
}

// parseDateArg converts a YYYY-MM-DD string to pgtype.Date (invalid on empty).
func parseDateArg(s string) pgtype.Date {
	if s == "" {
		return pgtype.Date{}
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: t, Valid: true}
}

// ── Repository methods ────────────────────────────────────────────────────────

// ListMilestones returns all milestones for an org ordered by milestone_date ASC.
// If statusFilter is non-empty only that status is returned.
func (r *Repository) ListMilestones(ctx context.Context, orgID, statusFilter string) ([]AuditMilestone, error) {
	rows, err := r.q.ListCKMilestones(ctx, db.ListCKMilestonesParams{
		OrgID:  orgID,
		Status: ckOptText(statusFilter),
	})
	if err != nil {
		return nil, fmt.Errorf("list milestones: %w", err)
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	milestones := make([]AuditMilestone, 0, len(rows))
	for _, row := range rows {
		milestones = append(milestones, milestoneFromListRow(row, today))
	}
	return milestones, nil
}

// GetMilestone retrieves a single milestone by ID.
func (r *Repository) GetMilestone(ctx context.Context, orgID, milestoneID string) (*AuditMilestone, error) {
	row, err := r.q.GetCKMilestone(ctx, db.GetCKMilestoneParams{ID: milestoneID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get milestone: %w", err)
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	m := milestoneFromGetRow(row, today)
	return &m, nil
}

// CreateMilestone inserts a new milestone.
func (r *Repository) CreateMilestone(ctx context.Context, orgID, createdBy string, in CreateMilestoneInput) (*AuditMilestone, error) {
	row, err := r.q.CreateCKMilestone(ctx, db.CreateCKMilestoneParams{
		OrgID:         orgID,
		FrameworkID:   ckOptUUIDFromStr(in.FrameworkID),
		Title:         in.Title,
		Description:   ckOptText(in.Description),
		MilestoneDate: parseDateArg(in.MilestoneDate),
		MilestoneType: in.MilestoneType,
		CreatedBy:     ckOptUUIDFromStr(createdBy),
	})
	if err != nil {
		return nil, fmt.Errorf("create milestone: %w", err)
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	m := milestoneFromCreateRow(row, today)
	return &m, nil
}

// UpdateMilestone applies a partial update to an existing milestone.
func (r *Repository) UpdateMilestone(ctx context.Context, orgID, milestoneID string, in UpdateMilestoneInput) (*AuditMilestone, error) {
	// Fetch current to merge
	cur, err := r.GetMilestone(ctx, orgID, milestoneID)
	if err != nil {
		return nil, err
	}

	title := cur.Title
	description := cur.Description
	milestoneDate := cur.MilestoneDate
	milestoneType := cur.MilestoneType
	status := cur.Status

	if in.Title != nil {
		title = *in.Title
	}
	if in.Description != nil {
		description = *in.Description
	}
	if in.MilestoneDate != nil {
		milestoneDate = *in.MilestoneDate
	}
	if in.MilestoneType != nil {
		milestoneType = *in.MilestoneType
	}
	if in.Status != nil {
		status = *in.Status
	}

	row, err := r.q.UpdateCKMilestone(ctx, db.UpdateCKMilestoneParams{
		Title:         title,
		Description:   ckOptText(description),
		MilestoneDate: parseDateArg(milestoneDate),
		MilestoneType: milestoneType,
		Status:        status,
		ID:            milestoneID,
		OrgID:         orgID,
	})
	if err != nil {
		return nil, fmt.Errorf("update milestone: %w", err)
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	m := milestoneFromUpdateRow(row, today)
	return &m, nil
}

// DeleteMilestone removes a milestone.
func (r *Repository) DeleteMilestone(ctx context.Context, orgID, milestoneID string) error {
	n, err := r.q.DeleteCKMilestone(ctx, db.DeleteCKMilestoneParams{ID: milestoneID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete milestone: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("milestone not found")
	}
	return nil
}

// NextMilestone returns the nearest upcoming milestone or nil if none exist.
func (r *Repository) NextMilestone(ctx context.Context, orgID string) (*AuditMilestone, error) {
	row, err := r.q.NextCKMilestone(ctx, orgID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, err // caller checks pgx.ErrNoRows
		}
		return nil, err
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	m := milestoneFromNextRow(row, today)
	return &m, nil
}

// computeDaysRemaining returns a pointer to the number of days between today and the milestone date.
// Negative values mean the milestone is overdue.
func computeDaysRemaining(dateStr string, today time.Time) *int {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return nil
	}
	days := int(math.Round(t.Sub(today).Hours() / 24))
	return &days
}

func campaignFromCk(r db.CkAccessReviewCampaigns) AccessReviewCampaign {
	return AccessReviewCampaign{
		ID:            r.ID,
		OrgID:         r.OrgID,
		Title:         r.Title,
		Description:   r.Description.String,
		Status:        r.Status,
		ReviewerEmail: r.ReviewerEmail,
		Scope:         r.Scope.String,
		DueDate:       ckTsToTimePtr(r.DueDate),
		CompletedAt:   ckTsToTimePtr(r.CompletedAt),
		CreatedBy:     r.CreatedBy.String,
		CreatedAt:     ckTsToTime(r.CreatedAt),
		UpdatedAt:     ckTsToTime(r.UpdatedAt),
	}
}

// parseAccessReviewDueDate accepts RFC3339 or YYYY-MM-DD.
func parseAccessReviewDueDate(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return &t, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return &t, nil
	}
	return nil, fmt.Errorf("invalid due_date format: %s", s)
}

// ListAccessReviewCampaigns returns all campaigns for an organisation ordered by created_at DESC.
func (r *Repository) ListAccessReviewCampaigns(ctx context.Context, orgID string) ([]AccessReviewCampaign, error) {
	rows, err := r.q.ListCKAccessReviewCampaigns(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list access review campaigns: %w", err)
	}
	out := make([]AccessReviewCampaign, 0, len(rows))
	for _, row := range rows {
		out = append(out, campaignFromCk(db.CkAccessReviewCampaigns(row)))
	}
	return out, nil
}

// GetAccessReviewCampaign returns a single campaign by ID. Returns ErrNotFound if absent.
func (r *Repository) GetAccessReviewCampaign(ctx context.Context, orgID, id string) (*AccessReviewCampaign, error) {
	row, err := r.q.GetCKAccessReviewCampaign(ctx, db.GetCKAccessReviewCampaignParams{ID: id, OrgID: orgID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get access review campaign: %w", err)
	}
	c := campaignFromCk(db.CkAccessReviewCampaigns(row))
	return &c, nil
}

// CreateAccessReviewCampaign inserts a new campaign and returns the created record.
func (r *Repository) CreateAccessReviewCampaign(ctx context.Context, orgID string, in CreateAccessReviewCampaignInput) (*AccessReviewCampaign, error) {
	var dueDate *time.Time
	if in.DueDate != nil {
		t, err := parseAccessReviewDueDate(*in.DueDate)
		if err != nil {
			return nil, err
		}
		dueDate = t
	}
	row, err := r.q.CreateCKAccessReviewCampaign(ctx, db.CreateCKAccessReviewCampaignParams{
		OrgID:         orgID,
		Title:         in.Title,
		Description:   ckOptText(in.Description),
		ReviewerEmail: in.ReviewerEmail,
		Scope:         ckOptText(in.Scope),
		DueDate:       ckOptTsPtr(dueDate),
	})
	if err != nil {
		return nil, fmt.Errorf("create access review campaign: %w", err)
	}
	c := campaignFromCk(db.CkAccessReviewCampaigns(row))
	return &c, nil
}

// UpdateAccessReviewCampaign applies updates to an existing campaign and returns the updated record.
func (r *Repository) UpdateAccessReviewCampaign(ctx context.Context, orgID, id string, in UpdateAccessReviewCampaignInput) (*AccessReviewCampaign, error) {
	cur, err := r.GetAccessReviewCampaign(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	title := cur.Title
	description := cur.Description
	reviewerEmail := cur.ReviewerEmail
	scope := cur.Scope
	status := cur.Status
	dueDate := cur.DueDate

	if in.Title != "" {
		title = in.Title
	}
	if in.Description != "" {
		description = in.Description
	}
	if in.ReviewerEmail != "" {
		reviewerEmail = in.ReviewerEmail
	}
	if in.Scope != "" {
		scope = in.Scope
	}
	if in.Status != "" {
		status = in.Status
	}
	if in.DueDate != nil {
		if *in.DueDate == "" {
			dueDate = nil
		} else {
			t, err := parseAccessReviewDueDate(*in.DueDate)
			if err != nil {
				return nil, err
			}
			dueDate = t
		}
	}

	// Set completed_at when transitioning to completed
	var completedAt *time.Time
	if status == "completed" && cur.Status != "completed" {
		now := time.Now().UTC()
		completedAt = &now
	} else {
		completedAt = cur.CompletedAt
	}

	row, err := r.q.UpdateCKAccessReviewCampaign(ctx, db.UpdateCKAccessReviewCampaignParams{
		Title:         title,
		Description:   ckOptText(description),
		ReviewerEmail: reviewerEmail,
		Scope:         ckOptText(scope),
		Status:        status,
		DueDate:       ckOptTsPtr(dueDate),
		CompletedAt:   ckOptTsPtr(completedAt),
		ID:            id,
		OrgID:         orgID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update access review campaign: %w", err)
	}
	c := campaignFromCk(db.CkAccessReviewCampaigns(row))
	return &c, nil
}

// DeleteAccessReviewCampaign removes a campaign (cascades to items).
func (r *Repository) DeleteAccessReviewCampaign(ctx context.Context, orgID, id string) error {
	n, err := r.q.DeleteCKAccessReviewCampaign(ctx, db.DeleteCKAccessReviewCampaignParams{ID: id, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete access review campaign: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Access Review Items ---

func itemFromCk(r db.CkAccessReviewItems) AccessReviewItem {
	return AccessReviewItem{
		ID:              r.ID,
		CampaignID:      r.CampaignID,
		OrgID:           r.OrgID,
		UserEmail:       r.UserEmail,
		AccessLevel:     r.AccessLevel,
		Decision:        r.Decision,
		ReviewerComment: r.ReviewerComment.String,
		DecidedAt:       ckTsToTimePtr(r.DecidedAt),
		CreatedAt:       ckTsToTime(r.CreatedAt),
	}
}

// ListAccessReviewItems returns all items for a campaign.
func (r *Repository) ListAccessReviewItems(ctx context.Context, orgID, campaignID string) ([]AccessReviewItem, error) {
	rows, err := r.q.ListCKAccessReviewItems(ctx, db.ListCKAccessReviewItemsParams{
		CampaignID: campaignID,
		OrgID:      orgID,
	})
	if err != nil {
		return nil, fmt.Errorf("list access review items: %w", err)
	}
	out := make([]AccessReviewItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, itemFromCk(db.CkAccessReviewItems(row)))
	}
	return out, nil
}

// CreateAccessReviewItem inserts a new review item and returns the created record.
func (r *Repository) CreateAccessReviewItem(ctx context.Context, orgID string, in CreateAccessReviewItemInput) (*AccessReviewItem, error) {
	row, err := r.q.CreateCKAccessReviewItem(ctx, db.CreateCKAccessReviewItemParams{
		CampaignID:  in.CampaignID,
		OrgID:       orgID,
		UserEmail:   in.UserEmail,
		AccessLevel: in.AccessLevel,
	})
	if err != nil {
		return nil, fmt.Errorf("create access review item: %w", err)
	}
	it := itemFromCk(db.CkAccessReviewItems(row))
	return &it, nil
}

// UpdateAccessReviewItem applies a decision to a review item.
func (r *Repository) UpdateAccessReviewItem(ctx context.Context, orgID, id string, in UpdateAccessReviewItemInput) (*AccessReviewItem, error) {
	var decidedAt pgtype.Timestamptz
	if in.Decision == "approved" || in.Decision == "revoked" {
		decidedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	}
	row, err := r.q.UpdateCKAccessReviewItem(ctx, db.UpdateCKAccessReviewItemParams{
		Decision:        in.Decision,
		ReviewerComment: in.ReviewerComment,
		DecidedAt:       decidedAt,
		ID:              id,
		OrgID:           orgID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update access review item: %w", err)
	}
	it := itemFromCk(db.CkAccessReviewItems(row))
	return &it, nil
}

func (r *Repository) ListInterestedParties(ctx context.Context, orgID string) ([]InterestedParty, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, org_id, name, category,
		       COALESCE(requirements,''), COALESCE(concerns,''),
		       to_char(review_date,'YYYY-MM-DD'), is_system_default,
		       created_at, updated_at
		FROM ck_interested_parties WHERE org_id = $1 ORDER BY name`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	today := time.Now().UTC().Format("2006-01-02")
	var parties []InterestedParty
	for rows.Next() {
		var p InterestedParty
		var reviewDate pgtype.Text
		var createdAt, updatedAt time.Time
		if err := rows.Scan(
			&p.ID, &p.OrgID, &p.Name, &p.Category,
			&p.Requirements, &p.Concerns, &reviewDate,
			&p.IsSystemDefault, &createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}
		p.CreatedAt = createdAt.Format(time.RFC3339)
		p.UpdatedAt = updatedAt.Format(time.RFC3339)
		if reviewDate.Valid {
			rd := reviewDate.String
			p.ReviewDate = &rd
			p.ReviewOverdue = rd < today
		}
		parties = append(parties, p)
	}
	return parties, rows.Err()
}

// CreateInterestedParty inserts a new entry.
func (r *Repository) CreateInterestedParty(ctx context.Context, orgID string, in CreateInterestedPartyInput, isDefault bool) (*InterestedParty, error) {
	var reviewDate pgtype.Date
	if in.ReviewDate != nil && *in.ReviewDate != "" {
		if err := reviewDate.Scan(*in.ReviewDate); err != nil {
			return nil, err
		}
	}
	var p InterestedParty
	var reviewDateOut pgtype.Text
	var createdAt, updatedAt time.Time
	err := r.db.QueryRow(ctx, `
		INSERT INTO ck_interested_parties (org_id, name, category, requirements, concerns, review_date, is_system_default)
		VALUES ($1, $2, $3, NULLIF($4,''), NULLIF($5,''), NULLIF($6::date, NULL), $7)
		RETURNING id, org_id, name, category,
		          COALESCE(requirements,''), COALESCE(concerns,''),
		          to_char(review_date,'YYYY-MM-DD'), is_system_default, created_at, updated_at`,
		orgID, in.Name, in.Category, in.Requirements, in.Concerns, reviewDate, isDefault,
	).Scan(&p.ID, &p.OrgID, &p.Name, &p.Category, &p.Requirements, &p.Concerns,
		&reviewDateOut, &p.IsSystemDefault, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	p.CreatedAt = createdAt.Format(time.RFC3339)
	p.UpdatedAt = updatedAt.Format(time.RFC3339)
	if reviewDateOut.Valid {
		rd := reviewDateOut.String
		p.ReviewDate = &rd
	}
	return &p, nil
}

// UpdateInterestedParty modifies an existing entry.
func (r *Repository) UpdateInterestedParty(ctx context.Context, orgID, id string, in CreateInterestedPartyInput) (*InterestedParty, error) {
	var reviewDate pgtype.Date
	if in.ReviewDate != nil && *in.ReviewDate != "" {
		if err := reviewDate.Scan(*in.ReviewDate); err != nil {
			return nil, err
		}
	}
	var p InterestedParty
	var reviewDateOut pgtype.Text
	var createdAt, updatedAt time.Time
	err := r.db.QueryRow(ctx, `
		UPDATE ck_interested_parties SET
			name         = $1,
			category     = $2,
			requirements = NULLIF($3,''),
			concerns     = NULLIF($4,''),
			review_date  = NULLIF($5::date, NULL),
			updated_at   = NOW()
		WHERE org_id = $6 AND id = $7
		RETURNING id, org_id, name, category,
		          COALESCE(requirements,''), COALESCE(concerns,''),
		          to_char(review_date,'YYYY-MM-DD'), is_system_default, created_at, updated_at`,
		in.Name, in.Category, in.Requirements, in.Concerns, reviewDate, orgID, id,
	).Scan(&p.ID, &p.OrgID, &p.Name, &p.Category, &p.Requirements, &p.Concerns,
		&reviewDateOut, &p.IsSystemDefault, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	p.CreatedAt = createdAt.Format(time.RFC3339)
	p.UpdatedAt = updatedAt.Format(time.RFC3339)
	if reviewDateOut.Valid {
		rd := reviewDateOut.String
		p.ReviewDate = &rd
	}
	return &p, nil
}

// DeleteInterestedParty removes an entry by ID.
func (r *Repository) DeleteInterestedParty(ctx context.Context, orgID, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM ck_interested_parties WHERE org_id = $1 AND id = $2`, orgID, id)
	return err
}

// CountInterestedParties returns the total count for the org.
func (r *Repository) CountInterestedParties(ctx context.Context, orgID string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM ck_interested_parties WHERE org_id = $1`, orgID).Scan(&count)
	return count, err
}

// CheckClause42Fulfilled returns true if ≥3 entries have requirements set.
func (r *Repository) CheckClause42Fulfilled(ctx context.Context, orgID string) (bool, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM ck_interested_parties
		WHERE org_id = $1 AND requirements IS NOT NULL AND requirements != ''`, orgID,
	).Scan(&count)
	return count >= 3, err
}

func scanISMSScope(row pgx.Row) (ISMSScope, error) {
	var s ISMSScope
	var createdAt, updatedAt pgtype.Timestamptz
	var approvedAt pgtype.Timestamptz
	var approvedBy pgtype.Text
	var exclusionsRaw []byte

	err := row.Scan(
		&s.ID,
		&s.OrgID,
		&s.Version,
		&s.Status,
		&s.ScopeDefinition,
		&exclusionsRaw,
		&s.OutsourcingDependencies,
		&s.ChangeNote,
		&approvedBy,
		&approvedAt,
		&s.CreatedBy,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return ISMSScope{}, err
	}

	if exclusionsRaw != nil {
		s.Exclusions = json.RawMessage(exclusionsRaw)
	} else {
		s.Exclusions = json.RawMessage("[]")
	}
	if approvedBy.Valid {
		v := approvedBy.String
		s.ApprovedBy = &v
	}
	s.ApprovedAt = ckTsToTimePtr(approvedAt)
	s.CreatedAt = ckTsToTime(createdAt)
	s.UpdatedAt = ckTsToTime(updatedAt)
	return s, nil
}

const ismsScopeSelectCols = `id, org_id, version, status, scope_definition, exclusions,
	outsourcing_dependencies, change_note, approved_by, approved_at, created_by, created_at, updated_at`

// CreateOrVersionISMSScope inserts a new scope document. If one already exists for
// the org, the new record gets version = max(existing) + 1.
func (r *Repository) CreateOrVersionISMSScope(ctx context.Context, orgID, userID string, in CreateISMSScopeInput) (ISMSScope, error) {
	exclusions := in.Exclusions
	if exclusions == nil {
		exclusions = json.RawMessage("[]")
	}

	row := r.db.QueryRow(ctx, `
		INSERT INTO ck_isms_scope (
			org_id, version, status, scope_definition, exclusions,
			outsourcing_dependencies, change_note, created_by
		)
		VALUES (
			$1::uuid,
			COALESCE((SELECT MAX(version) FROM ck_isms_scope WHERE org_id = $1::uuid), 0) + 1,
			'draft',
			$2, $3::jsonb, $4, $5,
			$6::uuid
		)
		RETURNING `+ismsScopeSelectCols,
		orgID,
		in.ScopeDefinition,
		exclusions,
		in.OutsourcingDependencies,
		in.ChangeNote,
		userID,
	)
	scope, err := scanISMSScope(row)
	if err != nil {
		return ISMSScope{}, fmt.Errorf("create isms scope: %w", err)
	}
	return scope, nil
}

// GetCurrentISMSScope returns the latest version for the given org.
func (r *Repository) GetCurrentISMSScope(ctx context.Context, orgID string) (ISMSScope, error) {
	row := r.db.QueryRow(ctx, `
		SELECT `+ismsScopeSelectCols+`
		FROM ck_isms_scope
		WHERE org_id = $1::uuid
		ORDER BY version DESC
		LIMIT 1`,
		orgID,
	)
	scope, err := scanISMSScope(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ISMSScope{}, fmt.Errorf("isms scope not found")
		}
		return ISMSScope{}, fmt.Errorf("get current isms scope: %w", err)
	}
	return scope, nil
}

// ListISMSScopeVersions returns all versions for the given org, newest first.
func (r *Repository) ListISMSScopeVersions(ctx context.Context, orgID string) ([]ISMSScope, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+ismsScopeSelectCols+`
		FROM ck_isms_scope
		WHERE org_id = $1::uuid
		ORDER BY version DESC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list isms scope versions: %w", err)
	}
	defer rows.Close()

	var out []ISMSScope
	for rows.Next() {
		scope, err := scanISMSScope(rows)
		if err != nil {
			return nil, fmt.Errorf("scan isms scope: %w", err)
		}
		out = append(out, scope)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("list isms scope versions rows: %w", rows.Err())
	}
	return out, nil
}

// ApproveISMSScope sets status='approved' and records the approver.
func (r *Repository) ApproveISMSScope(ctx context.Context, orgID, id, approverID string) (ISMSScope, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE ck_isms_scope
		SET status = 'approved', approved_by = $1::uuid, approved_at = NOW(), updated_at = NOW()
		WHERE id = $2::uuid AND org_id = $3::uuid
		RETURNING `+ismsScopeSelectCols,
		approverID, id, orgID,
	)
	scope, err := scanISMSScope(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ISMSScope{}, fmt.Errorf("isms scope not found")
		}
		return ISMSScope{}, fmt.Errorf("approve isms scope: %w", err)
	}
	return scope, nil
}

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

func (r *Repository) InsertScoreSnapshot(ctx context.Context, orgID string, frameworkID *string, score float64, total, implemented int) error {
	var fwID pgtype.UUID
	if frameworkID != nil && *frameworkID != "" {
		fwID = ckOptUUIDFromStr(*frameworkID)
	}
	return r.q.InsertCKScoreSnapshot(ctx, db.InsertCKScoreSnapshotParams{
		OrgID:               orgID,
		FrameworkID:         fwID,
		Score:               float64PtrToNumericCK(&score),
		ControlsTotal:       int32(total),
		ControlsImplemented: int32(implemented),
	})
}

// float64PtrToNumericCK is the vaktcomply-local copy of the vaktscan helper.
func float64PtrToNumericCK(f *float64) pgtype.Numeric {
	if f == nil {
		return pgtype.Numeric{}
	}
	var n pgtype.Numeric
	if err := n.Scan(strconv.FormatFloat(*f, 'f', -1, 64)); err != nil {
		return pgtype.Numeric{}
	}
	return n
}

// ScoreHistoryEntry is a single data point for the score trend chart.
type ScoreHistoryEntry struct {
	Date                string  `json:"date"`
	Score               float64 `json:"score"`
	ControlsTotal       int     `json:"controls_total"`
	ControlsImplemented int     `json:"controls_implemented"`
}

// GetScoreHistory returns aggregated daily score history for an organisation.
// framework_id is nil to query the org-wide score. Days is the look-back window.
func (r *Repository) GetScoreHistory(ctx context.Context, orgID string, days int) ([]ScoreHistoryEntry, error) {
	rows, err := r.q.GetCKScoreHistory(ctx, db.GetCKScoreHistoryParams{
		OrgID: orgID,
		Days:  int32(days),
	})
	if err != nil {
		return nil, fmt.Errorf("get score history: %w", err)
	}
	out := make([]ScoreHistoryEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, ScoreHistoryEntry{
			Date:                row.Date,
			Score:               row.Score,
			ControlsTotal:       int(row.ControlsTotal),
			ControlsImplemented: int(row.ControlsImplemented),
		})
	}
	return out, nil
}

// --- Board Report + Executive Summary (s26-sqlc-vitals-4) ---

// BoardReportComplianceScoreRow is a single framework's control counts for the board report score.
type BoardReportComplianceScoreRow struct {
	Total       int
	Implemented int
}

// GetBoardReportComplianceScoreRows returns per-framework control counts for computing the weighted compliance score.
func (r *Repository) GetBoardReportComplianceScoreRows(ctx context.Context, orgID string) ([]BoardReportComplianceScoreRow, error) {
	rows, err := r.q.GetBoardReportComplianceScoreRows(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("board report compliance score: %w", err)
	}
	out := make([]BoardReportComplianceScoreRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, BoardReportComplianceScoreRow{
			Total:       int(row.Total),
			Implemented: int(row.Implemented),
		})
	}
	return out, nil
}

// GetPreviousScore returns the most recent compliance score snapshot before today (for board report delta).
// Returns 0 and no error when no prior snapshot exists.
func (r *Repository) GetPreviousScore(ctx context.Context, orgID string) (int, error) {
	score, err := r.q.GetCKPreviousScore(ctx, orgID)
	if err != nil {
		return 0, err
	}
	return int(score), nil
}

// ListActiveOrgIDs returns IDs of all non-deleted organisations.
func (r *Repository) ListActiveOrgIDs(ctx context.Context) ([]string, error) {
	ids, err := r.q.ListActiveOrgIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active org ids: %w", err)
	}
	return ids, nil
}

// ExecutiveFrameworkScoreRow holds name + control counts for the executive summary.
type ExecutiveFrameworkScoreRow struct {
	Name        string
	Total       int
	Implemented int
}

// GetExecutiveFrameworkScores returns per-framework name + control counts for the executive summary.
func (r *Repository) GetExecutiveFrameworkScores(ctx context.Context, orgID string) ([]ExecutiveFrameworkScoreRow, error) {
	rows, err := r.q.GetExecutiveFrameworkScores(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("executive framework scores: %w", err)
	}
	out := make([]ExecutiveFrameworkScoreRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, ExecutiveFrameworkScoreRow{
			Name:        row.Name,
			Total:       int(row.Total),
			Implemented: int(row.Implemented),
		})
	}
	return out, nil
}

// ExecutiveTopRiskRow holds title, score and severity for the top-5 risks.
type ExecutiveTopRiskRow struct {
	Title    string
	Score    int
	Severity string
}

// GetExecutiveTopRisks returns the top-5 open risks by score for the executive summary.
func (r *Repository) GetExecutiveTopRisks(ctx context.Context, orgID string) ([]ExecutiveTopRiskRow, error) {
	rows, err := r.q.GetExecutiveTopRisks(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("executive top risks: %w", err)
	}
	out := make([]ExecutiveTopRiskRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, ExecutiveTopRiskRow{
			Title:    row.Title,
			Score:    int(row.Score),
			Severity: row.Severity,
		})
	}
	return out, nil
}

// CountClosedControlsSince returns the number of controls set to 'implemented' since `since`.
func (r *Repository) CountClosedControlsSince(ctx context.Context, orgID string, since time.Time) (int, error) {
	n, err := r.q.CountCKClosedControlsSince(ctx, db.CountCKClosedControlsSinceParams{OrgID: orgID, Since: since})
	if err != nil {
		return 0, fmt.Errorf("count closed controls since: %w", err)
	}
	return int(n), nil
}

// CountResolvedFindingsSince returns the number of findings set to 'resolved' since `since`.
func (r *Repository) CountResolvedFindingsSince(ctx context.Context, orgID string, since time.Time) (int, error) {
	n, err := r.q.CountSPResolvedFindingsSince(ctx, db.CountSPResolvedFindingsSinceParams{OrgID: orgID, Since: since})
	if err != nil {
		return 0, fmt.Errorf("count resolved findings since: %w", err)
	}
	return int(n), nil
}

func incidentFromCreateRow(row db.CreateCKIncidentRow) Incident {
	return incidentFromFields(incidentFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title,
		Description: row.Description, Severity: row.Severity, Status: row.Status,
		DiscoveredAt: row.DiscoveredAt, ResolvedAt: row.ResolvedAt,
		AffectedSystems: row.AffectedSystems, BreachID: row.BreachID,
		IncidentType: row.IncidentType, ReportingObligation: row.ReportingObligation,
		NotificationAuthority: row.NotificationAuthority,
		Deadline4h:            row.Deadline4h, Deadline24h: row.Deadline24h,
		Deadline72h: row.Deadline72h, Deadline30d: row.Deadline30d,
		Reported4hAt: row.Reported4hAt, Reported24hAt: row.Reported24hAt,
		Reported72hAt: row.Reported72hAt, Reported30dAt: row.Reported30dAt,
		AffectedCustomers: row.AffectedCustomers, FinancialImpactEstimate: row.FinancialImpactEstimate,
		IsMajorIncident: row.IsMajorIncident, SupplierID: row.SupplierID,
		NotifiedWarn24h: row.NotifiedWarn24h, NotifiedWarn72h: row.NotifiedWarn72h,
		NotifiedWarn30d: row.NotifiedWarn30d,
		CreatedAt:       row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
}

func incidentFromGetRow(row db.GetCKIncidentRow) Incident {
	return incidentFromFields(incidentFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title,
		Description: row.Description, Severity: row.Severity, Status: row.Status,
		DiscoveredAt: row.DiscoveredAt, ResolvedAt: row.ResolvedAt,
		AffectedSystems: row.AffectedSystems, BreachID: row.BreachID,
		IncidentType: row.IncidentType, ReportingObligation: row.ReportingObligation,
		NotificationAuthority: row.NotificationAuthority,
		Deadline4h:            row.Deadline4h, Deadline24h: row.Deadline24h,
		Deadline72h: row.Deadline72h, Deadline30d: row.Deadline30d,
		Reported4hAt: row.Reported4hAt, Reported24hAt: row.Reported24hAt,
		Reported72hAt: row.Reported72hAt, Reported30dAt: row.Reported30dAt,
		AffectedCustomers: row.AffectedCustomers, FinancialImpactEstimate: row.FinancialImpactEstimate,
		IsMajorIncident: row.IsMajorIncident, SupplierID: row.SupplierID,
		NotifiedWarn24h: row.NotifiedWarn24h, NotifiedWarn72h: row.NotifiedWarn72h,
		NotifiedWarn30d: row.NotifiedWarn30d,
		CreatedAt:       row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
}

func (r *Repository) ListIncidents(ctx context.Context, orgID string) ([]Incident, error) {
	rows, err := r.q.ListCKIncidents(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list incidents: %w", err)
	}
	out := make([]Incident, 0, len(rows))
	for _, row := range rows {
		out = append(out, incidentFromFields(incidentFields{
			ID: row.ID, OrgID: row.OrgID, Title: row.Title,
			Description: row.Description, Severity: row.Severity, Status: row.Status,
			DiscoveredAt: row.DiscoveredAt, ResolvedAt: row.ResolvedAt,
			AffectedSystems: row.AffectedSystems, BreachID: row.BreachID,
			IncidentType: row.IncidentType, ReportingObligation: row.ReportingObligation,
			NotificationAuthority: row.NotificationAuthority,
			Deadline4h:            row.Deadline4h, Deadline24h: row.Deadline24h,
			Deadline72h: row.Deadline72h, Deadline30d: row.Deadline30d,
			Reported4hAt: row.Reported4hAt, Reported24hAt: row.Reported24hAt,
			Reported72hAt: row.Reported72hAt, Reported30dAt: row.Reported30dAt,
			AffectedCustomers: row.AffectedCustomers, FinancialImpactEstimate: row.FinancialImpactEstimate,
			IsMajorIncident: row.IsMajorIncident, SupplierID: row.SupplierID,
			NotifiedWarn24h: row.NotifiedWarn24h, NotifiedWarn72h: row.NotifiedWarn72h,
			NotifiedWarn30d: row.NotifiedWarn30d,
			CreatedAt:       row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return out, nil
}

func (r *Repository) GetIncident(ctx context.Context, orgID, id string) (*Incident, error) {
	row, err := r.q.GetCKIncident(ctx, db.GetCKIncidentParams{ID: id, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get incident: %w", err)
	}
	inc := incidentFromGetRow(row)
	return &inc, nil
}

func (r *Repository) UpdateIncident(ctx context.Context, orgID, id string, in UpdateIncidentInput) (*Incident, error) {
	incType := in.IncidentType
	if incType == "" {
		incType = "general"
	}
	obligation := in.ReportingObligation
	if obligation == "" {
		obligation = "unknown"
	}
	row, err := r.q.UpdateCKIncident(ctx, db.UpdateCKIncidentParams{
		ID:                      id,
		OrgID:                   orgID,
		Title:                   in.Title,
		Description:             in.Description,
		Severity:                in.Severity,
		Status:                  in.Status,
		AffectedSystems:         in.AffectedSystems,
		IncidentType:            incType,
		ReportingObligation:     obligation,
		NotificationAuthority:   ckOptText(in.NotificationAuthority),
		AffectedCustomers:       ckOptIntPtr(in.AffectedCustomers),
		FinancialImpactEstimate: optTextStrPtr(in.FinancialImpactEstimate),
		IsMajorIncident:         in.IsMajorIncident,
	})
	if err != nil {
		return nil, fmt.Errorf("update incident: %w", err)
	}
	inc := incidentFromFields(incidentFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title,
		Description: row.Description, Severity: row.Severity, Status: row.Status,
		DiscoveredAt: row.DiscoveredAt, ResolvedAt: row.ResolvedAt,
		AffectedSystems: row.AffectedSystems, BreachID: row.BreachID,
		IncidentType: row.IncidentType, ReportingObligation: row.ReportingObligation,
		NotificationAuthority: row.NotificationAuthority,
		Deadline4h:            row.Deadline4h, Deadline24h: row.Deadline24h,
		Deadline72h: row.Deadline72h, Deadline30d: row.Deadline30d,
		Reported4hAt: row.Reported4hAt, Reported24hAt: row.Reported24hAt,
		Reported72hAt: row.Reported72hAt, Reported30dAt: row.Reported30dAt,
		AffectedCustomers: row.AffectedCustomers, FinancialImpactEstimate: row.FinancialImpactEstimate,
		IsMajorIncident: row.IsMajorIncident, SupplierID: row.SupplierID,
		NotifiedWarn24h: row.NotifiedWarn24h, NotifiedWarn72h: row.NotifiedWarn72h,
		NotifiedWarn30d: row.NotifiedWarn30d,
		CreatedAt:       row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &inc, nil
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
	row, err := r.q.CreateCKIncident(ctx, db.CreateCKIncidentParams{
		OrgID:                   orgID,
		Title:                   in.Title,
		Description:             in.Description,
		Severity:                in.Severity,
		DiscoveredAt:            pgtype.Timestamptz{Time: in.DiscoveredAt, Valid: true},
		AffectedSystems:         in.AffectedSystems,
		BreachID:                ckOptUUIDFromPtr(in.BreachID),
		IncidentType:            incType,
		ReportingObligation:     obligation,
		NotificationAuthority:   ckOptText(in.NotificationAuthority),
		Deadline4h:              ckOptTsPtr(d4h),
		Deadline24h:             ckOptTsPtr(d24h),
		Deadline72h:             ckOptTsPtr(d72h),
		Deadline30d:             ckOptTsPtr(d30d),
		AffectedCustomers:       ckOptIntPtr(in.AffectedCustomers),
		FinancialImpactEstimate: optTextStrPtr(in.FinancialImpactEstimate),
		IsMajorIncident:         in.IsMajorIncident,
	})
	if err != nil {
		return nil, fmt.Errorf("create incident: %w", err)
	}
	inc := incidentFromCreateRow(row)
	return &inc, nil
}

// ListIncidentsByType returns all non-closed incidents of a specific type for an organisation.
func (r *Repository) ListIncidentsByType(ctx context.Context, orgID, incidentType string) ([]Incident, error) {
	rows, err := r.q.ListCKIncidentsByType(ctx, db.ListCKIncidentsByTypeParams{OrgID: orgID, IncidentType: incidentType})
	if err != nil {
		return nil, fmt.Errorf("list incidents by type: %w", err)
	}
	out := make([]Incident, 0, len(rows))
	for _, row := range rows {
		out = append(out, incidentFromFields(incidentFields{
			ID: row.ID, OrgID: row.OrgID, Title: row.Title,
			Description: row.Description, Severity: row.Severity, Status: row.Status,
			DiscoveredAt: row.DiscoveredAt, ResolvedAt: row.ResolvedAt,
			AffectedSystems: row.AffectedSystems, BreachID: row.BreachID,
			IncidentType: row.IncidentType, ReportingObligation: row.ReportingObligation,
			NotificationAuthority: row.NotificationAuthority,
			Deadline4h:            row.Deadline4h, Deadline24h: row.Deadline24h,
			Deadline72h: row.Deadline72h, Deadline30d: row.Deadline30d,
			Reported4hAt: row.Reported4hAt, Reported24hAt: row.Reported24hAt,
			Reported72hAt: row.Reported72hAt, Reported30dAt: row.Reported30dAt,
			AffectedCustomers: row.AffectedCustomers, FinancialImpactEstimate: row.FinancialImpactEstimate,
			IsMajorIncident: row.IsMajorIncident, SupplierID: row.SupplierID,
			NotifiedWarn24h: row.NotifiedWarn24h, NotifiedWarn72h: row.NotifiedWarn72h,
			NotifiedWarn30d: row.NotifiedWarn30d,
			CreatedAt:       row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return out, nil
}

func (r *Repository) MarkDeadlineReported(ctx context.Context, orgID, id, deadline string) (*Incident, error) {
	if deadline != "4h" && deadline != "24h" && deadline != "72h" && deadline != "30d" {
		return nil, fmt.Errorf("unknown deadline: %s", deadline)
	}
	row, err := r.q.MarkCKIncidentDeadlineReported(ctx, db.MarkCKIncidentDeadlineReportedParams{
		ID:       id,
		OrgID:    orgID,
		Deadline: deadline,
	})
	if err != nil {
		return nil, fmt.Errorf("mark deadline reported: %w", err)
	}
	inc := incidentFromFields(incidentFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title,
		Description: row.Description, Severity: row.Severity, Status: row.Status,
		DiscoveredAt: row.DiscoveredAt, ResolvedAt: row.ResolvedAt,
		AffectedSystems: row.AffectedSystems, BreachID: row.BreachID,
		IncidentType: row.IncidentType, ReportingObligation: row.ReportingObligation,
		NotificationAuthority: row.NotificationAuthority,
		Deadline4h:            row.Deadline4h, Deadline24h: row.Deadline24h,
		Deadline72h: row.Deadline72h, Deadline30d: row.Deadline30d,
		Reported4hAt: row.Reported4hAt, Reported24hAt: row.Reported24hAt,
		Reported72hAt: row.Reported72hAt, Reported30dAt: row.Reported30dAt,
		AffectedCustomers: row.AffectedCustomers, FinancialImpactEstimate: row.FinancialImpactEstimate,
		IsMajorIncident: row.IsMajorIncident, SupplierID: row.SupplierID,
		NotifiedWarn24h: row.NotifiedWarn24h, NotifiedWarn72h: row.NotifiedWarn72h,
		NotifiedWarn30d: row.NotifiedWarn30d,
		CreatedAt:       row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &inc, nil
}

// UpdateIncidentReportability stores the questionnaire answers and updates
// reporting_obligation, notification_authority, and gdpr_notification_required.
func (r *Repository) UpdateIncidentReportability(
	ctx context.Context,
	orgID, incidentID, obligation, authority string,
	gdprRequired bool,
	answersJSON []byte,
) error {
	if err := r.q.UpdateCKIncidentReportability(ctx, db.UpdateCKIncidentReportabilityParams{
		ID:                       incidentID,
		OrgID:                    orgID,
		ReportingObligation:      obligation,
		NotificationAuthority:    ckOptText(authority),
		GdprNotificationRequired: gdprRequired,
		ReportabilityAnswers:     answersJSON,
	}); err != nil {
		return fmt.Errorf("update incident reportability: %w", err)
	}
	return nil
}

// SaveIncidentReport archives a generated Meldungsformular with optional PDF bytes.
func (r *Repository) SaveIncidentReport(ctx context.Context, orgID, incidentID, reportType, authority string, pdfData []byte, metadata []byte) (*IncidentReport, error) {
	row, err := r.q.SaveCKIncidentReport(ctx, db.SaveCKIncidentReportParams{
		OrgID:      orgID,
		IncidentID: incidentID,
		ReportType: reportType,
		Authority:  authority,
		PdfData:    pdfData,
		Metadata:   metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("save incident report: %w", err)
	}
	return &IncidentReport{
		ID:          row.ID,
		OrgID:       row.OrgID,
		IncidentID:  row.IncidentID,
		ReportType:  row.ReportType,
		Authority:   row.Authority,
		GeneratedAt: ckTsToTime(row.GeneratedAt),
	}, nil
}

// ListIncidentReports returns all archived reports for a given incident.
func (r *Repository) ListIncidentReports(ctx context.Context, orgID, incidentID string) ([]IncidentReport, error) {
	rows, err := r.q.ListCKIncidentReports(ctx, db.ListCKIncidentReportsParams{OrgID: orgID, IncidentID: incidentID})
	if err != nil {
		return nil, fmt.Errorf("list incident reports: %w", err)
	}
	reports := make([]IncidentReport, 0, len(rows))
	for _, row := range rows {
		reports = append(reports, IncidentReport{
			ID:          row.ID,
			OrgID:       row.OrgID,
			IncidentID:  row.IncidentID,
			ReportType:  row.ReportType,
			Authority:   row.Authority,
			GeneratedAt: ckTsToTime(row.GeneratedAt),
		})
	}
	return reports, nil
}

// GetIncidentReportPDF returns the stored PDF bytes for a report entry.
func (r *Repository) GetIncidentReportPDF(ctx context.Context, orgID, reportID string) ([]byte, error) {
	data, err := r.q.GetCKIncidentReportPDF(ctx, db.GetCKIncidentReportPDFParams{ID: reportID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get incident report pdf: %w", err)
	}
	return data, nil
}

// MarkIncidentWarnNotified sets the notified_warn_* flag for a given deadline
// so the 12h-before warning is only sent once per incident + deadline pair.
func (r *Repository) MarkIncidentWarnNotified(ctx context.Context, orgID, incidentID, deadline string) error {
	if deadline != "24h" && deadline != "72h" && deadline != "30d" {
		return fmt.Errorf("unknown deadline: %s", deadline)
	}
	return r.q.MarkCKIncidentWarnNotified(ctx, db.MarkCKIncidentWarnNotifiedParams{
		ID:       incidentID,
		OrgID:    orgID,
		Deadline: deadline,
	})
}

// GetOrgSector returns the sector and federal_state for the given org.
func (r *Repository) GetOrgSector(ctx context.Context, orgID string) (*OrgSectorSettings, error) {
	row, err := r.q.GetCKOrgSector(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("get org sector: %w", err)
	}
	return &OrgSectorSettings{
		Sector:       row.Sector,
		FederalState: row.FederalState,
	}, nil
}

// UpdateOrgSector sets the sector and federal_state for the given org.
func (r *Repository) UpdateOrgSector(ctx context.Context, orgID, sector, federalState string) error {
	if _, err := r.q.UpdateCKOrgSector(ctx, db.UpdateCKOrgSectorParams{
		ID:           orgID,
		Sector:       sector,
		FederalState: federalState,
	}); err != nil {
		return fmt.Errorf("update org sector: %w", err)
	}
	return nil
}

// GetAdminEmails returns the e-mail addresses of active Admin users for the given org.
func (r *Repository) GetAdminEmails(ctx context.Context, orgID string) ([]string, error) {
	return r.q.GetCKOrgAdminEmails(ctx, orgID)
}

// ListIncidentsBySupplier returns all incidents linked to a given supplier via supplier_id FK.
func (r *Repository) ListIncidentsBySupplier(ctx context.Context, orgID, supplierID string) ([]Incident, error) {
	rows, err := r.q.ListCKIncidentsBySupplier(ctx, db.ListCKIncidentsBySupplierParams{
		OrgID:      orgID,
		SupplierID: ckOptUUIDFromStr(supplierID),
	})
	if err != nil {
		return nil, fmt.Errorf("list incidents by supplier: %w", err)
	}
	out := make([]Incident, 0, len(rows))
	for _, row := range rows {
		out = append(out, incidentFromFields(incidentFields{
			ID: row.ID, OrgID: row.OrgID, Title: row.Title,
			Description: row.Description, Severity: row.Severity, Status: row.Status,
			DiscoveredAt: row.DiscoveredAt, ResolvedAt: row.ResolvedAt,
			AffectedSystems: row.AffectedSystems, BreachID: row.BreachID,
			IncidentType: row.IncidentType, ReportingObligation: row.ReportingObligation,
			NotificationAuthority: row.NotificationAuthority,
			Deadline4h:            row.Deadline4h, Deadline24h: row.Deadline24h,
			Deadline72h: row.Deadline72h, Deadline30d: row.Deadline30d,
			Reported4hAt: row.Reported4hAt, Reported24hAt: row.Reported24hAt,
			Reported72hAt: row.Reported72hAt, Reported30dAt: row.Reported30dAt,
			AffectedCustomers: row.AffectedCustomers, FinancialImpactEstimate: row.FinancialImpactEstimate,
			IsMajorIncident: row.IsMajorIncident, SupplierID: row.SupplierID,
			NotifiedWarn24h: row.NotifiedWarn24h, NotifiedWarn72h: row.NotifiedWarn72h,
			NotifiedWarn30d: row.NotifiedWarn30d,
			CreatedAt:       row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return out, nil
}

// ListIncidentsPaged returns a page of incidents plus the total count.
func (r *Repository) ListIncidentsPaged(ctx context.Context, orgID string, offset, limit int) ([]Incident, int, error) {
	total, err := r.q.CountCKIncidents(ctx, orgID)
	if err != nil {
		return nil, 0, fmt.Errorf("count incidents: %w", err)
	}
	rows, err := r.q.ListCKIncidentsPaged(ctx, db.ListCKIncidentsPagedParams{
		OrgID:  orgID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list incidents paged: %w", err)
	}
	incidents := make([]Incident, 0, len(rows))
	for _, row := range rows {
		incidents = append(incidents, incidentFromFields(incidentFields{
			ID: row.ID, OrgID: row.OrgID, Title: row.Title,
			Description: row.Description, Severity: row.Severity, Status: row.Status,
			DiscoveredAt: row.DiscoveredAt, ResolvedAt: row.ResolvedAt,
			AffectedSystems: row.AffectedSystems, BreachID: row.BreachID,
			IncidentType: row.IncidentType, ReportingObligation: row.ReportingObligation,
			NotificationAuthority: row.NotificationAuthority,
			Deadline4h:            row.Deadline4h, Deadline24h: row.Deadline24h,
			Deadline72h: row.Deadline72h, Deadline30d: row.Deadline30d,
			Reported4hAt: row.Reported4hAt, Reported24hAt: row.Reported24hAt,
			Reported72hAt: row.Reported72hAt, Reported30dAt: row.Reported30dAt,
			AffectedCustomers: row.AffectedCustomers, FinancialImpactEstimate: row.FinancialImpactEstimate,
			IsMajorIncident: row.IsMajorIncident, SupplierID: row.SupplierID,
			NotifiedWarn24h: row.NotifiedWarn24h, NotifiedWarn72h: row.NotifiedWarn72h,
			NotifiedWarn30d: row.NotifiedWarn30d,
			CreatedAt:       row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return incidents, int(total), nil
}

// UpdateIncidentDORADeadlineStatus persists the computed Ampel-Status map to
// ck_incidents.dora_deadline_status JSONB. S37-4.
func (r *Repository) UpdateIncidentDORADeadlineStatus(ctx context.Context, incidentID string, status map[string]string) error {
	b, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("marshal dora deadline status: %w", err)
	}
	// orgid-lint: global — UPDATE by PK; incidentID comes from a prior org-scoped query in the background job
	_, err = r.db.Exec(ctx,
		`UPDATE ck_incidents SET dora_deadline_status = $2, updated_at = NOW() WHERE id = $1::uuid`,
		incidentID, b,
	)
	return err
}

// SaveClassificationResult persists the classify-reporting wizard result to
// ck_incidents.classification_result JSONB (S39-1, Migration 140).
func (r *Repository) SaveClassificationResult(ctx context.Context, orgID, incidentID string, result ClassificationResult) error {
	b, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal classification result: %w", err)
	}
	tag, err := r.db.Exec(ctx,
		`UPDATE ck_incidents
		    SET classification_result = $3, updated_at = NOW()
		  WHERE id = $1::uuid AND org_id = $2::uuid`,
		incidentID, orgID, b,
	)
	if err != nil {
		return fmt.Errorf("save classification result: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ListNIS2ClassifiedIncidents returns all incidents where the classification
// wizard marked the obligation as "probably" — used by the NIS2 deadline check job (S39-2).
func (r *Repository) ListNIS2ClassifiedIncidents(ctx context.Context, orgID string) ([]Incident, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id::text, org_id::text, title, description, severity, status,
		        discovered_at, resolved_at, affected_systems, incident_type,
		        reporting_obligation, notification_authority,
		        deadline_24h, deadline_72h, deadline_30d,
		        reported_24h_at, reported_72h_at, reported_30d_at,
		        notified_warn_24h, notified_warn_72h, notified_warn_30d,
		        created_at, updated_at
		   FROM ck_incidents
		  WHERE org_id = $1::uuid
		    AND classification_result->>'obligation' = 'probably'
		    AND status NOT IN ('resolved','closed')`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list nis2 classified incidents: %w", err)
	}
	defer rows.Close()

	var out []Incident
	for rows.Next() {
		var inc Incident
		var desc, severity, status, incType, obligation pgtype.Text
		var authority pgtype.Text
		var resolvedAt, d24h, d72h, d30d pgtype.Timestamptz
		var r24h, r72h, r30d pgtype.Timestamptz
		var systems []string
		var warn24h, warn72h, warn30d bool
		var createdAt, updatedAt pgtype.Timestamptz

		if err := rows.Scan(
			&inc.ID, &inc.OrgID, &inc.Title, &desc, &severity, &status,
			&inc.DiscoveredAt, &resolvedAt, &systems, &incType,
			&obligation, &authority,
			&d24h, &d72h, &d30d,
			&r24h, &r72h, &r30d,
			&warn24h, &warn72h, &warn30d,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan nis2 classified incident: %w", err)
		}
		inc.Description = desc.String
		inc.Severity = severity.String
		inc.Status = status.String
		inc.AffectedSystems = systems
		inc.IncidentType = incType.String
		inc.ReportingObligation = obligation.String
		inc.NotificationAuthority = authority.String
		inc.ResolvedAt = ckTsToTimePtr(resolvedAt)
		inc.Deadline24h = ckTsToTimePtr(d24h)
		inc.Deadline72h = ckTsToTimePtr(d72h)
		inc.Deadline30d = ckTsToTimePtr(d30d)
		inc.Reported24hAt = ckTsToTimePtr(r24h)
		inc.Reported72hAt = ckTsToTimePtr(r72h)
		inc.Reported30dAt = ckTsToTimePtr(r30d)
		inc.NotifiedWarn24h = warn24h
		inc.NotifiedWarn72h = warn72h
		inc.NotifiedWarn30d = warn30d
		inc.CreatedAt = ckTsToTime(createdAt)
		inc.UpdatedAt = ckTsToTime(updatedAt)
		out = append(out, inc)
	}
	return out, rows.Err()
}

// CountRecentIncidents returns the number of incidents created at or after `since`.
func (r *Repository) CountRecentIncidents(ctx context.Context, orgID string, since time.Time) (int, error) {
	n, err := r.q.CountCKRecentIncidents(ctx, db.CountCKRecentIncidentsParams{OrgID: orgID, Since: since})
	if err != nil {
		return 0, fmt.Errorf("count recent incidents: %w", err)
	}
	return int(n), nil
}

// CountIncidentsSince returns the number of incidents created at or after `since`.
func (r *Repository) CountIncidentsSince(ctx context.Context, orgID string, since time.Time) (int, error) {
	n, err := r.q.CountCKIncidentsSince(ctx, db.CountCKIncidentsSinceParams{OrgID: orgID, Since: since})
	if err != nil {
		return 0, fmt.Errorf("count incidents since: %w", err)
	}
	return int(n), nil
}

type ChangeLogEntry struct {
	ID        string    `json:"id"`
	ControlID string    `json:"control_id"`
	UserEmail string    `json:"user_email"`
	Field     string    `json:"field"`
	OldValue  string    `json:"old_value"`
	NewValue  string    `json:"new_value"`
	ChangedAt time.Time `json:"changed_at"`
}

// AppendControlChange inserts a change log entry into ck_control_changelog.
// Errors are logged but not returned — a changelog write failure must never
// abort the primary update operation.
func (r *Repository) AppendControlChange(ctx context.Context, orgID, controlID, userID, userEmail, field, oldVal, newVal string) {
	err := r.q.AppendCKControlChange(ctx, db.AppendCKControlChangeParams{
		ControlID: controlID,
		OrgID:     orgID,
		UserID:    ckOptUUIDFromStr(userID),
		UserEmail: ckOptText(userEmail),
		Field:     field,
		OldValue:  ckOptText(oldVal),
		NewValue:  ckOptText(newVal),
	})
	if err != nil {
		log.Error().
			Err(err).
			Str("control_id", controlID).
			Str("field", field).
			Msg("changelog: failed to append control change")
	}
}

// ListControlChanges returns the last 50 field-level changes for a control,
// ordered newest first.
func (r *Repository) ListControlChanges(ctx context.Context, orgID, controlID string) ([]ChangeLogEntry, error) {
	rows, err := r.q.ListCKControlChanges(ctx, db.ListCKControlChangesParams{OrgID: orgID, ControlID: controlID})
	if err != nil {
		return nil, err
	}
	out := make([]ChangeLogEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, ChangeLogEntry{
			ID:        row.ID,
			ControlID: row.ControlID,
			UserEmail: row.UserEmail.String,
			Field:     row.Field,
			OldValue:  row.OldValue.String,
			NewValue:  row.NewValue.String,
			ChangedAt: ckTsToTime(row.ChangedAt),
		})
	}
	return out, nil
}
