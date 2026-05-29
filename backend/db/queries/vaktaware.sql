-- Vakt Aware (Phishing-Simulation & Awareness-Training)
--
-- Tabellen-Präfix `sr_*` seit Migration 122 (vorher `pg_*` — kollidierte
-- mit dem PostgreSQL-System-Katalog-Namespace und blockierte den
-- sqlc-Parser, siehe ADR-0005).

-- ── Templates ─────────────────────────────────────────────────────────────

-- name: CreateSRTemplate :one
INSERT INTO sr_templates
       (org_id, name, subject, from_name, from_email, html_body, attack_type, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, sqlc.narg('created_by')::uuid)
RETURNING id, org_id, name, subject, from_name, from_email, html_body,
          attack_type, is_preset, created_by, created_at;

-- name: ListSRTemplates :many
SELECT id, org_id, name, subject, from_name, from_email, html_body,
       attack_type, is_preset, created_by, created_at
FROM sr_templates
WHERE org_id = $1
ORDER BY created_at DESC
LIMIT 500;

-- name: GetSRTemplate :one
SELECT id, org_id, name, subject, from_name, from_email, html_body,
       attack_type, is_preset, created_by, created_at
FROM sr_templates
WHERE id = $1 AND (org_id = $2 OR is_preset = TRUE);

-- ── Target groups ─────────────────────────────────────────────────────────

-- name: CreateSRTargetGroup :one
INSERT INTO sr_target_groups (org_id, name, source)
VALUES ($1, $2, $3)
RETURNING id, org_id, name, source, ad_ou, created_at;

-- name: ListSRTargetGroups :many
SELECT id, org_id, name, source, ad_ou, created_at
FROM sr_target_groups
WHERE org_id = $1
ORDER BY name
LIMIT 500;

-- ── Targets ───────────────────────────────────────────────────────────────

-- name: UpsertSRTarget :one
INSERT INTO sr_targets (org_id, group_id, email, first_name, last_name, department)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (group_id, email) DO UPDATE
   SET first_name = EXCLUDED.first_name,
       last_name  = EXCLUDED.last_name,
       department = EXCLUDED.department
RETURNING id, org_id, group_id, email, first_name, last_name, department,
          is_bounced, created_at;

-- name: ListSRTargets :many
SELECT id, org_id, group_id, email, first_name, last_name, department,
       is_bounced, created_at
FROM sr_targets
WHERE group_id = $1 AND org_id = $2
ORDER BY email
LIMIT 500;

-- name: CountSRTargetsInGroup :one
SELECT COUNT(*) FROM sr_targets WHERE group_id = $1;

-- ── Landing pages ─────────────────────────────────────────────────────────

-- name: CreateSRLandingPage :one
INSERT INTO sr_landing_pages (org_id, name, html_content)
VALUES ($1, $2, $3)
RETURNING id, org_id, name, html_content, created_at;

-- name: ListSRLandingPages :many
SELECT id, org_id, name, html_content, created_at
FROM sr_landing_pages
WHERE org_id = $1
ORDER BY name
LIMIT 500;

-- name: GetSRLandingPageForCampaign :one
SELECT lp.id, lp.org_id, lp.name, lp.html_content, lp.created_at
FROM sr_landing_pages lp
JOIN sr_campaigns c ON c.landing_page_id = lp.id
WHERE c.id = $1;

-- ── Campaigns ─────────────────────────────────────────────────────────────

-- name: CreateSRCampaign :one
INSERT INTO sr_campaigns
       (org_id, name, template_id, group_id, landing_page_id,
        from_name, from_email, subject, scheduled_at, recurrence,
        track_opens, betriebsrat_mode, created_by)
VALUES ($1, $2,
        sqlc.narg('template_id')::uuid,
        sqlc.narg('group_id')::uuid,
        sqlc.narg('landing_page_id')::uuid,
        $3, $4, $5,
        sqlc.narg('scheduled_at')::timestamptz,
        sqlc.narg('recurrence')::text,
        $6, $7,
        sqlc.narg('created_by')::uuid)
