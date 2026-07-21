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

-- HR-Offboarding: alle drei Queries waren born-broken (users.org_id, users.status und
-- sessions.revoked_at existieren nicht — 42703 bei jedem Aufruf, Offboarding entzog also
-- NIE Zugriff). users↔org laeuft ueber org_members; refresh_sessions und api_keys tragen
-- org_id direkt. Alle drei jetzt org-scoped ueber die E-Mail des Mitarbeiters.

-- name: HRRevokeUserSessions :exec
DELETE FROM refresh_sessions rs
USING users u
WHERE rs.user_id = u.id
  AND rs.org_id  = $1::uuid
  AND u.email    = $2;

-- name: HRDisableUser :exec
-- Offboarding aus EINER Org entzieht die Mitgliedschaft in dieser Org (Multi-Org: der
-- globale Account und andere Orgs bleiben unberuehrt) — das RBAC-System liest org_members.
DELETE FROM org_members om
USING users u
WHERE om.user_id = u.id
  AND om.org_id  = $1::uuid
  AND u.email    = $2;

-- name: HRRevokeUserAPIKeys :exec
UPDATE api_keys ak SET revoked_at = NOW()
FROM users u
WHERE ak.created_by = u.id
  AND ak.org_id     = $1::uuid
  AND u.email       = $2
  AND ak.revoked_at IS NULL;


-- ── Berechtigungskonzept (S60) ────────────────────────────────────────────────

-- name: CreateHRAccessConcept :one
INSERT INTO hr_access_concepts (org_id, title, scope, owner)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListHRAccessConcepts :many
SELECT * FROM hr_access_concepts
WHERE org_id = $1
ORDER BY created_at DESC;

-- name: GetHRAccessConcept :one
SELECT * FROM hr_access_concepts
WHERE id = $1 AND org_id = $2;

-- name: UpdateHRAccessConcept :one
UPDATE hr_access_concepts
SET title = $3, scope = $4, owner = $5, updated_at = NOW()
WHERE id = $1 AND org_id = $2
RETURNING *;

-- name: DeleteHRAccessConcept :execrows
DELETE FROM hr_access_concepts
WHERE id = $1 AND org_id = $2;

-- name: IncrementHRAccessConceptVersion :one
UPDATE hr_access_concepts
SET current_version = current_version + 1, updated_at = NOW()
WHERE id = $1 AND org_id = $2
RETURNING current_version;

-- name: AddHRAccessRole :one
INSERT INTO hr_access_roles (concept_id, org_id, role_name, system_name, access_level, justification, review_interval_months)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListHRAccessRoles :many
SELECT * FROM hr_access_roles
WHERE concept_id = $1 AND org_id = $2
ORDER BY role_name, system_name;

-- name: UpdateHRAccessRole :one
UPDATE hr_access_roles
SET role_name = $3, system_name = $4, access_level = $5, justification = $6, review_interval_months = $7, updated_at = NOW()
WHERE id = $1 AND org_id = $2
RETURNING *;

-- name: DeleteHRAccessRole :execrows
DELETE FROM hr_access_roles
WHERE id = $1 AND org_id = $2;

-- name: InsertHRAccessConceptVersion :one
INSERT INTO hr_access_concept_versions (concept_id, org_id, version_number, snapshot)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListHRAccessConceptVersions :many
SELECT id, concept_id, org_id, version_number, created_at
FROM hr_access_concept_versions
WHERE concept_id = $1 AND org_id = $2
ORDER BY version_number DESC;

-- name: GetHRAccessConceptVersion :one
SELECT * FROM hr_access_concept_versions
WHERE concept_id = $1 AND org_id = $2 AND version_number = $3;
