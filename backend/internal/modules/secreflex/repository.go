package secreflex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/matharnica/vakt/internal/db"
)

// Repository handles Vakt Aware (SecReflex) data access via sqlc-generated
// queries. Tables were renamed from `pg_*` → `sr_*` in migration 122 to
// unblock the sqlc parser (see ADR-0005).
type Repository struct {
	db *pgxpool.Pool
	q  *db.Queries
}

// NewRepository creates a new SecReflex repository backed by the given pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	if pool == nil {
		return &Repository{}
	}
	return &Repository{db: pool, q: db.New(pool)}
}

// ── pgtype <-> domain helpers ─────────────────────────────────────────────

// optText collapses an empty string into a NULL pgtype.Text.
func optText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// textOrEmpty returns the inner string, or "" if NULL.
func textOrEmpty(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

// textOrNilPtr returns *string (nil when NULL).
func textOrNilPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

// uuidPtrFromUUID returns the UUID string pointer, or nil when NULL.
func uuidPtrFromUUID(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	s := u.String()
	return &s
}

// optUUIDFromPtr converts an optional UUID-string pointer into a pgtype.UUID.
func optUUIDFromPtr(s *string) pgtype.UUID {
	if s == nil || *s == "" {
		return pgtype.UUID{}
	}
	var u pgtype.UUID
	_ = u.Scan(*s)
	return u
}

// optUUIDFromString converts a UUID string into a pgtype.UUID (NULL on "").
func optUUIDFromString(s string) pgtype.UUID {
	if s == "" {
		return pgtype.UUID{}
	}
	var u pgtype.UUID
	_ = u.Scan(s)
	return u
}

// optTimestamptz wraps an optional time pointer.
func optTimestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// tsToTime returns the inner time, or zero if NULL.
func tsToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

// tsToTimePtr returns *time.Time, nil when NULL.
func tsToTimePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	tt := t.Time
	return &tt
}

// optInt4 wraps an optional int pointer into pgtype.Int4.
func optInt4(p *int) pgtype.Int4 {
	if p == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*p), Valid: true}
}

// int4ToIntPtr returns *int (nil when NULL).
func int4ToIntPtr(v pgtype.Int4) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int32)
	return &n
}

// ── Domain mappers (db row → domain model) ────────────────────────────────

func templateFromRow(r db.SrTemplates) Template {
	return Template{
		ID:         r.ID,
		OrgID:      r.OrgID,
		Name:       r.Name,
		Subject:    r.Subject,
		FromName:   r.FromName,
		FromEmail:  r.FromEmail,
		HTMLBody:   r.HtmlBody,
		AttackType: r.AttackType,
		IsPreset:   r.IsPreset,
		CreatedBy:  uuidPtrFromUUID(r.CreatedBy),
		CreatedAt:  tsToTime(r.CreatedAt),
	}
}

func targetGroupFromRow(r db.SrTargetGroups) TargetGroup {
	return TargetGroup{
		ID:        r.ID,
		OrgID:     r.OrgID,
		Name:      r.Name,
		Source:    r.Source,
		ADOU:      textOrNilPtr(r.AdOu),
		CreatedAt: tsToTime(r.CreatedAt),
	}
}

func targetFromRow(r db.SrTargets) Target {
	return Target{
		ID:         r.ID,
		OrgID:      r.OrgID,
		GroupID:    r.GroupID,
		Email:      r.Email,
		FirstName:  r.FirstName,
		LastName:   r.LastName,
		Department: r.Department,
		IsBounced:  r.IsBounced,
		CreatedAt:  tsToTime(r.CreatedAt),
	}
}

func landingPageFromRow(r db.SrLandingPages) LandingPage {
	return LandingPage{
		ID:          r.ID,
		OrgID:       r.OrgID,
		Name:        r.Name,
		HTMLContent: r.HtmlContent,
		CreatedAt:   tsToTime(r.CreatedAt),
	}
}