RETURNING id, org_id, name, status, template_id, group_id, landing_page_id,
          from_name, from_email, subject, scheduled_at, started_at,
          completed_at, recurrence, track_opens, betriebsrat_mode,
          created_by, created_at, updated_at;

-- name: GetSRCampaign :one
SELECT id, org_id, name, status, template_id, group_id, landing_page_id,
       from_name, from_email, subject, scheduled_at, started_at,
       completed_at, recurrence, track_opens, betriebsrat_mode,
       created_by, created_at, updated_at
FROM sr_campaigns
WHERE id = $1 AND org_id = $2;

-- name: ListSRCampaigns :many
SELECT id, org_id, name, status, template_id, group_id, landing_page_id,
       from_name, from_email, subject, scheduled_at, started_at,
       completed_at, recurrence, track_opens, betriebsrat_mode,
       created_by, created_at, updated_at
FROM sr_campaigns
WHERE org_id = $1
ORDER BY created_at DESC
LIMIT 500;

-- name: UpdateSRCampaignStatus :exec
UPDATE sr_campaigns
SET status = $1, updated_at = NOW()
WHERE id = $2 AND org_id = $3;

-- name: SetSRCampaignCompleted :exec
UPDATE sr_campaigns
SET status = 'completed', completed_at = NOW(), updated_at = NOW()
WHERE id = $1 AND org_id = $2;

-- name: GetSRCampaignGroupID :one
SELECT group_id FROM sr_campaigns WHERE id = $1;

-- name: GetSRCampaignByTrackingToken :one
SELECT c.id, c.org_id, c.name, c.status, c.template_id, c.group_id,
       c.landing_page_id, c.from_name, c.from_email, c.subject,
       c.scheduled_at, c.started_at, c.completed_at, c.recurrence,
       c.track_opens, c.betriebsrat_mode, c.created_by, c.created_at,
       c.updated_at
FROM sr_campaigns c
JOIN sr_events e ON e.campaign_id = c.id
WHERE e.tracking_token = $1
LIMIT 1;

-- name: FindActiveSRCampaignForReporter :one
SELECT c.id
FROM sr_campaigns c
JOIN sr_target_groups tg ON tg.id = c.group_id
JOIN sr_targets t ON t.group_id = tg.id AND t.org_id = c.org_id
WHERE c.org_id = $1
  AND c.status = 'running'
  AND lower(t.email) = lower(sqlc.arg('reporter_email')::text)
LIMIT 1;

-- ── Tracking events ───────────────────────────────────────────────────────

-- name: CountSREventsByType :one
SELECT COUNT(*) FROM sr_events
WHERE campaign_id = $1 AND type = $2;

-- name: CreateSRTrackingEvent :exec
INSERT INTO sr_events
       (org_id, campaign_id, target_id, department, type,
        tracking_token, ip_address, user_agent)
VALUES ($1, $2,
        sqlc.narg('target_id')::uuid,
        sqlc.narg('department')::text,
        $3, $4,
        sqlc.narg('ip_address')::text,
        sqlc.narg('user_agent')::text);

-- ── Training modules ──────────────────────────────────────────────────────

-- name: CreateSRTrainingModule :one
INSERT INTO sr_training_modules
       (org_id, title, type, attack_type, content_url,
        duration_seconds, passing_score, questions, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, sqlc.narg('created_by')::uuid)
RETURNING id, org_id, title, type, attack_type, content_url,
          duration_seconds, passing_score, questions, created_by, created_at;

-- name: ListSRTrainingModules :many
SELECT id, org_id, title, type, attack_type, content_url,
       duration_seconds, passing_score, questions, created_by, created_at
FROM sr_training_modules
WHERE org_id = $1
ORDER BY title
LIMIT 500;

-- name: GetSRTrainingModuleByAttackType :one
SELECT id, org_id, title, type, attack_type, content_url,
       duration_seconds, passing_score, questions, created_by, created_at
