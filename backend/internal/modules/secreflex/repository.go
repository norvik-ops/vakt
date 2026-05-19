package secreflex

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles PhishGuard data access.
type Repository struct{ db *pgxpool.Pool }

// NewRepository creates a new PhishGuard repository.
func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

// ── Templates ─────────────────────────────────────────────────────────────────

func (r *Repository) CreateTemplate(ctx context.Context, orgID, userID string, input CreateTemplateInput) (*Template, error) {
	const q = `INSERT INTO pg_templates (org_id, name, subject, from_name, from_email, html_body, attack_type, created_by)
	           VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	           RETURNING id, org_id, name, subject, from_name, from_email, html_body, attack_type, is_preset, created_by, created_at`
	var t Template
	err := r.db.QueryRow(ctx, q, orgID, input.Name, input.Subject, input.FromName, input.FromEmail, input.HTMLBody, input.AttackType, userID).
		Scan(&t.ID, &t.OrgID, &t.Name, &t.Subject, &t.FromName, &t.FromEmail, &t.HTMLBody, &t.AttackType, &t.IsPreset, &t.CreatedBy, &t.CreatedAt)
	return &t, err
}

func (r *Repository) ListTemplates(ctx context.Context, orgID string) ([]Template, error) {
	const q = `SELECT id, org_id, name, subject, from_name, from_email, html_body, attack_type, is_preset, created_by, created_at
	           FROM pg_templates WHERE org_id=$1 ORDER BY created_at DESC LIMIT 500`
	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Template
	for rows.Next() {
		var t Template
		if err := rows.Scan(&t.ID, &t.OrgID, &t.Name, &t.Subject, &t.FromName, &t.FromEmail, &t.HTMLBody, &t.AttackType, &t.IsPreset, &t.CreatedBy, &t.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

// ── Target groups ─────────────────────────────────────────────────────────────

func (r *Repository) CreateTargetGroup(ctx context.Context, orgID, name, source string) (*TargetGroup, error) {
	const q = `INSERT INTO pg_target_groups (org_id, name, source) VALUES ($1,$2,$3)
	           RETURNING id, org_id, name, source, ad_ou, created_at`
	var g TargetGroup
	err := r.db.QueryRow(ctx, q, orgID, name, source).Scan(&g.ID, &g.OrgID, &g.Name, &g.Source, &g.ADOU, &g.CreatedAt)
	return &g, err
}

func (r *Repository) ListTargetGroups(ctx context.Context, orgID string) ([]TargetGroup, error) {
	const q = `SELECT id, org_id, name, source, ad_ou, created_at FROM pg_target_groups WHERE org_id=$1 ORDER BY name LIMIT 500`
	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []TargetGroup
	for rows.Next() {
		var g TargetGroup
		if err := rows.Scan(&g.ID, &g.OrgID, &g.Name, &g.Source, &g.ADOU, &g.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, g)
	}
	return items, rows.Err()
}

func (r *Repository) CreateTarget(ctx context.Context, orgID, groupID, email, firstName, lastName, department string) (*Target, error) {
	const q = `INSERT INTO pg_targets (org_id, group_id, email, first_name, last_name, department) VALUES ($1,$2,$3,$4,$5,$6)
	           ON CONFLICT (group_id, email) DO UPDATE SET first_name=EXCLUDED.first_name, last_name=EXCLUDED.last_name, department=EXCLUDED.department
	           RETURNING id, org_id, group_id, email, first_name, last_name, department, is_bounced, created_at`
	var t Target
	err := r.db.QueryRow(ctx, q, orgID, groupID, email, firstName, lastName, department).
		Scan(&t.ID, &t.OrgID, &t.GroupID, &t.Email, &t.FirstName, &t.LastName, &t.Department, &t.IsBounced, &t.CreatedAt)
	return &t, err
}

func (r *Repository) ListTargets(ctx context.Context, orgID, groupID string) ([]Target, error) {
	const q = `SELECT id, org_id, group_id, email, first_name, last_name, department, is_bounced, created_at
	           FROM pg_targets WHERE group_id=$1 AND org_id=$2 ORDER BY email LIMIT 500`
	rows, err := r.db.Query(ctx, q, groupID, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Target
	for rows.Next() {
		var t Target
		if err := rows.Scan(&t.ID, &t.OrgID, &t.GroupID, &t.Email, &t.FirstName, &t.LastName, &t.Department, &t.IsBounced, &t.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

func (r *Repository) CountTargetsInGroup(ctx context.Context, groupID string) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM pg_targets WHERE group_id=$1`, groupID).Scan(&n)
	return n, err
}

// ── Landing pages ─────────────────────────────────────────────────────────────

func (r *Repository) CreateLandingPage(ctx context.Context, orgID, name, html string) (*LandingPage, error) {
	const q = `INSERT INTO pg_landing_pages (org_id, name, html_content) VALUES ($1,$2,$3)
	           RETURNING id, org_id, name, html_content, created_at`
	var p LandingPage
	err := r.db.QueryRow(ctx, q, orgID, name, html).Scan(&p.ID, &p.OrgID, &p.Name, &p.HTMLContent, &p.CreatedAt)
	return &p, err
}

func (r *Repository) ListLandingPages(ctx context.Context, orgID string) ([]LandingPage, error) {
	const q = `SELECT id, org_id, name, html_content, created_at FROM pg_landing_pages WHERE org_id=$1 ORDER BY name LIMIT 500`
	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []LandingPage
	for rows.Next() {
		var p LandingPage
		if err := rows.Scan(&p.ID, &p.OrgID, &p.Name, &p.HTMLContent, &p.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

// ── Campaigns ─────────────────────────────────────────────────────────────────

func (r *Repository) CreateCampaign(ctx context.Context, orgID, userID string, input CreateCampaignInput) (*Campaign, error) {
	const q = `INSERT INTO pg_campaigns (org_id, name, template_id, group_id, landing_page_id, from_name, from_email, subject, scheduled_at, recurrence, track_opens, betriebsrat_mode, created_by)
	           VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	           RETURNING id, org_id, name, status, template_id, group_id, landing_page_id, from_name, from_email, subject, scheduled_at, started_at, completed_at, recurrence, track_opens, betriebsrat_mode, created_by, created_at, updated_at`
	var c Campaign
	err := r.db.QueryRow(ctx, q, orgID, input.Name, input.TemplateID, input.GroupID, input.LandingPageID,
		input.FromName, input.FromEmail, input.Subject, input.ScheduledAt, input.Recurrence, input.TrackOpens, input.BetriebsratMode, userID).
		Scan(&c.ID, &c.OrgID, &c.Name, &c.Status, &c.TemplateID, &c.GroupID, &c.LandingPageID, &c.FromName, &c.FromEmail, &c.Subject,
			&c.ScheduledAt, &c.StartedAt, &c.CompletedAt, &c.Recurrence, &c.TrackOpens, &c.BetriebsratMode, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
	return &c, err
}

func (r *Repository) GetCampaign(ctx context.Context, orgID, campaignID string) (*Campaign, error) {
	const q = `SELECT id, org_id, name, status, template_id, group_id, landing_page_id, from_name, from_email, subject,
	           scheduled_at, started_at, completed_at, recurrence, track_opens, betriebsrat_mode, created_by, created_at, updated_at
	           FROM pg_campaigns WHERE id=$1 AND org_id=$2`
	var c Campaign
	err := r.db.QueryRow(ctx, q, campaignID, orgID).
		Scan(&c.ID, &c.OrgID, &c.Name, &c.Status, &c.TemplateID, &c.GroupID, &c.LandingPageID, &c.FromName, &c.FromEmail, &c.Subject,
			&c.ScheduledAt, &c.StartedAt, &c.CompletedAt, &c.Recurrence, &c.TrackOpens, &c.BetriebsratMode, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *Repository) ListCampaigns(ctx context.Context, orgID string) ([]Campaign, error) {
	const q = `SELECT id, org_id, name, status, template_id, group_id, landing_page_id, from_name, from_email, subject,
	           scheduled_at, started_at, completed_at, recurrence, track_opens, betriebsrat_mode, created_by, created_at, updated_at
	           FROM pg_campaigns WHERE org_id=$1 ORDER BY created_at DESC LIMIT 500`
	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Campaign
	for rows.Next() {
		var c Campaign
		if err := rows.Scan(&c.ID, &c.OrgID, &c.Name, &c.Status, &c.TemplateID, &c.GroupID, &c.LandingPageID, &c.FromName, &c.FromEmail, &c.Subject,
			&c.ScheduledAt, &c.StartedAt, &c.CompletedAt, &c.Recurrence, &c.TrackOpens, &c.BetriebsratMode, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (r *Repository) UpdateCampaignStatus(ctx context.Context, orgID, campaignID, status string) error {
	_, err := r.db.Exec(ctx, `UPDATE pg_campaigns SET status=$1, updated_at=NOW() WHERE id=$2 AND org_id=$3`, status, campaignID, orgID)
	return err
}

func (r *Repository) GetCampaignStats(ctx context.Context, orgID, campaignID string) (*CampaignStats, error) {
	var stats CampaignStats
	// Count targets from campaign's group
	var groupID *string
	_ = r.db.QueryRow(ctx, `SELECT group_id FROM pg_campaigns WHERE id=$1`, campaignID).Scan(&groupID)
	if groupID != nil {
		_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM pg_targets WHERE group_id=$1`, *groupID).Scan(&stats.TotalTargets)
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
	var n int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM pg_events WHERE campaign_id=$1 AND type=$2`, campaignID, eventType).Scan(&n)
	return n, err
}

// ── Tracking events ───────────────────────────────────────────────────────────

func (r *Repository) GetCampaignByTrackingToken(ctx context.Context, token string) (*Campaign, error) {
	const q = `SELECT c.id, c.org_id, c.name, c.status, c.template_id, c.group_id, c.landing_page_id,
	           c.from_name, c.from_email, c.subject, c.scheduled_at, c.started_at, c.completed_at,
	           c.recurrence, c.track_opens, c.betriebsrat_mode, c.created_by, c.created_at, c.updated_at
	           FROM pg_campaigns c JOIN pg_events e ON e.campaign_id=c.id WHERE e.tracking_token=$1 LIMIT 1`
	var c Campaign
	err := r.db.QueryRow(ctx, q, token).
		Scan(&c.ID, &c.OrgID, &c.Name, &c.Status, &c.TemplateID, &c.GroupID, &c.LandingPageID, &c.FromName, &c.FromEmail, &c.Subject,
			&c.ScheduledAt, &c.StartedAt, &c.CompletedAt, &c.Recurrence, &c.TrackOpens, &c.BetriebsratMode, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *Repository) CreateTrackingEvent(ctx context.Context, orgID, campaignID string, targetID *string, department, token, eventType, ip, ua string) error {
	const q = `INSERT INTO pg_events (org_id, campaign_id, target_id, department, type, tracking_token, ip_address, user_agent) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`
	_, err := r.db.Exec(ctx, q, orgID, campaignID, targetID, department, eventType, token, ip, ua)
	return err
}

func (r *Repository) GetLandingPageForCampaign(ctx context.Context, campaignID string) (*LandingPage, error) {
	const q = `SELECT lp.id, lp.org_id, lp.name, lp.html_content, lp.created_at
	           FROM pg_landing_pages lp JOIN pg_campaigns c ON c.landing_page_id=lp.id WHERE c.id=$1`
	var p LandingPage
	err := r.db.QueryRow(ctx, q, campaignID).Scan(&p.ID, &p.OrgID, &p.Name, &p.HTMLContent, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetTemplate returns a template by ID within the org.
func (r *Repository) GetTemplate(ctx context.Context, orgID, templateID string) (*Template, error) {
	const q = `SELECT id, org_id, name, subject, from_name, from_email, html_body, attack_type, is_preset, created_by, created_at
	           FROM pg_templates WHERE id=$1 AND (org_id=$2 OR is_preset=true)`
	var t Template
	err := r.db.QueryRow(ctx, q, templateID, orgID).
		Scan(&t.ID, &t.OrgID, &t.Name, &t.Subject, &t.FromName, &t.FromEmail, &t.HTMLBody, &t.AttackType, &t.IsPreset, &t.CreatedBy, &t.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get template: %w", err)
	}
	return &t, nil
}

// SetCampaignCompleted marks a campaign as completed and sets completed_at.
func (r *Repository) SetCampaignCompleted(ctx context.Context, orgID, campaignID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE pg_campaigns SET status='completed', completed_at=NOW(), updated_at=NOW() WHERE id=$1 AND org_id=$2`,
		campaignID, orgID,
	)
	return err
}

// ── Training modules ──────────────────────────────────────────────────────────

func (r *Repository) CreateModule(ctx context.Context, orgID, userID string, input CreateModuleInput) (*TrainingModule, error) {
	questionsJSON, _ := json.Marshal(input.Questions)
	passingScore := input.PassingScore
	if passingScore == 0 {
		passingScore = 80
	}
	const q = `INSERT INTO pg_training_modules (org_id, title, type, attack_type, content_url, duration_seconds, passing_score, questions, created_by)
	           VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	           RETURNING id, org_id, title, type, attack_type, content_url, duration_seconds, passing_score, questions, created_by, created_at`
	var m TrainingModule
	var questionsB []byte
	err := r.db.QueryRow(ctx, q, orgID, input.Title, input.Type, input.AttackType, input.ContentURL, input.DurationSeconds, passingScore, questionsJSON, userID).
		Scan(&m.ID, &m.OrgID, &m.Title, &m.Type, &m.AttackType, &m.ContentURL, &m.DurationSeconds, &m.PassingScore, &questionsB, &m.CreatedBy, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(questionsB, &m.Questions)
	return &m, nil
}

func (r *Repository) ListModules(ctx context.Context, orgID string) ([]TrainingModule, error) {
	const q = `SELECT id, org_id, title, type, attack_type, content_url, duration_seconds, passing_score, questions, created_by, created_at
	           FROM pg_training_modules WHERE org_id=$1 ORDER BY title LIMIT 500`
	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []TrainingModule
	for rows.Next() {
		var m TrainingModule
		var questionsB []byte
		if err := rows.Scan(&m.ID, &m.OrgID, &m.Title, &m.Type, &m.AttackType, &m.ContentURL, &m.DurationSeconds, &m.PassingScore, &questionsB, &m.CreatedBy, &m.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(questionsB, &m.Questions)
		items = append(items, m)
	}
	return items, rows.Err()
}

func (r *Repository) GetModuleByAttackType(ctx context.Context, orgID, attackType string) (*TrainingModule, error) {
	const q = `SELECT id, org_id, title, type, attack_type, content_url, duration_seconds, passing_score, questions, created_by, created_at
	           FROM pg_training_modules WHERE org_id=$1 AND attack_type=$2 LIMIT 1`
	var m TrainingModule
	var questionsB []byte
	err := r.db.QueryRow(ctx, q, orgID, attackType).Scan(&m.ID, &m.OrgID, &m.Title, &m.Type, &m.AttackType, &m.ContentURL, &m.DurationSeconds, &m.PassingScore, &questionsB, &m.CreatedBy, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(questionsB, &m.Questions)
	return &m, nil
}

// ── Assignments ───────────────────────────────────────────────────────────────

func (r *Repository) UpsertAssignment(ctx context.Context, orgID, moduleID string, targetID *string, department string, dueDate time.Time) (*Assignment, error) {
	const q = `INSERT INTO pg_assignments (org_id, module_id, target_id, department, due_date)
	           VALUES ($1,$2,$3,$4,$5)
	           ON CONFLICT (module_id, target_id) DO UPDATE SET due_date=GREATEST(EXCLUDED.due_date, pg_assignments.due_date)
	           RETURNING id, org_id, module_id, target_id, department, due_date, is_overdue, created_at`
	var a Assignment
	err := r.db.QueryRow(ctx, q, orgID, moduleID, targetID, department, dueDate).
		Scan(&a.ID, &a.OrgID, &a.ModuleID, &a.TargetID, &a.Department, &a.DueDate, &a.IsOverdue, &a.CreatedAt)
	return &a, err
}

func (r *Repository) GetAssignment(ctx context.Context, orgID, assignmentID string) (*Assignment, error) {
	const q = `SELECT id, org_id, module_id, target_id, department, due_date, is_overdue, created_at
	           FROM pg_assignments WHERE id=$1 AND org_id=$2`
	var a Assignment
	err := r.db.QueryRow(ctx, q, assignmentID, orgID).Scan(&a.ID, &a.OrgID, &a.ModuleID, &a.TargetID, &a.Department, &a.DueDate, &a.IsOverdue, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *Repository) ListAssignments(ctx context.Context, orgID, status string) ([]Assignment, error) {
	q := `SELECT id, org_id, module_id, target_id, department, due_date, is_overdue, created_at FROM pg_assignments WHERE org_id=$1`
	args := []interface{}{orgID}
	if status == "overdue" {
		q += ` AND is_overdue=true`
	} else if status == "completed" {
		q += ` AND id IN (SELECT assignment_id FROM pg_completions)`
	}
	q += ` ORDER BY due_date LIMIT 500`
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Assignment
	for rows.Next() {
		var a Assignment
		if err := rows.Scan(&a.ID, &a.OrgID, &a.ModuleID, &a.TargetID, &a.Department, &a.DueDate, &a.IsOverdue, &a.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, rows.Err()
}

func (r *Repository) CreateCompletion(ctx context.Context, orgID, assignmentID string, score *int, passed bool) (*Completion, error) {
	const q = `INSERT INTO pg_completions (org_id, assignment_id, score, passed) VALUES ($1,$2,$3,$4)
	           ON CONFLICT (assignment_id) DO UPDATE SET score=EXCLUDED.score, passed=EXCLUDED.passed, completed_at=NOW()
	           RETURNING id, org_id, assignment_id, score, passed, completed_at`
	var c Completion
	err := r.db.QueryRow(ctx, q, orgID, assignmentID, score, passed).Scan(&c.ID, &c.OrgID, &c.AssignmentID, &c.Score, &c.Passed, &c.CompletedAt)
	return &c, err
}

// GetCompletionByAssignment returns the completion record for a given assignment, if one exists.
func (r *Repository) GetCompletionByAssignment(ctx context.Context, orgID, assignmentID string) (*Completion, error) {
	const q = `SELECT id, org_id, assignment_id, score, passed, completed_at
	           FROM pg_completions WHERE assignment_id=$1 AND org_id=$2 LIMIT 1`
	var c Completion
	err := r.db.QueryRow(ctx, q, assignmentID, orgID).Scan(&c.ID, &c.OrgID, &c.AssignmentID, &c.Score, &c.Passed, &c.CompletedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// GetModuleByID returns a training module by its ID within the org.
func (r *Repository) GetModuleByID(ctx context.Context, orgID, moduleID string) (*TrainingModule, error) {
	const q = `SELECT id, org_id, title, type, attack_type, content_url, duration_seconds, passing_score, questions, created_by, created_at
	           FROM pg_training_modules WHERE id=$1 AND org_id=$2`
	var m TrainingModule
	var questionsB []byte
	err := r.db.QueryRow(ctx, q, moduleID, orgID).
		Scan(&m.ID, &m.OrgID, &m.Title, &m.Type, &m.AttackType, &m.ContentURL, &m.DurationSeconds, &m.PassingScore, &questionsB, &m.CreatedBy, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(questionsB, &m.Questions)
	return &m, nil
}

// ── Phish-Button (Feature 5) ──────────────────────────────────────────────────

// GetOrgByPhishToken returns the org ID for the given phish_report_token.
func (r *Repository) GetOrgByPhishToken(ctx context.Context, token string) (string, error) {
	var orgID string
	err := r.db.QueryRow(ctx, `SELECT id FROM organizations WHERE phish_report_token=$1`, token).Scan(&orgID)
	return orgID, err
}

// findActiveCampaignForReporter checks whether the reporter_email appears in any
// active campaign's target group for the given org. Returns the campaign ID if found.
func (r *Repository) findActiveCampaignForReporter(ctx context.Context, orgID, reporterEmail string) (*string, error) {
	const q = `SELECT c.id FROM pg_campaigns c
	           JOIN pg_target_groups tg ON tg.id = c.group_id
	           JOIN pg_targets t ON t.group_id = tg.id AND t.org_id = c.org_id
	           WHERE c.org_id = $1
	             AND c.status = 'running'
	             AND lower(t.email) = lower($2)
	           LIMIT 1`
	var id string
	err := r.db.QueryRow(ctx, q, orgID, reporterEmail).Scan(&id)
	if err != nil {
		return nil, nil //nolint:nilerr // no match is not an error
	}
	return &id, nil
}

// CreatePhishReport inserts a new phishing report and returns the created record.
func (r *Repository) CreatePhishReport(ctx context.Context, orgID string, campaignID *string, in PhishReportWebhookInput, isSimulation bool) (*PhishReport, error) {
	const q = `INSERT INTO pg_phish_reports (org_id, campaign_id, reporter_email, subject, sender, is_simulation)
	           VALUES ($1, $2, $3, $4, $5, $6)
	           RETURNING id, org_id, campaign_id, reporter_email, reported_at, subject, sender, is_simulation, created_at`
	var rpt PhishReport
	err := r.db.QueryRow(ctx, q, orgID, campaignID, in.ReporterEmail, in.Subject, in.Sender, isSimulation).
		Scan(&rpt.ID, &rpt.OrgID, &rpt.CampaignID, &rpt.ReporterEmail, &rpt.ReportedAt, &rpt.Subject, &rpt.Sender, &rpt.IsSimulation, &rpt.CreatedAt)
	return &rpt, err
}

// ListPhishReports returns all phishing reports for the org, newest first.
func (r *Repository) ListPhishReports(ctx context.Context, orgID string) ([]PhishReport, error) {
	const q = `SELECT id, org_id, campaign_id, reporter_email, reported_at, subject, sender, is_simulation, created_at
	           FROM pg_phish_reports WHERE org_id=$1 ORDER BY reported_at DESC LIMIT 500`
	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []PhishReport
	for rows.Next() {
		var rpt PhishReport
		if err := rows.Scan(&rpt.ID, &rpt.OrgID, &rpt.CampaignID, &rpt.ReporterEmail, &rpt.ReportedAt, &rpt.Subject, &rpt.Sender, &rpt.IsSimulation, &rpt.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, rpt)
	}
	return items, rows.Err()
}

// GetPhishReportStats returns aggregate stats for an org's phishing reports.
func (r *Repository) GetPhishReportStats(ctx context.Context, orgID string) (*PhishReportStats, error) {
	const q = `SELECT
	             COUNT(*)                                    AS total,
	             COUNT(*) FILTER (WHERE is_simulation=true)  AS simulations,
	             COUNT(*) FILTER (WHERE is_simulation=false) AS real_threats
	           FROM pg_phish_reports WHERE org_id=$1`
	var s PhishReportStats
	err := r.db.QueryRow(ctx, q, orgID).Scan(&s.Total, &s.Simulations, &s.RealThreats)
	return &s, err
}

// SetPhishReportToken stores a new phish_report_token on the organization.
func (r *Repository) SetPhishReportToken(ctx context.Context, orgID, token string) error {
	_, err := r.db.Exec(ctx, `UPDATE organizations SET phish_report_token=$1 WHERE id=$2`, token, orgID)
	return err
}