// campaignFields is the set of campaign columns common to every campaign-
// returning sqlc row. sqlc emits a separate Row type per query whose only
// difference is the field declaration order — we extract them explicitly so
// the mapping logic lives in one place.
type campaignFields struct {
	ID, OrgID, Name, Status, FromName, FromEmail, Subject string
	TemplateID, GroupID, LandingPageID, CreatedBy         pgtype.UUID
	ScheduledAt, StartedAt, CompletedAt                   pgtype.Timestamptz
	Recurrence                                            pgtype.Text
	TrackOpens, BetriebsratMode                           bool
	CreatedAt, UpdatedAt                                  pgtype.Timestamptz
}

func campaignFromFields(f campaignFields) Campaign {
	return Campaign{
		ID:              f.ID,
		OrgID:           f.OrgID,
		Name:            f.Name,
		Status:          f.Status,
		TemplateID:      uuidPtrFromUUID(f.TemplateID),
		GroupID:         uuidPtrFromUUID(f.GroupID),
		LandingPageID:   uuidPtrFromUUID(f.LandingPageID),
		FromName:        f.FromName,
		FromEmail:       f.FromEmail,
		Subject:         f.Subject,
		ScheduledAt:     tsToTimePtr(f.ScheduledAt),
		StartedAt:       tsToTimePtr(f.StartedAt),
		CompletedAt:     tsToTimePtr(f.CompletedAt),
		Recurrence:      textOrNilPtr(f.Recurrence),
		TrackOpens:      f.TrackOpens,
		BetriebsratMode: f.BetriebsratMode,
		CreatedBy:       uuidPtrFromUUID(f.CreatedBy),
		CreatedAt:       tsToTime(f.CreatedAt),
		UpdatedAt:       tsToTime(f.UpdatedAt),
	}
}

func campaignFromCreateRow(r db.CreateSRCampaignRow) Campaign {
	return campaignFromFields(campaignFields{
		ID: r.ID, OrgID: r.OrgID, Name: r.Name, Status: r.Status,
		FromName: r.FromName, FromEmail: r.FromEmail, Subject: r.Subject,
		TemplateID: r.TemplateID, GroupID: r.GroupID, LandingPageID: r.LandingPageID,
		CreatedBy:   r.CreatedBy,
		ScheduledAt: r.ScheduledAt, StartedAt: r.StartedAt, CompletedAt: r.CompletedAt,
		Recurrence: r.Recurrence,
		TrackOpens: r.TrackOpens, BetriebsratMode: r.BetriebsratMode,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	})
}

func campaignFromGetRow(r db.GetSRCampaignRow) Campaign {
	return campaignFromFields(campaignFields{
		ID: r.ID, OrgID: r.OrgID, Name: r.Name, Status: r.Status,
		FromName: r.FromName, FromEmail: r.FromEmail, Subject: r.Subject,
		TemplateID: r.TemplateID, GroupID: r.GroupID, LandingPageID: r.LandingPageID,
		CreatedBy:   r.CreatedBy,
		ScheduledAt: r.ScheduledAt, StartedAt: r.StartedAt, CompletedAt: r.CompletedAt,
		Recurrence: r.Recurrence,
		TrackOpens: r.TrackOpens, BetriebsratMode: r.BetriebsratMode,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	})
}

func campaignFromListRow(r db.ListSRCampaignsRow) Campaign {
	return campaignFromFields(campaignFields{
		ID: r.ID, OrgID: r.OrgID, Name: r.Name, Status: r.Status,
		FromName: r.FromName, FromEmail: r.FromEmail, Subject: r.Subject,
		TemplateID: r.TemplateID, GroupID: r.GroupID, LandingPageID: r.LandingPageID,
		CreatedBy:   r.CreatedBy,
		ScheduledAt: r.ScheduledAt, StartedAt: r.StartedAt, CompletedAt: r.CompletedAt,
		Recurrence: r.Recurrence,
		TrackOpens: r.TrackOpens, BetriebsratMode: r.BetriebsratMode,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	})
}

