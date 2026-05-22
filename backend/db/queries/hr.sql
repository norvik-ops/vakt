-- name: ListHREmployees :many
SELECT id, org_id, first_name, last_name, email, department, role,
       start_date, end_date, status, notes, created_at, updated_at
FROM hr_employees
WHERE org_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountHREmployees :one
SELECT COUNT(*) FROM hr_employees WHERE org_id = $1;

-- name: GetHREmployee :one
SELECT id, org_id, first_name, last_name, email, department, role,
       start_date, end_date, status, notes, created_at, updated_at
FROM hr_employees
WHERE org_id = $1 AND id = $2;

-- name: GetHREmployeeByEmail :one
SELECT id, org_id, first_name, last_name, email, department, role,
       start_date, end_date, status, notes, created_at, updated_at
FROM hr_employees
WHERE org_id = $1 AND email = $2;

-- name: CreateHREmployee :one
INSERT INTO hr_employees (org_id, first_name, last_name, email, department, role, start_date, notes)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, org_id, first_name, last_name, email, department, role,
          start_date, end_date, status, notes, created_at, updated_at;

-- name: UpdateHREmployee :one
UPDATE hr_employees
SET first_name = $3,
    last_name  = $4,
    department = $5,
    role       = $6,
    end_date   = $7,
    status     = $8,
    notes      = $9,
    updated_at = now()
WHERE org_id = $1 AND id = $2
RETURNING id, org_id, first_name, last_name, email, department, role,
          start_date, end_date, status, notes, created_at, updated_at;

-- name: SetHREmployeeStatus :exec
UPDATE hr_employees
SET status = $3, updated_at = now()
WHERE org_id = $1 AND id = $2;

-- name: DeleteHREmployee :exec
DELETE FROM hr_employees WHERE org_id = $1 AND id = $2;

-- name: ListHRChecklists :many
SELECT id, org_id, type, name, items, created_at, updated_at
FROM hr_checklists
WHERE org_id = $1
ORDER BY created_at DESC;

-- name: GetHRChecklist :one
SELECT id, org_id, type, name, items, created_at, updated_at
FROM hr_checklists
WHERE org_id = $1 AND id = $2;

-- name: CreateHRChecklist :one
INSERT INTO hr_checklists (org_id, type, name, items)
VALUES ($1, $2, $3, $4)
RETURNING id, org_id, type, name, items, created_at, updated_at;

-- name: DeleteHRChecklist :exec
DELETE FROM hr_checklists WHERE org_id = $1 AND id = $2;

-- name: FirstHRChecklistByType :one
SELECT id, org_id, type, name, items, created_at, updated_at
FROM hr_checklists
WHERE org_id = $1 AND type = $2
ORDER BY created_at ASC
LIMIT 1;

-- name: StartHRChecklistRun :one
INSERT INTO hr_checklist_runs (org_id, employee_id, checklist_id)
VALUES ($1, $2, $3)
RETURNING id, org_id, employee_id, checklist_id, status, completed_items,
          started_at, completed_at, created_at, updated_at;

-- name: GetHRChecklistRun :one
SELECT id, org_id, employee_id, checklist_id, status, completed_items,
       started_at, completed_at, created_at, updated_at
FROM hr_checklist_runs
WHERE org_id = $1 AND id = $2;

-- name: ListHRChecklistRuns :many
SELECT id, org_id, employee_id, checklist_id, status, completed_items,
       started_at, completed_at, created_at, updated_at
FROM hr_checklist_runs
WHERE org_id = $1 AND employee_id = $2
ORDER BY started_at DESC;

-- name: UpdateHRChecklistRun :one
UPDATE hr_checklist_runs
SET completed_items = $3,
    status          = $4,
    completed_at    = $5,
    updated_at      = now()
WHERE org_id = $1 AND id = $2
RETURNING id, org_id, employee_id, checklist_id, status, completed_items,
          started_at, completed_at, created_at, updated_at;

-- name: InsertHRRunEvent :exec
INSERT INTO hr_run_events (run_id, org_id, step_id, completed_by)
VALUES ($1, $2, $3, $4);

-- name: ListHRRunEvents :many
SELECT id, run_id, org_id, step_id, completed_by, completed_at
FROM hr_run_events
WHERE org_id = $1 AND run_id = $2
ORDER BY completed_at ASC;

-- name: HRRevokeUserSessions :exec
UPDATE sessions SET revoked_at = NOW()
FROM users
WHERE sessions.user_id = users.id
  AND users.org_id    = $1::uuid
  AND users.email     = $2
  AND sessions.revoked_at IS NULL;

-- name: HRDisableUser :exec
UPDATE users SET status = 'disabled'
WHERE org_id = $1::uuid AND email = $2;

-- name: HRRevokeUserAPIKeys :exec
UPDATE api_keys SET revoked_at = NOW()
FROM users
WHERE api_keys.created_by = users.id
  AND users.org_id        = $1::uuid
  AND users.email         = $2
  AND api_keys.revoked_at IS NULL;