FROM sr_training_modules
WHERE org_id = $1 AND attack_type = $2
LIMIT 1;

-- name: GetSRTrainingModuleByID :one
SELECT id, org_id, title, type, attack_type, content_url,
       duration_seconds, passing_score, questions, created_by, created_at
FROM sr_training_modules
WHERE id = $1 AND org_id = $2;

-- ── Assignments ───────────────────────────────────────────────────────────

-- name: UpsertSRAssignment :one
INSERT INTO sr_assignments (org_id, module_id, target_id, department, due_date)
VALUES ($1, $2,
        sqlc.narg('target_id')::uuid,
        sqlc.narg('department')::text,
        $3)
ON CONFLICT (module_id, target_id) DO UPDATE
   SET due_date = GREATEST(EXCLUDED.due_date, sr_assignments.due_date)
RETURNING id, org_id, module_id, target_id, department, due_date,
          is_overdue, created_at;

-- name: GetSRAssignment :one
SELECT id, org_id, module_id, target_id, department, due_date,
       is_overdue, created_at
FROM sr_assignments
WHERE id = $1 AND org_id = $2;

-- name: ListSRAssignments :many
SELECT id, org_id, module_id, target_id, department, due_date,
       is_overdue, created_at
FROM sr_assignments
WHERE org_id = $1
ORDER BY due_date
LIMIT 500;

-- name: ListSROverdueAssignments :many
SELECT id, org_id, module_id, target_id, department, due_date,
       is_overdue, created_at
FROM sr_assignments
WHERE org_id = $1 AND is_overdue = TRUE
ORDER BY due_date
LIMIT 500;

-- name: ListSRCompletedAssignments :many
SELECT a.id, a.org_id, a.module_id, a.target_id, a.department, a.due_date,
       a.is_overdue, a.created_at
FROM sr_assignments a
WHERE a.org_id = $1
  AND a.id IN (SELECT assignment_id FROM sr_completions)
ORDER BY a.due_date
LIMIT 500;

-- name: UpsertSRCompletion :one
INSERT INTO sr_completions (org_id, assignment_id, score, passed)
VALUES ($1, $2, sqlc.narg('score')::int, $3)
ON CONFLICT (assignment_id) DO UPDATE
   SET score        = EXCLUDED.score,
       passed       = EXCLUDED.passed,
       completed_at = NOW()
RETURNING id, org_id, assignment_id, score, passed, completed_at;

-- name: GetSRCompletionByAssignment :one
SELECT id, org_id, assignment_id, score, passed, completed_at
FROM sr_completions
WHERE assignment_id = $1 AND org_id = $2
LIMIT 1;

-- ── Phish-Button (Feature 5) ──────────────────────────────────────────────

-- name: GetOrgByPhishReportToken :one
SELECT id FROM organizations WHERE phish_report_token = $1;

-- name: CreateSRPhishReport :one
INSERT INTO sr_phish_reports
       (org_id, campaign_id, reporter_email, subject, sender, is_simulation)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, org_id, campaign_id, reporter_email, reported_at,
          subject, sender, is_simulation, created_at;

-- name: ListSRPhishReports :many
SELECT id, org_id, campaign_id, reporter_email, reported_at,
       subject, sender, is_simulation, created_at
FROM sr_phish_reports
WHERE org_id = $1
ORDER BY reported_at DESC
LIMIT 500;

-- name: GetSRPhishReportStats :one
SELECT
    COUNT(*)::bigint                                      AS total,
    COUNT(*) FILTER (WHERE is_simulation = TRUE)::bigint  AS simulations,
    COUNT(*) FILTER (WHERE is_simulation = FALSE)::bigint AS real_threats
FROM sr_phish_reports
WHERE org_id = $1;

-- name: SetOrgPhishReportToken :exec
UPDATE organizations SET phish_report_token = $1 WHERE id = $2;

-- name: GetSROrganizationName :one
SELECT name FROM organizations WHERE id = $1::uuid;

-- name: GetSRTargetEmail :one
SELECT email FROM sr_targets WHERE id = $1;