func campaignFromTrackingRow(r db.GetSRCampaignByTrackingTokenRow) Campaign {
	return campaignFromFields(campaignFields{
		ID: r.ID, OrgID: r.OrgID, Name: r.Name, Status: r.Status,
		FromName: r.FromName, FromEmail: r.FromEmail, Subject: r.Subject,
		TemplateID: r.TemplateID, GroupID: r.GroupID, LandingPageID: r.LandingPageID,
		CreatedBy:   r.CreatedBy,
		ScheduledAt: r.ScheduledAt, StartedAt: r.StartedAt, CompletedAt: r.CompletedAt,
		Recurrence: r.Recurrence,
		TrackOpens: r.TrackOpens, BetriebsratMode: r.BetriebsratMode,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	})
}

func trainingModuleFromRow(r db.SrTrainingModules) TrainingModule {
	m := TrainingModule{
		ID:              r.ID,
		OrgID:           r.OrgID,
		Title:           r.Title,
		Type:            r.Type,
		AttackType:      r.AttackType,
		ContentURL:      r.ContentUrl,
		DurationSeconds: int(r.DurationSeconds),
		PassingScore:    int(r.PassingScore),
		CreatedBy:       uuidPtrFromUUID(r.CreatedBy),
		CreatedAt:       tsToTime(r.CreatedAt),
	}
	if len(r.Questions) > 0 {
		_ = json.Unmarshal(r.Questions, &m.Questions)
	}
	return m
}

func assignmentFromRow(r db.SrAssignments) Assignment {
	return Assignment{
		ID:         r.ID,
		OrgID:      r.OrgID,
		ModuleID:   r.ModuleID,
		TargetID:   uuidPtrFromUUID(r.TargetID),
		Department: textOrEmpty(r.Department),
		DueDate:    tsToTime(r.DueDate),
		IsOverdue:  r.IsOverdue,
		CreatedAt:  tsToTime(r.CreatedAt),
	}
}

func completionFromRow(r db.SrCompletions) Completion {
	return Completion{
		ID:           r.ID,
		OrgID:        r.OrgID,
		AssignmentID: r.AssignmentID,
		Score:        int4ToIntPtr(r.Score),
		Passed:       r.Passed,
		CompletedAt:  tsToTime(r.CompletedAt),
	}
}

func phishReportFromRow(r db.SrPhishReports) PhishReport {
	return PhishReport{
		ID:            r.ID,
		OrgID:         r.OrgID,
		CampaignID:    uuidPtrFromUUID(r.CampaignID),
		ReporterEmail: r.ReporterEmail,
		ReportedAt:    tsToTime(r.ReportedAt),
		Subject:       textOrEmpty(r.Subject),
		Sender:        textOrEmpty(r.Sender),
		IsSimulation:  r.IsSimulation,
		CreatedAt:     tsToTime(r.CreatedAt),
	}
}

// ── Templates ─────────────────────────────────────────────────────────────

func (r *Repository) CreateTemplate(ctx context.Context, orgID, userID string, input CreateTemplateInput) (*Template, error) {
	row, err := r.q.CreateSRTemplate(ctx, db.CreateSRTemplateParams{
		OrgID:      orgID,
		Name:       input.Name,
		Subject:    input.Subject,
		FromName:   input.FromName,
		FromEmail:  input.FromEmail,
		HtmlBody:   input.HTMLBody,
		AttackType: input.AttackType,
		CreatedBy:  optUUIDFromString(userID),
	})
	if err != nil {
		return nil, fmt.Errorf("create template: %w", err)
	}
	t := templateFromRow(row)
	return &t, nil
}

func (r *Repository) ListTemplates(ctx context.Context, orgID string) ([]Template, error) {
	rows, err := r.q.ListSRTemplates(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	out := make([]Template, 0, len(rows))
	for _, row := range rows {
		out = append(out, templateFromRow(row))
	}
	return out, nil
}

// GetTemplate returns a template by ID within the org.
func (r *Repository) GetTemplate(ctx context.Context, orgID, templateID string) (*Template, error) {
	row, err := r.q.GetSRTemplate(ctx, db.GetSRTemplateParams{
		ID:    templateID,
		OrgID: orgID,
	})
	if err != nil {
		return nil, fmt.Errorf("get template: %w", err)
	}
	t := templateFromRow(row)
	return &t, nil
}

// ── Target groups ─────────────────────────────────────────────────────────

func (r *Repository) CreateTargetGroup(ctx context.Context, orgID, name, source string) (*TargetGroup, error) {
	row, err := r.q.CreateSRTargetGroup(ctx, db.CreateSRTargetGroupParams{
		OrgID:  orgID,
		Name:   name,
		Source: source,
	})
	if err != nil {
		return nil, fmt.Errorf("create target group: %w", err)
	}
	g := targetGroupFromRow(row)
	return &g, nil
}

func (r *Repository) ListTargetGroups(ctx context.Context, orgID string) ([]TargetGroup, error) {
	rows, err := r.q.ListSRTargetGroups(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list target groups: %w", err)
	}
	out := make([]TargetGroup, 0, len(rows))
	for _, row := range rows {
		out = append(out, targetGroupFromRow(row))
	}
	return out, nil
}

func (r *Repository) CreateTarget(ctx context.Context, orgID, groupID, email, firstName, lastName, department string) (*Target, error) {
	row, err := r.q.UpsertSRTarget(ctx, db.UpsertSRTargetParams{
		OrgID:      orgID,
		GroupID:    groupID,
		Email:      email,
		FirstName:  firstName,
		LastName:   lastName,
		Department: department,
	})
	if err != nil {
		return nil, fmt.Errorf("upsert target: %w", err)
	}
	t := targetFromRow(row)
	return &t, nil
}

func (r *Repository) ListTargets(ctx context.Context, orgID, groupID string) ([]Target, error) {
	rows, err := r.q.ListSRTargets(ctx, db.ListSRTargetsParams{
		GroupID: groupID,
		OrgID:   orgID,
	})
	if err != nil {
		return nil, fmt.Errorf("list targets: %w", err)
	}
	out := make([]Target, 0, len(rows))
	for _, row := range rows {
		out = append(out, targetFromRow(row))
	}
	return out, nil
}

func (r *Repository) CountTargetsInGroup(ctx context.Context, groupID string) (int, error) {
	n, err := r.q.CountSRTargetsInGroup(ctx, groupID)
	if err != nil {
		return 0, fmt.Errorf("count targets: %w", err)
	}
	return int(n), nil
}

// ── Landing pages ─────────────────────────────────────────────────────────

func (r *Repository) CreateLandingPage(ctx context.Context, orgID, name, html string) (*LandingPage, error) {
	row, err := r.q.CreateSRLandingPage(ctx, db.CreateSRLandingPageParams{
		OrgID:       orgID,
		Name:        name,
		HtmlContent: html,
	})
	if err != nil {
		return nil, fmt.Errorf("create landing page: %w", err)
	}
	p := landingPageFromRow(row)
	return &p, nil
}

func (r *Repository) ListLandingPages(ctx context.Context, orgID string) ([]LandingPage, error) {
	rows, err := r.q.ListSRLandingPages(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list landing pages: %w", err)
	}
	out := make([]LandingPage, 0, len(rows))
	for _, row := range rows {
		out = append(out, landingPageFromRow(row))
	}
	return out, nil
}

// ── Campaigns ─────────────────────────────────────────────────────────────

func (r *Repository) CreateCampaign(ctx context.Context, orgID, userID string, input CreateCampaignInput) (*Campaign, error) {
	recurrence := pgtype.Text{}
	if input.Recurrence != "" {
		recurrence = pgtype.Text{String: input.Recurrence, Valid: true}
	}
	row, err := r.q.CreateSRCampaign(ctx, db.CreateSRCampaignParams{
		OrgID:           orgID,
		Name:            input.Name,
		FromName:        input.FromName,
		FromEmail:       input.FromEmail,
		Subject:         input.Subject,
		TrackOpens:      input.TrackOpens,
		BetriebsratMode: input.BetriebsratMode,
		TemplateID:      optUUIDFromPtr(input.TemplateID),
		GroupID:         optUUIDFromPtr(input.GroupID),
		LandingPageID:   optUUIDFromPtr(input.LandingPageID),
		ScheduledAt:     optTimestamptz(input.ScheduledAt),
		Recurrence:      recurrence,
		CreatedBy:       optUUIDFromString(userID),
	})
	if err != nil {
		return nil, fmt.Errorf("create campaign: %w", err)
	}
	c := campaignFromCreateRow(row)
	return &c, nil
}

func (r *Repository) GetCampaign(ctx context.Context, orgID, campaignID string) (*Campaign, error) {
	row, err := r.q.GetSRCampaign(ctx, db.GetSRCampaignParams{
		ID:    campaignID,
		OrgID: orgID,
	})
	if err != nil {
		return nil, err
	}
	c := campaignFromGetRow(row)
	return &c, nil
}

func (r *Repository) ListCampaigns(ctx context.Context, orgID string) ([]Campaign, error) {
	rows, err := r.q.ListSRCampaigns(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list campaigns: %w", err)
	}
	out := make([]Campaign, 0, len(rows))
	for _, row := range rows {
		out = append(out, campaignFromListRow(row))
	}
	return out, nil
}

func (r *Repository) UpdateCampaignStatus(ctx context.Context, orgID, campaignID, status string) error {
	return r.q.UpdateSRCampaignStatus(ctx, db.UpdateSRCampaignStatusParams{
		Status: status,
		ID:     campaignID,
		OrgID:  orgID,
	})
}

func (r *Repository) GetCampaignStats(ctx context.Context, orgID, campaignID string) (*CampaignStats, error) {
	var stats CampaignStats
	groupID, err := r.q.GetSRCampaignGroupID(ctx, campaignID)
	if err == nil && groupID.Valid {
		// pgtype.UUID -> canonical string
		groupIDStr := groupID.String()
		n, cerr := r.q.CountSRTargetsInGroup(ctx, groupIDStr)
		if cerr == nil {
			stats.TotalTargets = int(n)
		}
	}
	stats.Opens, _ = r.countEventsByType(ctx, campaignID, "open")
	stats.Clicks, _ = r.countEventsByType(ctx, campaignID, "click")
	stats.FormSubmissions, _ = r.countEventsByType(ctx, campaignID, "form_submission")
	if stats.TotalTargets > 0 {
		stats.ClickRate = float64(stats.Clicks) / float64(stats.TotalTargets) * 100
		stats.SubmissionRate = float64(stats.FormSubmissions) / float64(stats.TotalTargets) * 100
	}
	return &stats, nil
}

func (r *Repository) countEventsByType(ctx context.Context, campaignID, eventType string) (int, error) {
	if eventType == "" {
		return 0, nil
	}
	n, err := r.q.CountSREventsByType(ctx, db.CountSREventsByTypeParams{
		CampaignID: campaignID,
		Type:       eventType,
	})
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// SetCampaignCompleted marks a campaign as completed and sets completed_at.
func (r *Repository) SetCampaignCompleted(ctx context.Context, orgID, campaignID string) error {
	return r.q.SetSRCampaignCompleted(ctx, db.SetSRCampaignCompletedParams{
		ID:    campaignID,
		OrgID: orgID,
	})
}

// ── Tracking events ───────────────────────────────────────────────────────

func (r *Repository) GetCampaignByTrackingToken(ctx context.Context, token string) (*Campaign, error) {
	row, err := r.q.GetSRCampaignByTrackingToken(ctx, token)
	if err != nil {
		return nil, err
	}
	c := campaignFromTrackingRow(row)
	return &c, nil
}

func (r *Repository) CreateTrackingEvent(ctx context.Context, orgID, campaignID string, targetID *string, department, token, eventType, ip, ua string) error {
	return r.q.CreateSRTrackingEvent(ctx, db.CreateSRTrackingEventParams{
		OrgID:         orgID,
		CampaignID:    campaignID,
		Type:          eventType,
		TrackingToken: token,
		TargetID:      optUUIDFromPtr(targetID),
		Department:    optText(department),
		IpAddress:     optText(ip),
		UserAgent:     optText(ua),
	})
}

func (r *Repository) GetLandingPageForCampaign(ctx context.Context, campaignID string) (*LandingPage, error) {
	row, err := r.q.GetSRLandingPageForCampaign(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	p := landingPageFromRow(row)
	return &p, nil
}

// ── Training modules ──────────────────────────────────────────────────────

func (r *Repository) CreateModule(ctx context.Context, orgID, userID string, input CreateModuleInput) (*TrainingModule, error) {
	questionsJSON, err := json.Marshal(input.Questions)
	if err != nil {
		return nil, fmt.Errorf("marshal questions: %w", err)
	}
	passingScore := input.PassingScore
	if passingScore == 0 {
		passingScore = 80
	}
	row, err := r.q.CreateSRTrainingModule(ctx, db.CreateSRTrainingModuleParams{
		OrgID:           orgID,
		Title:           input.Title,
		Type:            input.Type,
		AttackType:      input.AttackType,
		ContentUrl:      input.ContentURL,
		DurationSeconds: int32(input.DurationSeconds),
		PassingScore:    int32(passingScore),
		Questions:       questionsJSON,
		CreatedBy:       optUUIDFromString(userID),
	})
	if err != nil {
		return nil, fmt.Errorf("create training module: %w", err)
	}
	m := trainingModuleFromRow(row)
	return &m, nil
}

func (r *Repository) ListModules(ctx context.Context, orgID string) ([]TrainingModule, error) {
	rows, err := r.q.ListSRTrainingModules(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list training modules: %w", err)
	}
	out := make([]TrainingModule, 0, len(rows))
	for _, row := range rows {
		out = append(out, trainingModuleFromRow(row))
	}
	return out, nil
}

func (r *Repository) GetModuleByAttackType(ctx context.Context, orgID, attackType string) (*TrainingModule, error) {
	row, err := r.q.GetSRTrainingModuleByAttackType(ctx, db.GetSRTrainingModuleByAttackTypeParams{
		OrgID:      orgID,
		AttackType: attackType,
	})
	if err != nil {
		return nil, err
	}
	m := trainingModuleFromRow(row)
	return &m, nil
}

// GetModuleByID returns a training module by its ID within the org.
func (r *Repository) GetModuleByID(ctx context.Context, orgID, moduleID string) (*TrainingModule, error) {
	row, err := r.q.GetSRTrainingModuleByID(ctx, db.GetSRTrainingModuleByIDParams{
		ID:    moduleID,
		OrgID: orgID,
	})
	if err != nil {
		return nil, err
	}
	m := trainingModuleFromRow(row)
	return &m, nil
}

// ── Assignments ───────────────────────────────────────────────────────────

func (r *Repository) UpsertAssignment(ctx context.Context, orgID, moduleID string, targetID *string, department string, dueDate time.Time) (*Assignment, error) {
	row, err := r.q.UpsertSRAssignment(ctx, db.UpsertSRAssignmentParams{
		OrgID:      orgID,
		ModuleID:   moduleID,
		DueDate:    pgtype.Timestamptz{Time: dueDate, Valid: true},
		TargetID:   optUUIDFromPtr(targetID),
		Department: optText(department),
	})
	if err != nil {
		return nil, fmt.Errorf("upsert assignment: %w", err)
	}
	a := assignmentFromRow(row)
	return &a, nil
}

func (r *Repository) GetAssignment(ctx context.Context, orgID, assignmentID string) (*Assignment, error) {
	row, err := r.q.GetSRAssignment(ctx, db.GetSRAssignmentParams{
		ID:    assignmentID,
		OrgID: orgID,
	})
	if err != nil {
		return nil, err
	}
	a := assignmentFromRow(row)
	return &a, nil
}

func (r *Repository) ListAssignments(ctx context.Context, orgID, status string) ([]Assignment, error) {
	var rows []db.SrAssignments
	var err error
	switch status {
	case "overdue":
		rows, err = r.q.ListSROverdueAssignments(ctx, orgID)
	case "completed":
		rows, err = r.q.ListSRCompletedAssignments(ctx, orgID)
	default:
		rows, err = r.q.ListSRAssignments(ctx, orgID)
	}
	if err != nil {
		return nil, fmt.Errorf("list assignments: %w", err)
	}
	out := make([]Assignment, 0, len(rows))
	for _, row := range rows {
		out = append(out, assignmentFromRow(row))
	}
	return out, nil
}

func (r *Repository) CreateCompletion(ctx context.Context, orgID, assignmentID string, score *int, passed bool) (*Completion, error) {
	row, err := r.q.UpsertSRCompletion(ctx, db.UpsertSRCompletionParams{
		OrgID:        orgID,
		AssignmentID: assignmentID,
		Passed:       passed,
		Score:        optInt4(score),
	})
	if err != nil {
		return nil, fmt.Errorf("upsert completion: %w", err)
	}
	c := completionFromRow(row)
	return &c, nil
}

// GetCompletionByAssignment returns the completion record for a given assignment, if one exists.
func (r *Repository) GetCompletionByAssignment(ctx context.Context, orgID, assignmentID string) (*Completion, error) {
	row, err := r.q.GetSRCompletionByAssignment(ctx, db.GetSRCompletionByAssignmentParams{
		AssignmentID: assignmentID,
		OrgID:        orgID,
	})
	if err != nil {
		return nil, err
	}
	c := completionFromRow(row)
	return &c, nil
}

// ── Phish-Button (Feature 5) ──────────────────────────────────────────────

// GetOrgByPhishToken returns the org ID for the given phish_report_token.
func (r *Repository) GetOrgByPhishToken(ctx context.Context, token string) (string, error) {
	return r.q.GetOrgByPhishReportToken(ctx, pgtype.Text{String: token, Valid: true})
}

// findActiveCampaignForReporter checks whether the reporter_email appears in any
// active campaign's target group for the given org. Returns the campaign ID if found.
func (r *Repository) findActiveCampaignForReporter(ctx context.Context, orgID, reporterEmail string) (*string, error) {
	id, err := r.q.FindActiveSRCampaignForReporter(ctx, db.FindActiveSRCampaignForReporterParams{
		OrgID:         orgID,
		ReporterEmail: reporterEmail,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil //nolint:nilerr // no match is not an error
		}
		return nil, nil //nolint:nilerr // preserve historical behaviour: errors swallow to nil
	}
	return &id, nil
}

// CreatePhishReport inserts a new phishing report and returns the created record.
func (r *Repository) CreatePhishReport(ctx context.Context, orgID string, campaignID *string, in PhishReportWebhookInput, isSimulation bool) (*PhishReport, error) {
	row, err := r.q.CreateSRPhishReport(ctx, db.CreateSRPhishReportParams{
		OrgID:         orgID,
		CampaignID:    optUUIDFromPtr(campaignID),
		ReporterEmail: in.ReporterEmail,
		Subject:       optText(in.Subject),
		Sender:        optText(in.Sender),
		IsSimulation:  isSimulation,
	})
	if err != nil {
		return nil, fmt.Errorf("create phish report: %w", err)
	}
	rpt := phishReportFromRow(row)
	return &rpt, nil
}

// ListPhishReports returns all phishing reports for the org, newest first.
func (r *Repository) ListPhishReports(ctx context.Context, orgID string) ([]PhishReport, error) {
	rows, err := r.q.ListSRPhishReports(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list phish reports: %w", err)
	}
	out := make([]PhishReport, 0, len(rows))
	for _, row := range rows {
		out = append(out, phishReportFromRow(row))
	}
	return out, nil
}

// GetPhishReportStats returns aggregate stats for an org's phishing reports.
func (r *Repository) GetPhishReportStats(ctx context.Context, orgID string) (*PhishReportStats, error) {
	row, err := r.q.GetSRPhishReportStats(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("phish report stats: %w", err)
	}
	return &PhishReportStats{
		Total:       int(row.Total),
		Simulations: int(row.Simulations),
		RealThreats: int(row.RealThreats),
	}, nil
}

// SetPhishReportToken stores a new phish_report_token on the organization.
func (r *Repository) SetPhishReportToken(ctx context.Context, orgID, token string) error {
	return r.q.SetOrgPhishReportToken(ctx, db.SetOrgPhishReportTokenParams{
		PhishReportToken: pgtype.Text{String: token, Valid: true},
		ID:               orgID,
	})
}
