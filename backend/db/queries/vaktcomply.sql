-- Vakt Comply queries — sqlc migration in v0.6.x (ADR-0005).
--
-- Migrationsstand:
--   ✅ ck_frameworks — sqlc (this file)
--   ⏳ ck_controls / ck_evidence / ck_risks / ck_incidents / ck_policies /
--      ck_audits / ... — embedded SQL (Sitzungen E/F)

-- ── Frameworks ──────────────────────────────────────────────────────────────

-- name: CreateCKFramework :one
INSERT INTO ck_frameworks (org_id, name, version, is_builtin, framework_variant)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, org_id, name, version, is_builtin, created_at, framework_variant;

-- name: ListCKFrameworks :many
SELECT id, org_id, name, version, is_builtin, created_at, framework_variant
FROM ck_frameworks
WHERE org_id = $1
ORDER BY created_at ASC;

-- name: GetCKFramework :one
SELECT id, org_id, name, version, is_builtin, created_at, framework_variant
FROM ck_frameworks
WHERE id = $1 AND org_id = $2;

-- name: FindCKFrameworkByName :one
SELECT id, org_id, name, version, is_builtin, created_at, framework_variant
FROM ck_frameworks
WHERE org_id = $1 AND name = $2
LIMIT 1;

-- name: ListAllBuiltinCKFrameworks :many
SELECT id, org_id, name, version, is_builtin, created_at, framework_variant
FROM ck_frameworks
WHERE is_builtin = TRUE
ORDER BY created_at ASC;

-- name: CKFrameworkExists :one
SELECT EXISTS(SELECT 1 FROM ck_frameworks WHERE org_id = $1 AND name = $2);

-- name: DeleteCKFramework :execrows
DELETE FROM ck_frameworks
WHERE id = $1 AND org_id = $2;

-- name: UpdateCKFrameworkVariant :exec
UPDATE ck_frameworks
SET framework_variant = $1
WHERE id = $2 AND org_id = $3;

-- ── Controls ────────────────────────────────────────────────────────────────

-- name: BulkInsertCKControl :exec
-- Caller iterates per Control inside a tx (r.q.WithTx(tx)). Title/description
-- werden bei (framework_id, control_id)-Duplikat aktualisiert — andere Felder
-- bleiben intakt (z.B. manuell gesetztes manual_status).
INSERT INTO ck_controls
  (framework_id, org_id, control_id, title, description, domain, evidence_type, weight)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (framework_id, control_id) DO UPDATE
  SET title       = EXCLUDED.title,
      description = EXCLUDED.description;

-- name: UpdateCKControl :execrows
-- Partial-Update via NULL/COALESCE-Konvention:
--   - not_applicable wird IMMER überschrieben (Bool, nicht-NULL).
--   - reason/manual_status/owner: leerer String → NULL (NULLIF in Repo via spOptText).
--   - maturity_score = NULL → behalten; sonst überschreiben.
--   - due_date = NULL → behalten; sonst überschreiben.
UPDATE ck_controls SET
  not_applicable        = $3,
  not_applicable_reason = NULLIF(COALESCE(sqlc.narg('reason')::text, ''), ''),
  manual_status         = NULLIF(COALESCE(sqlc.narg('manual_status')::text, ''), ''),
  owner                 = NULLIF(COALESCE(sqlc.narg('owner')::text, ''), ''),
  maturity_score        = COALESCE(sqlc.narg('maturity_score')::int, maturity_score),
  due_date              = COALESCE(sqlc.narg('due_date')::date, due_date)
WHERE id = $1 AND org_id = $2;

-- name: UpdateCKControlSoAMetadata :execrows
UPDATE ck_controls SET
  soa_justification  = NULLIF(COALESCE(sqlc.narg('justification')::text, ''), ''),
  soa_implementation = NULLIF(COALESCE(sqlc.narg('implementation')::text, ''), ''),
  soa_responsible    = NULLIF(COALESCE(sqlc.narg('responsible')::text, ''), '')
WHERE id = $1 AND org_id = $2;

-- name: ListCKControls :many
SELECT id, framework_id, org_id, control_id, title,
       description, domain, evidence_type, weight,
       not_applicable, not_applicable_reason,
       manual_status, maturity_score, owner,
       last_reviewed_at, review_interval_days, next_review_due,
       last_reviewed_by, review_note, due_date
FROM ck_controls
WHERE framework_id = $1 AND org_id = $2
ORDER BY control_id ASC
LIMIT 1000;

-- name: GetCKControl :one
SELECT id, framework_id, org_id, control_id, title,
       description, domain, evidence_type, weight,
       not_applicable, not_applicable_reason,
       manual_status, maturity_score, owner,
       last_reviewed_at, review_interval_days, next_review_due,
       last_reviewed_by, review_note, due_date
FROM ck_controls
WHERE id = $1 AND org_id = $2;

-- name: ListCKControlsForSoA :many
-- SoA-Export: Controls + Evidence-Count (approved+pending) je Control.
SELECT c.control_id, c.title, c.domain,
       (NOT c.not_applicable)::bool         AS applicable,
       COALESCE(c.soa_justification, '')    AS justification,
       COALESCE(c.soa_implementation, '')   AS implementation,
       COALESCE(c.soa_responsible, '')      AS responsible,
       COALESCE(c.manual_status, '')        AS manual_status,
       COUNT(e.id) FILTER (WHERE e.status IN ('approved', 'pending'))::int AS evidence_count
FROM ck_controls c
LEFT JOIN ck_evidence e ON e.control_id = c.id AND e.org_id = c.org_id
WHERE c.framework_id = $1 AND c.org_id = $2
GROUP BY c.id, c.control_id, c.title, c.domain, c.not_applicable,
         c.soa_justification, c.soa_implementation, c.soa_responsible, c.manual_status
ORDER BY c.control_id ASC;

-- name: CountCKEvidenceByControl :many
-- Returns one row per control_id with the count of approved+pending evidence.
SELECT e.control_id::uuid AS control_id, COUNT(*)::int AS evidence_count
FROM ck_evidence e
JOIN ck_controls c ON c.id = e.control_id
WHERE e.org_id = $1 AND c.framework_id = $2
  AND e.status IN ('approved', 'pending')
GROUP BY e.control_id;

-- ── Evidence ────────────────────────────────────────────────────────────────

-- name: AddCKEvidence :one
INSERT INTO ck_evidence (control_id, org_id, title, description, source, file_path, file_size, expires_at, uploaded_by)
VALUES ($1, $2, $3, $4, $5, NULLIF(sqlc.arg('file_path')::text, ''), NULLIF(sqlc.arg('file_size')::bigint, 0), $6, $7)
RETURNING id, control_id, org_id, title, description, source,
          file_path, file_size, status, version,
          expires_at, expiry_notified_at, created_at, updated_at;

-- name: ListCKEvidence :many
SELECT id, control_id, org_id, title, description, source,
       file_path, file_size, status, version,
       expires_at, expiry_notified_at, created_at, updated_at
FROM ck_evidence
WHERE control_id = $1 AND org_id = $2
ORDER BY created_at DESC;

-- name: ListCKEvidenceByControls :many
SELECT id, control_id, org_id, title, description, source,
       file_path, file_size, status, version,
       expires_at, expiry_notified_at, created_at, updated_at
FROM ck_evidence
WHERE control_id = ANY($1::uuid[]) AND org_id = $2
ORDER BY control_id, created_at DESC;

-- name: GetCKExpiringEvidence :many
-- Within a single framework; only approved evidence.
SELECT e.id, e.control_id, e.org_id, e.title, e.description, e.source,
       e.file_path, e.file_size, e.status, e.version,
       e.expires_at, e.expiry_notified_at, e.created_at, e.updated_at
FROM ck_evidence e
JOIN ck_controls c ON c.id = e.control_id
WHERE e.org_id = $1 AND c.framework_id = $2
  AND e.status = 'approved'
  AND e.expires_at IS NOT NULL AND e.expires_at <= $3;

-- name: GetCKExpiringEvidenceAllFrameworks :many
SELECT e.id, e.control_id, e.org_id, e.title, e.description, e.source,
       e.file_path, e.file_size, e.status, e.version,
       e.expires_at, e.expiry_notified_at, e.created_at, e.updated_at
FROM ck_evidence e
JOIN ck_controls c ON c.id = e.control_id
WHERE e.org_id = $1
  AND e.status = 'approved'
  AND e.expires_at IS NOT NULL AND e.expires_at <= $2
ORDER BY e.expires_at ASC;

-- name: GetCKUnnotifiedExpiringEvidence :many
-- Minimal projection für den Expiry-Notifier-Worker; nur Evidence die
-- noch nicht benachrichtigt wurde (expiry_notified_at IS NULL).
SELECT e.id, e.org_id, e.title AS evidence_title,
       c.title AS control_title, e.expires_at
FROM ck_evidence e
JOIN ck_controls c ON c.id = e.control_id
WHERE e.org_id = $1
  AND e.expires_at IS NOT NULL
  AND e.expires_at <= $2
  AND e.expiry_notified_at IS NULL
ORDER BY e.expires_at ASC;

-- name: MarkCKEvidenceExpiryNotified :exec
UPDATE ck_evidence
SET expiry_notified_at = NOW(), updated_at = NOW()
WHERE id = ANY($1::uuid[]);

-- name: GetCKEvidence :one
SELECT id, control_id, org_id, title, description, source,
       file_path, file_size, status, version,
       expires_at, expiry_notified_at, created_at, updated_at
FROM ck_evidence
WHERE id = $1 AND org_id = $2;

-- name: ListCKEvidenceHistory :many
SELECT id, evidence_id, changed_by, changed_at,
       title, status, change_note
FROM ck_evidence_history
WHERE evidence_id = $1 AND org_id = $2
ORDER BY changed_at DESC;

-- name: ReviewCKEvidence :execrows
UPDATE ck_evidence
SET status      = $1,
    reviewed_by = $2,
    reviewed_at = NOW(),
    updated_at  = NOW()
WHERE id = $3 AND org_id = $4;

-- name: AddCKCollectorEvidence :one
INSERT INTO ck_evidence (control_id, org_id, title, source, collector_data, uploaded_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, control_id, org_id, title, description, source,
          file_path, file_size, status, version,
          expires_at, expiry_notified_at, created_at, updated_at;

-- ── Evidence Files ─────────────────────────────────────────────────────────

-- name: CreateCKEvidenceFile :one
INSERT INTO ck_evidence_files
  (org_id, evidence_id, control_id, original_name, stored_name, mime_type, size_bytes, uploaded_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, org_id, evidence_id, control_id, original_name, stored_name,
          mime_type, size_bytes, uploaded_by, created_at;

-- name: ListCKEvidenceFiles :many
SELECT id, org_id, evidence_id, control_id, original_name, stored_name,
       mime_type, size_bytes, uploaded_by, created_at
FROM ck_evidence_files
WHERE org_id = $1 AND evidence_id = $2
ORDER BY created_at DESC;

-- name: ListCKEvidenceFilesByControl :many
SELECT id, org_id, evidence_id, control_id, original_name, stored_name,
       mime_type, size_bytes, uploaded_by, created_at
FROM ck_evidence_files
WHERE org_id = $1 AND control_id = $2
ORDER BY created_at DESC;

-- name: GetCKEvidenceFile :one
SELECT id, org_id, evidence_id, control_id, original_name, stored_name,
       mime_type, size_bytes, uploaded_by, created_at
FROM ck_evidence_files
WHERE id = $1 AND org_id = $2;

-- name: DeleteCKEvidenceFile :one
DELETE FROM ck_evidence_files
WHERE id = $1 AND org_id = $2
RETURNING id, org_id, evidence_id, control_id, original_name, stored_name,
          mime_type, size_bytes, uploaded_by, created_at;

-- name: ListCKEvidenceForFramework :many
-- Audit-Export: jede Control mit allen Evidence (LEFT JOIN — Controls ohne
-- Evidence erscheinen mit leeren Evidence-Feldern).
SELECT c.id AS control_uuid, c.title AS control_title, c.control_id AS control_code,
       e.id AS evidence_id, e.title AS evidence_title, e.source AS evidence_source,
       e.description AS evidence_desc, e.file_path AS evidence_file_path,
       e.created_at AS evidence_created_at
FROM ck_controls c
LEFT JOIN ck_evidence e ON e.control_id = c.id AND e.org_id = $1
WHERE c.framework_id = $2 AND c.org_id = $1
ORDER BY c.control_id ASC, e.created_at DESC;

-- ── Risks ───────────────────────────────────────────────────────────────────
-- risk_score ist GENERATED ALWAYS (likelihood * impact) — wird nicht direkt
-- gesetzt. CreateRisk/UpdateRisk lassen die Spalte aus dem INSERT/SET, sie
-- erscheint nur im RETURNING.

-- name: CreateCKRisk :one
INSERT INTO ck_risks (org_id, title, description, category, likelihood, impact, owner, treatment, treatment_notes)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, org_id, title, description, category,
          likelihood, impact, risk_score, owner, status, treatment, treatment_notes,
          treatment_option, treatment_plan, treatment_owner,
          treatment_due_date, treatment_status,
          residual_likelihood, residual_impact,
          created_at, updated_at;

-- name: ListCKRisks :many
SELECT id, org_id, title, description, category,
       likelihood, impact, risk_score, owner, status, treatment, treatment_notes,
       treatment_option, treatment_plan, treatment_owner,
       treatment_due_date, treatment_status,
       residual_likelihood, residual_impact,
       created_at, updated_at
FROM ck_risks
WHERE org_id = $1
ORDER BY risk_score DESC, created_at DESC
LIMIT 10000;

-- name: ListCKRisksPaged :many
SELECT id, org_id, title, description, category,
       likelihood, impact, risk_score, owner, status, treatment, treatment_notes,
       treatment_option, treatment_plan, treatment_owner,
       treatment_due_date, treatment_status,
       residual_likelihood, residual_impact,
       created_at, updated_at
FROM ck_risks
WHERE org_id = $1
ORDER BY risk_score DESC, created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountCKRisks :one
SELECT COUNT(*) FROM ck_risks WHERE org_id = $1;

-- name: GetCKRisk :one
SELECT id, org_id, title, description, category,
       likelihood, impact, risk_score, owner, status, treatment, treatment_notes,
       treatment_option, treatment_plan, treatment_owner,
       treatment_due_date, treatment_status,
       residual_likelihood, residual_impact,
       created_at, updated_at
FROM ck_risks
WHERE id = $1 AND org_id = $2;

-- name: UpdateCKRisk :one
UPDATE ck_risks SET
  title           = $3,
  description     = $4,
  category        = $5,
  likelihood      = $6,
  impact          = $7,
  owner           = $8,
  status          = $9,
  treatment       = $10,
  treatment_notes = $11,
  updated_at      = NOW()
WHERE id = $1 AND org_id = $2
RETURNING id, org_id, title, description, category,
          likelihood, impact, risk_score, owner, status, treatment, treatment_notes,
          treatment_option, treatment_plan, treatment_owner,
          treatment_due_date, treatment_status,
          residual_likelihood, residual_impact,
          created_at, updated_at;

-- name: DeleteCKRisk :execrows
DELETE FROM ck_risks WHERE id = $1 AND org_id = $2;

-- name: CKSupplierExists :one
SELECT EXISTS(SELECT 1 FROM ck_suppliers WHERE id = $1 AND org_id = $2);

-- name: LinkCKSupplierRisk :exec
INSERT INTO ck_supplier_risks (supplier_id, risk_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: UnlinkCKSupplierRisk :exec
DELETE FROM ck_supplier_risks WHERE supplier_id = $1 AND risk_id = $2;

-- name: ListCKSupplierRisks :many
SELECT r.id, r.org_id, r.title, r.description, r.category,
       r.likelihood, r.impact, r.risk_score, r.owner, r.status, r.treatment, r.treatment_notes,
       r.treatment_option, r.treatment_plan, r.treatment_owner,
       r.treatment_due_date, r.treatment_status,
       r.residual_likelihood, r.residual_impact,
       r.created_at, r.updated_at
FROM ck_risks r
JOIN ck_supplier_risks sr ON sr.risk_id = r.id
WHERE sr.supplier_id = $1 AND r.org_id = $2
ORDER BY r.created_at DESC;

-- ── Control Tasks ───────────────────────────────────────────────────────────

-- name: ListCKControlTasks :many
SELECT id, control_id, org_id, text, completed, created_at, updated_at
FROM ck_control_tasks
WHERE control_id = $1 AND org_id = $2
ORDER BY created_at ASC
LIMIT 500;

-- name: CreateCKControlTask :one
INSERT INTO ck_control_tasks (control_id, org_id, text)
VALUES ($1, $2, $3)
RETURNING id, control_id, org_id, text, completed, created_at, updated_at;

-- name: UpdateCKControlTask :one
UPDATE ck_control_tasks
SET completed = $1, updated_at = NOW()
WHERE id = $2 AND control_id = $3 AND org_id = $4
RETURNING id, control_id, org_id, text, completed, created_at, updated_at;

-- name: DeleteCKControlTask :execrows
DELETE FROM ck_control_tasks
WHERE id = $1 AND control_id = $2 AND org_id = $3;

-- ── Policies ────────────────────────────────────────────────────────────────

-- name: CreateCKPolicy :one
INSERT INTO ck_policies (org_id, title, description, category, version, effective_date, review_date, owner)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, org_id, title, description, category, status, version,
          effective_date, review_date, owner, created_at, updated_at,
          version_num, version_note, last_updated_by,
          reviewed_at, next_review_due;

-- name: ListCKPolicies :many
SELECT id, org_id, title, description, category, status, version,
       effective_date, review_date, owner, created_at, updated_at,
       version_num, version_note, last_updated_by,
       reviewed_at, next_review_due
FROM ck_policies
WHERE org_id = $1
ORDER BY created_at DESC
LIMIT 10000;

-- name: GetCKPolicy :one
SELECT id, org_id, title, description, category, status, version,
       effective_date, review_date, owner, created_at, updated_at,
       version_num, version_note, last_updated_by,
       reviewed_at, next_review_due
FROM ck_policies
WHERE id = $1 AND org_id = $2;

-- name: CountCKPolicies :one
SELECT COUNT(*) FROM ck_policies WHERE org_id = $1;

-- name: ListCKPoliciesPaged :many
SELECT id, org_id, title, description, category, status, version,
       effective_date, review_date, owner, created_at, updated_at,
       version_num, version_note, last_updated_by,
       reviewed_at, next_review_due
FROM ck_policies
WHERE org_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: SnapshotCKPolicyVersion :exec
-- Snapshots the current ck_policies row into ck_policy_versions before UpdatePolicy
-- bumps version_num. ON CONFLICT (policy_id, version) DO NOTHING macht das
-- idempotent — wenn der Snapshot durch frühere Updates schon existiert,
-- bleibt er unverändert.
INSERT INTO ck_policy_versions (org_id, policy_id, version, title, content, status, version_note, updated_by)
SELECT p.org_id, p.id, p.version_num, p.title, COALESCE(p.description,''), p.status,
       COALESCE(p.version_note,''), COALESCE(p.last_updated_by,'')
FROM ck_policies p
WHERE p.id = $1 AND p.org_id = $2
ON CONFLICT (policy_id, version) DO NOTHING;

-- name: UpdateCKPolicy :one
-- reviewed_at-Logik: wenn last_updated_by != '' (also Review-Aktion), → NOW();
-- sonst behalten. Logik wird im Repo entschieden und via sqlc.arg gesteuert.
UPDATE ck_policies
SET title           = $3,
    description     = $4,
    category        = $5,
    status          = $6,
    version         = $7,
    effective_date  = $8,
    review_date     = $9,
    owner           = $10,
    version_num     = version_num + 1,
    version_note    = $11,
    last_updated_by = $12,
    reviewed_at     = CASE WHEN sqlc.arg('refresh_review')::bool THEN NOW() ELSE reviewed_at END,
    next_review_due = $13,
    updated_at      = NOW()
WHERE id = $1 AND org_id = $2
RETURNING id, org_id, title, description, category, status, version,
          effective_date, review_date, owner, created_at, updated_at,
          version_num, version_note, last_updated_by,
          reviewed_at, next_review_due;

-- ── Policy-Versions ─────────────────────────────────────────────────────────

-- name: ListCKPolicyVersions :many
SELECT id, org_id, policy_id, version, title, content, status,
       version_note, updated_by, created_at
FROM ck_policy_versions
WHERE policy_id = $1 AND org_id = $2
ORDER BY version DESC;

-- name: GetCKPolicyVersion :one
SELECT id, org_id, policy_id, version, title, content, status,
       version_note, updated_by, created_at
FROM ck_policy_versions
WHERE policy_id = $1 AND org_id = $2 AND version = $3;

-- ── Risk ↔ Control Links ────────────────────────────────────────────────────

-- name: LinkCKRiskControl :exec
INSERT INTO ck_risk_control_links (risk_id, control_id, org_id)
VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING;

-- name: UnlinkCKRiskControl :execrows
DELETE FROM ck_risk_control_links
WHERE risk_id = $1 AND control_id = $2 AND org_id = $3;

-- ── Questionnaires ──────────────────────────────────────────────────────────

-- name: CreateCKQuestionnaire :one
INSERT INTO ck_questionnaires (org_id, name, description, is_template)
VALUES ($1, $2, $3, $4)
RETURNING id, org_id, name, description, is_template, created_at, updated_at;

-- name: GetCKQuestionnaireBase :one
SELECT id, org_id, name, description, is_template, created_at, updated_at
FROM ck_questionnaires
WHERE id = $1 AND org_id = $2;

-- name: ListCKQuestionnaires :many
SELECT id, org_id, name, description, is_template, created_at, updated_at
FROM ck_questionnaires
WHERE org_id = $1
  AND (sqlc.narg('is_template')::bool IS NULL OR is_template = sqlc.narg('is_template')::bool)
ORDER BY created_at ASC;

-- name: UpdateCKQuestionnaire :one
UPDATE ck_questionnaires
SET name = $3, description = $4, is_template = $5, updated_at = NOW()
WHERE id = $1 AND org_id = $2
RETURNING id, org_id, name, description, is_template, created_at, updated_at;

-- name: DeleteCKQuestionnaire :execrows
DELETE FROM ck_questionnaires WHERE id = $1 AND org_id = $2;

-- ── Questions ───────────────────────────────────────────────────────────────

-- name: NextCKQuestionOrderIdx :one
SELECT COALESCE(MAX(order_idx) + 1, 0)::int
FROM ck_questionnaire_questions
WHERE questionnaire_id = $1;

-- name: CreateCKQuestion :one
INSERT INTO ck_questionnaire_questions
  (questionnaire_id, order_idx, question_text, question_type, options, required, control_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, questionnaire_id, order_idx, question_text, question_type,
          options, required, control_id, created_at, updated_at;

-- name: GetCKQuestion :one
SELECT id, questionnaire_id, order_idx, question_text, question_type,
       options, required, control_id, created_at, updated_at
FROM ck_questionnaire_questions
WHERE id = $1 AND questionnaire_id = $2;

-- name: UpdateCKQuestion :one
UPDATE ck_questionnaire_questions
SET question_text = $3, question_type = $4, options = $5, required = $6,
    control_id = $7, updated_at = NOW()
WHERE id = $1 AND questionnaire_id = $2
RETURNING id, questionnaire_id, order_idx, question_text, question_type,
          options, required, control_id, created_at, updated_at;

-- name: DeleteCKQuestion :execrows
DELETE FROM ck_questionnaire_questions
WHERE id = $1 AND questionnaire_id = $2;

-- name: ListCKQuestions :many
SELECT id, questionnaire_id, order_idx, question_text, question_type,
       options, required, control_id, created_at, updated_at
FROM ck_questionnaire_questions
WHERE questionnaire_id = $1
ORDER BY order_idx ASC;

-- name: ReorderCKQuestion :exec
-- Aufrufer iteriert die ID-Liste und ruft je Aufruf den neuen order_idx.
UPDATE ck_questionnaire_questions
SET order_idx = $1, updated_at = NOW()
WHERE id = $2 AND questionnaire_id = $3;

-- ── Access Review Campaigns ────────────────────────────────────────────────

-- name: CreateCKAccessReviewCampaign :one
INSERT INTO ck_access_review_campaigns
  (org_id, title, description, reviewer_email, scope, due_date)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, org_id, title, description, status, reviewer_email,
          scope, due_date, completed_at, created_by, created_at, updated_at;

-- name: GetCKAccessReviewCampaign :one
SELECT id, org_id, title, description, status, reviewer_email,
       scope, due_date, completed_at, created_by, created_at, updated_at
FROM ck_access_review_campaigns
WHERE id = $1 AND org_id = $2;

-- name: ListCKAccessReviewCampaigns :many
SELECT id, org_id, title, description, status, reviewer_email,
       scope, due_date, completed_at, created_by, created_at, updated_at
FROM ck_access_review_campaigns
WHERE org_id = $1
ORDER BY created_at DESC;

-- name: UpdateCKAccessReviewCampaign :one
UPDATE ck_access_review_campaigns
SET title          = $1,
    description    = $2,
    reviewer_email = $3,
    scope          = $4,
    status         = $5,
    due_date       = $6,
    completed_at   = $7,
    updated_at     = NOW()
WHERE id = $8 AND org_id = $9
RETURNING id, org_id, title, description, status, reviewer_email,
          scope, due_date, completed_at, created_by, created_at, updated_at;

-- name: DeleteCKAccessReviewCampaign :execrows
DELETE FROM ck_access_review_campaigns WHERE id = $1 AND org_id = $2;

-- ── Access Review Items ────────────────────────────────────────────────────

-- name: CreateCKAccessReviewItem :one
INSERT INTO ck_access_review_items
  (campaign_id, org_id, user_email, access_level)
VALUES ($1, $2, $3, $4)
RETURNING id, campaign_id, org_id, user_email, access_level, decision,
          reviewer_comment, decided_at, created_at;

-- name: ListCKAccessReviewItems :many
SELECT id, campaign_id, org_id, user_email, access_level, decision,
       reviewer_comment, decided_at, created_at
FROM ck_access_review_items
WHERE campaign_id = $1 AND org_id = $2
ORDER BY created_at ASC;

-- name: UpdateCKAccessReviewItem :one
-- decided_at wird in Go vorberechnet (only set when approving/revoking).
UPDATE ck_access_review_items
SET decision         = COALESCE(NULLIF(sqlc.arg('decision')::text, ''), decision),
    reviewer_comment = sqlc.arg('reviewer_comment')::text,
    decided_at       = CASE WHEN sqlc.arg('decision')::text IN ('approved','revoked')
                            THEN sqlc.narg('decided_at')::timestamptz
                            ELSE decided_at
                       END
WHERE id = $1 AND org_id = $2
RETURNING id, campaign_id, org_id, user_email, access_level, decision,
          reviewer_comment, decided_at, created_at;

-- ── Audit Milestones ───────────────────────────────────────────────────────

-- name: CreateCKMilestone :one
INSERT INTO ck_audit_milestones
  (org_id, framework_id, title, description, milestone_date, milestone_type, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, org_id, framework_id, title, description,
          milestone_date::text AS milestone_date,
          milestone_type, status, created_by, created_at, updated_at;

-- name: GetCKMilestone :one
SELECT id, org_id, framework_id, title, description,
       milestone_date::text AS milestone_date,
       milestone_type, status, created_by, created_at, updated_at
FROM ck_audit_milestones
WHERE id = $1 AND org_id = $2;

-- name: ListCKMilestones :many
SELECT id, org_id, framework_id, title, description,
       milestone_date::text AS milestone_date,
       milestone_type, status, created_by, created_at, updated_at
FROM ck_audit_milestones
WHERE org_id = $1
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
ORDER BY milestone_date ASC;

-- name: UpdateCKMilestone :one
UPDATE ck_audit_milestones
SET title          = $1,
    description    = $2,
    milestone_date = $3,
    milestone_type = $4,
    status         = $5,
    updated_at     = NOW()
WHERE id = $6 AND org_id = $7
RETURNING id, org_id, framework_id, title, description,
          milestone_date::text AS milestone_date,
          milestone_type, status, created_by, created_at, updated_at;

-- name: DeleteCKMilestone :execrows
DELETE FROM ck_audit_milestones WHERE id = $1 AND org_id = $2;

-- name: NextCKMilestone :one
SELECT id, org_id, framework_id, title, description,
       milestone_date::text AS milestone_date,
       milestone_type, status, created_by, created_at, updated_at
FROM ck_audit_milestones
WHERE org_id = $1
  AND status = 'upcoming'
  AND milestone_date >= CURRENT_DATE
ORDER BY milestone_date ASC
LIMIT 1;

-- ── CCM (Continuous Control Monitoring) ────────────────────────────────────

-- name: CreateCKCCMCheck :one
INSERT INTO ck_ccm_checks (org_id, control_id, name, check_type, config, interval_hours)
VALUES ($1, $2, $3, sqlc.arg('check_type')::ck_check_type, $4, $5)
RETURNING id, org_id, control_id, name, check_type::text AS check_type,
          config, interval_hours, last_run_at, last_status, last_output,
          enabled, created_at, updated_at;

-- name: GetCKCCMCheck :one
SELECT id, org_id, control_id, name, check_type::text AS check_type,
       config, interval_hours, last_run_at, last_status, last_output,
       enabled, created_at, updated_at
FROM ck_ccm_checks
WHERE id = $1 AND org_id = $2;

-- name: ListCKCCMChecks :many
SELECT id, org_id, control_id, name, check_type::text AS check_type,
       config, interval_hours, last_run_at, last_status, last_output,
       enabled, created_at, updated_at
FROM ck_ccm_checks
WHERE org_id = $1
ORDER BY created_at ASC;

-- name: ListCKDueCCMChecks :many
-- Liefert alle aktiven Checks, die wieder dran sind:
--   last_run_at IS NULL ODER last_run_at + interval_hours < NOW().
SELECT id, org_id, control_id, name, check_type::text AS check_type,
       config, interval_hours, last_run_at, last_status, last_output,
       enabled, created_at, updated_at
FROM ck_ccm_checks
WHERE enabled = TRUE
  AND (last_run_at IS NULL
       OR last_run_at + (interval_hours * INTERVAL '1 hour') < NOW())
ORDER BY last_run_at ASC NULLS FIRST;

-- name: DeleteCKCCMCheck :execrows
DELETE FROM ck_ccm_checks WHERE id = $1 AND org_id = $2;

-- name: UpdateCKCCMCheckEnabled :exec
UPDATE ck_ccm_checks SET enabled = $1, updated_at = NOW() WHERE id = $2;

-- name: UpdateCKCCMCheckLastRun :exec
UPDATE ck_ccm_checks
SET last_run_at = NOW(), last_status = $1, last_output = $2, updated_at = NOW()
WHERE id = $3;

-- name: SaveCKCCMResult :exec
INSERT INTO ck_ccm_results (check_id, status, output) VALUES ($1, $2, $3);

-- name: ListCKCCMResults :many
SELECT id, check_id, status, output, ran_at
FROM ck_ccm_results
WHERE check_id = $1
ORDER BY ran_at DESC
LIMIT $2;

-- ── Control Approvals ──────────────────────────────────────────────────────

-- name: CreateCKApprovalRequest :one
INSERT INTO ck_control_approvals
  (org_id, control_id, requested_by, requested_status, current_status, comment)
VALUES ($1, $2, $3, $4, $5, NULLIF(sqlc.arg('comment')::text, ''))
RETURNING id, org_id, control_id, requested_by, requested_status, current_status,
          comment, status, reviewed_by, reviewed_at, review_comment, created_at;

-- name: GetCKApproval :one
SELECT id, org_id, control_id, requested_by, requested_status, current_status,
       comment, status, reviewed_by, reviewed_at, review_comment, created_at
FROM ck_control_approvals
WHERE id = $1 AND org_id = $2;

-- name: ListCKPendingApprovals :many
SELECT
  a.id, a.org_id, a.control_id, a.requested_by,
  a.requested_status, a.current_status,
  COALESCE(a.comment, '')::text    AS comment,
  a.status,
  a.reviewed_by, a.reviewed_at,
  COALESCE(a.review_comment, '')::text AS review_comment,
  a.created_at,
  COALESCE(c.title, '')::text         AS control_title,
  COALESCE(c.control_id, '')::text    AS control_ref,
  COALESCE(u.display_name, u.email, '')::text AS requester_name,
  COALESCE(u.email, '')::text                 AS requester_email
FROM ck_control_approvals a
LEFT JOIN ck_controls c ON c.id = a.control_id
LEFT JOIN users u ON u.id = a.requested_by
WHERE a.org_id = $1 AND a.status = 'pending'
ORDER BY a.created_at DESC;

-- name: ReviewCKApproval :one
-- Setzt status auf approved/rejected und gibt control_id + requested_status
-- zurück, damit der Repo-Layer optional die Control-Status-Änderung anwenden kann.
UPDATE ck_control_approvals
SET status         = $3,
    reviewed_by    = $4,
    reviewed_at    = $5,
    review_comment = NULLIF(sqlc.arg('comment')::text, '')
WHERE id = $1 AND org_id = $2 AND status = 'pending'
RETURNING control_id, requested_status;

-- name: ApplyCKApprovedControlStatus :exec
UPDATE ck_controls
SET not_applicable = $3, manual_status = $4, updated_at = NOW()
WHERE id = $1 AND org_id = $2;

-- name: CountCKPendingApprovals :one
SELECT COUNT(*) FROM ck_control_approvals
WHERE org_id = $1 AND status = 'pending';

-- name: GetCKOrgApprovalRequired :one
SELECT COALESCE(approval_required, FALSE)::bool AS required
FROM organizations WHERE id = $1;

-- name: SetCKOrgApprovalRequired :execrows
UPDATE organizations
SET approval_required = $2, updated_at = NOW()
WHERE id = $1;

-- ── Control Exceptions (Waiver / Exception Records) ────────────────────────

-- name: ListCKControlExceptions :many
SELECT id, org_id, control_id, title, reason, risk_accepted,
       approved_by, expires_at, status, created_by, created_at, updated_at
FROM ck_control_exceptions
WHERE org_id = $1 AND control_id = $2
ORDER BY created_at DESC;

-- name: ListAllCKControlExceptions :many
SELECT id, org_id, control_id, title, reason, risk_accepted,
       approved_by, expires_at, status, created_by, created_at, updated_at
FROM ck_control_exceptions
WHERE org_id = $1
ORDER BY created_at DESC;

-- name: CreateCKControlException :one
INSERT INTO ck_control_exceptions
       (org_id, control_id, title, reason, risk_accepted, approved_by,
        expires_at, status, created_by)
VALUES ($1, $2, $3, $4, $5,
        NULLIF(sqlc.arg('approved_by')::text, ''),
        $6, 'active',
        NULLIF(sqlc.arg('created_by')::text, ''))
RETURNING id, org_id, control_id, title, reason, risk_accepted,
          approved_by, expires_at, status, created_by, created_at, updated_at;

-- name: UpdateCKControlException :one
-- Partial Update via COALESCE + NULLIF. approved_by hat eine Spezialregel:
-- "" wird zu NULL (explizit löschen), nil-Input behält den Wert.
UPDATE ck_control_exceptions SET
  title         = COALESCE(NULLIF(sqlc.narg('title')::text, ''), title),
  reason        = COALESCE(NULLIF(sqlc.narg('reason')::text, ''), reason),
  risk_accepted = COALESCE(NULLIF(sqlc.narg('risk_accepted')::text, ''), risk_accepted),
  approved_by   = CASE WHEN sqlc.narg('approved_by')::text IS NOT NULL
                       THEN NULLIF(sqlc.narg('approved_by')::text, '')
                       ELSE approved_by END,
  expires_at    = COALESCE(sqlc.narg('expires_at')::timestamptz, expires_at),
  status        = COALESCE(NULLIF(sqlc.narg('status')::text, ''), status),
  updated_at    = NOW()
WHERE id = $1 AND org_id = $2
RETURNING id, org_id, control_id, title, reason, risk_accepted,
          approved_by, expires_at, status, created_by, created_at, updated_at;

-- name: DeleteCKControlException :execrows
DELETE FROM ck_control_exceptions WHERE id = $1 AND org_id = $2;

-- ── Control Changelog ───────────────────────────────────────────────────────

-- name: AppendCKControlChange :exec
INSERT INTO ck_control_changelog
  (control_id, org_id, user_id, user_email, field, old_value, new_value, changed_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, NOW());

-- name: ListCKControlChanges :many
SELECT id, control_id, user_email, field, old_value, new_value, changed_at
FROM ck_control_changelog
WHERE org_id = $1 AND control_id = $2
ORDER BY changed_at DESC
LIMIT 50;

-- ── Comments ────────────────────────────────────────────────────────────────

-- name: ListCKComments :many
SELECT id, org_id, entity_type, entity_id, author_email, body, created_at
FROM ck_comments
WHERE org_id = $1 AND entity_type = $2 AND entity_id = $3
ORDER BY created_at ASC
LIMIT 200;

-- name: CreateCKComment :one
INSERT INTO ck_comments (org_id, entity_type, entity_id, author_email, body)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, org_id, entity_type, entity_id, author_email, body, created_at;

-- name: DeleteCKComment :execrows
DELETE FROM ck_comments WHERE id = $1 AND org_id = $2;

-- ── Score History ───────────────────────────────────────────────────────────

-- name: InsertCKScoreSnapshot :exec
INSERT INTO ck_score_history (org_id, framework_id, score, controls_total, controls_implemented)
VALUES ($1, $2, $3, $4, $5);

-- name: GetCKScoreHistory :many
-- Aggregiert pro Tag (TO_CHAR auf UTC); MAX statt LAST/FIRST_VALUE weil
-- mehrere Snapshots am gleichen Tag normaler Fall sind. Score wird in
-- float8 gecastet damit sqlc keinen interface{}-Typ generiert.
SELECT
    TO_CHAR(recorded_at AT TIME ZONE 'UTC', 'YYYY-MM-DD')::text AS date,
    MAX(score)::float8                                         AS score,
    MAX(controls_total)::int                                   AS controls_total,
    MAX(controls_implemented)::int                             AS controls_implemented
FROM ck_score_history
WHERE org_id = $1
  AND framework_id IS NULL
  AND recorded_at >= NOW() - (sqlc.arg('days')::int || ' days')::INTERVAL
GROUP BY date
ORDER BY date ASC;

-- ── Bulk-Control-Status ─────────────────────────────────────────────────────

-- name: BulkUpdateCKControlStatus :exec
UPDATE ck_controls
SET manual_status = $1, updated_at = NOW()
WHERE id = ANY(sqlc.arg('ids')::uuid[]) AND org_id = $2;

-- ── Collaborative Tasks ─────────────────────────────────────────────────────

-- name: CreateCKTask :one
INSERT INTO ck_tasks
  (org_id, entity_type, entity_id, title, description, assignee_email, due_date, status, priority)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, org_id, entity_type, entity_id, title, description,
          assignee_email, due_date, status, priority, created_by,
          created_at, updated_at;

-- name: ListCKTasks :many
SELECT id, org_id, entity_type, entity_id, title, description,
       assignee_email, due_date, status, priority, created_by,
       created_at, updated_at
FROM ck_tasks
WHERE org_id = $1 AND entity_type = $2 AND entity_id = $3
ORDER BY created_at DESC;

-- name: UpdateCKTask :one
UPDATE ck_tasks SET
  title          = COALESCE(sqlc.narg('title')::text, title),
  description    = COALESCE(sqlc.narg('description')::text, description),
  assignee_email = COALESCE(sqlc.narg('assignee_email')::text, assignee_email),
  due_date       = COALESCE(sqlc.narg('due_date')::date, due_date),
  status         = COALESCE(sqlc.narg('status')::text, status),
  priority       = COALESCE(sqlc.narg('priority')::text, priority),
  updated_at     = NOW()
WHERE id = $1 AND org_id = $2
RETURNING id, org_id, entity_type, entity_id, title, description,
          assignee_email, due_date, status, priority, created_by,
          created_at, updated_at;

-- name: DeleteCKTask :execrows
DELETE FROM ck_tasks WHERE id = $1 AND org_id = $2;

-- name: ListCKOverdueTasks :many
SELECT id, org_id, entity_type, entity_id, title, description,
       assignee_email, due_date, status, priority, created_by,
       created_at, updated_at
FROM ck_tasks
WHERE org_id = $1 AND due_date < NOW() AND status != 'done'
ORDER BY due_date ASC;

-- ── Assessments (Supplier-Portal) ───────────────────────────────────────────

-- name: CreateCKAssessment :exec
INSERT INTO ck_supplier_assessments
  (org_id, supplier_id, questionnaire_id, token_hash, expires_at, status)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetCKAssessmentByTokenHash :one
SELECT id, org_id, supplier_id, questionnaire_id, token_hash, expires_at,
       status, submitted_at, submitted_by_ip, user_agent, created_at
FROM ck_supplier_assessments
WHERE token_hash = $1;

-- name: UpdateCKAssessmentStatus :execrows
-- Submitted/reviewed sind Endzustände: einmal gesetzt, nicht mehr durch ein
-- weiteres submitted/reviewed-Update überschreibbar. Zustandsübergänge wie
-- expired→submitted oder draft→submitted bleiben weiterhin möglich.
UPDATE ck_supplier_assessments
SET status          = $2,
    submitted_at    = $3,
    submitted_by_ip = $4,
    user_agent      = $5
WHERE id = $1
  AND ($2 NOT IN ('submitted','reviewed') OR status NOT IN ('submitted','reviewed'));

-- name: GetCKAssessmentBase :one
-- Used by GetAssessmentWithQuestionnaire (Composer-Pattern).
SELECT id, org_id, supplier_id, questionnaire_id, token_hash, expires_at,
       status, submitted_at, submitted_by_ip, user_agent, created_at
FROM ck_supplier_assessments
WHERE id = $1;

-- name: ListCKAssessmentsForSupplier :many
SELECT id, org_id, supplier_id, questionnaire_id, token_hash, expires_at,
       status, submitted_at, submitted_by_ip, user_agent, created_at
FROM ck_supplier_assessments
WHERE org_id = $1 AND supplier_id = $2
ORDER BY created_at DESC;

-- name: UpdateCKSupplierAssessmentStatus :exec
UPDATE ck_suppliers
SET assessment_status = $2, last_assessment_at = $3
WHERE id = $1 AND org_id = sqlc.arg('org_id')::uuid;

-- name: MarkCKAssessmentReviewed :one
-- Setzt Assessment auf 'reviewed' und gibt supplier_id für den nachfolgenden
-- Supplier-Update zurück. Wird vom Repo in einer TX mit Update-Supplier
-- kombiniert.
UPDATE ck_supplier_assessments
SET status = 'reviewed', updated_at = NOW()
WHERE id = $1 AND org_id = $2 AND status = 'submitted'
RETURNING supplier_id;

-- ── Answers + Reviews ───────────────────────────────────────────────────────

-- name: UpsertCKAnswer :exec
INSERT INTO ck_supplier_answers
  (assessment_id, question_id, answer_text, answer_bool, answer_options, file_url)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (assessment_id, question_id) DO UPDATE
  SET answer_text    = EXCLUDED.answer_text,
      answer_bool    = EXCLUDED.answer_bool,
      answer_options = EXCLUDED.answer_options,
      file_url       = EXCLUDED.file_url,
      updated_at     = NOW();

-- name: UpdateCKAnswerReview :execrows
-- org_id wird via JOIN auf assessments validiert (ck_supplier_answers hat
-- keine eigene org_id-Spalte — siehe Migration 048; existierender Code
-- referenzierte sa.org_id was zur Laufzeit gefehlt hätte). Diese Variante
-- ist korrekt und multi-tenant-safe via FK-Kette.
UPDATE ck_supplier_answers
SET review_status = $1, rework_note = NULLIF($2::text, '')
WHERE ck_supplier_answers.id = $3
  AND assessment_id = $4
  AND assessment_id IN (
      SELECT asm.id FROM ck_supplier_assessments asm WHERE asm.org_id = $5
  );

-- name: GetCKAnswerWithQuestion :one
SELECT sa.id           AS answer_id,
       sa.assessment_id,
       asm.org_id,
       sa.question_id,
       qq.question_text,
       qq.control_id,
       COALESCE(sa.answer_text, '')::text AS answer_text,
       COALESCE(sa.file_url, '')::text    AS file_url,
       sa.review_status,
       sa.rework_note,
       sa.cert_expiry_date
FROM ck_supplier_answers sa
JOIN ck_supplier_assessments asm ON asm.id = sa.assessment_id
JOIN ck_questionnaire_questions qq ON qq.id = sa.question_id
WHERE sa.id = $1 AND asm.org_id = $2;

-- name: GetCKAnswersForAssessment :many
SELECT sa.id,
       qq.question_text,
       COALESCE(sa.answer_text, '')::text AS answer_text,
       COALESCE(sa.file_url, '')::text    AS file_url,
       sa.review_status,
       sa.rework_note,
       qq.control_id,
       sa.cert_expiry_date
FROM ck_supplier_answers sa
JOIN ck_supplier_assessments asm ON asm.id = sa.assessment_id
JOIN ck_questionnaire_questions qq ON qq.id = sa.question_id
WHERE sa.assessment_id = $1 AND asm.org_id = $2
ORDER BY qq.order_idx ASC;

-- ── Framework Mappings (Story 28.2) ─────────────────────────────────────────

-- name: CreateCKMapping :one
INSERT INTO ck_framework_mappings (org_id, source_control_id, target_control_id)
VALUES ($1, $2, $3)
ON CONFLICT (org_id, source_control_id, target_control_id) DO NOTHING
RETURNING id, org_id, source_control_id, target_control_id, created_at;

-- name: ListCKMappingsByOrg :many
SELECT id, org_id, source_control_id, target_control_id, created_at
FROM ck_framework_mappings
WHERE org_id = $1
ORDER BY created_at ASC;

-- name: DeleteCKMapping :execrows
DELETE FROM ck_framework_mappings WHERE id = $1 AND org_id = $2;

-- name: GetCKMappingsBySourceControlIDs :many
SELECT id, org_id, source_control_id, target_control_id, created_at
FROM ck_framework_mappings
WHERE org_id = $1 AND source_control_id = ANY($2::uuid[]);

-- ── Cross-Framework Mappings (global reference table) ───────────────────────

-- name: SeedCKGlobalControlMapping :exec
INSERT INTO ck_framework_control_mappings
  (source_framework, source_control_code, target_framework, target_control_code, mapping_type)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT DO NOTHING;

-- ── CAPAs (Corrective and Preventive Actions) ───────────────────────────────

-- name: CreateCKCAPA :one
INSERT INTO ck_capas (org_id, source_type, source_id, title, description,
                      assignee_email, due_date, priority)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, org_id, source_type, source_id, title, description,
          root_cause, action_plan, assignee_email, due_date, priority,
          status, verification_note, closed_at, created_at, updated_at;

-- name: GetCKCAPA :one
SELECT id, org_id, source_type, source_id, title, description,
       root_cause, action_plan, assignee_email, due_date, priority,
       status, verification_note, closed_at, created_at, updated_at
FROM ck_capas
WHERE id = $1 AND org_id = $2;

-- name: ListCKCAPAs :many
SELECT id, org_id, source_type, source_id, title, description,
       root_cause, action_plan, assignee_email, due_date, priority,
       status, verification_note, closed_at, created_at, updated_at
FROM ck_capas
WHERE org_id = $1
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
ORDER BY created_at DESC;

-- name: ListCKCAPAsForSource :many
SELECT id, org_id, source_type, source_id, title, description,
       root_cause, action_plan, assignee_email, due_date, priority,
       status, verification_note, closed_at, created_at, updated_at
FROM ck_capas
WHERE org_id = $1 AND source_type = $2 AND source_id = $3
ORDER BY created_at DESC;

-- name: ListCKCAPAsPaged :many
SELECT id, org_id, source_type, source_id, title, description,
       root_cause, action_plan, assignee_email, due_date, priority,
       status, verification_note, closed_at, created_at, updated_at
FROM ck_capas
WHERE org_id = $1
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountCKCAPAs :one
SELECT COUNT(*) FROM ck_capas
WHERE org_id = $1
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text);

-- name: UpdateCKCAPA :one
-- COALESCE-Pattern: nil-narg → behalten. Spezialregel für closed_at:
-- Bei Übergang in Status 'closed' wird closed_at = NOW() gesetzt — aber
-- nur wenn vorher noch NULL. Wechselt der Status zurück, bleibt closed_at
-- erhalten (audit-trail-Spalte).
UPDATE ck_capas SET
  title             = COALESCE(sqlc.narg('title')::text, title),
  description       = COALESCE(sqlc.narg('description')::text, description),
  root_cause        = COALESCE(sqlc.narg('root_cause')::text, root_cause),
  action_plan       = COALESCE(sqlc.narg('action_plan')::text, action_plan),
  assignee_email    = COALESCE(sqlc.narg('assignee_email')::text, assignee_email),
  due_date          = COALESCE(sqlc.narg('due_date')::date, due_date),
  priority          = COALESCE(sqlc.narg('priority')::text, priority),
  status            = COALESCE(sqlc.narg('status')::text, status),
  verification_note = COALESCE(sqlc.narg('verification_note')::text, verification_note),
  closed_at         = CASE
                       WHEN sqlc.narg('status')::text = 'closed' AND closed_at IS NULL THEN NOW()
                       ELSE closed_at
                      END,
  updated_at        = NOW()
WHERE id = $1 AND org_id = $2
RETURNING id, org_id, source_type, source_id, title, description,
          root_cause, action_plan, assignee_email, due_date, priority,
          status, verification_note, closed_at, created_at, updated_at;

-- name: DeleteCKCAPA :execrows
DELETE FROM ck_capas WHERE id = $1 AND org_id = $2;

-- name: BulkUpdateCKCAPAStatus :execrows
UPDATE ck_capas SET
  status     = sqlc.arg('status')::text,
  closed_at  = CASE WHEN sqlc.arg('status')::text = 'closed' AND closed_at IS NULL THEN NOW() ELSE closed_at END,
  updated_at = NOW()
WHERE org_id = $1 AND id = ANY(sqlc.arg('ids')::uuid[]);

-- ── Control Measures (Maßnahmen-Katalog, TISAX) ─────────────────────────────

-- name: ListCKMeasures :many
SELECT id, control_id, org_id, title, description, difficulty,
       step_order, is_builtin, created_at
FROM ck_control_measures
WHERE org_id = $1 AND control_id = $2
ORDER BY step_order ASC, created_at ASC;

-- name: CreateCKMeasure :one
INSERT INTO ck_control_measures (control_id, org_id, title, description, difficulty, step_order, is_builtin)
VALUES ($1, $2, $3, $4, $5, $6, FALSE)
RETURNING id, control_id, org_id, title, description, difficulty,
          step_order, is_builtin, created_at;

-- name: UpdateCKMeasure :one
UPDATE ck_control_measures SET
  title       = COALESCE(sqlc.narg('title')::text, title),
  description = COALESCE(sqlc.narg('description')::text, description),
  difficulty  = COALESCE(sqlc.narg('difficulty')::text, difficulty),
  step_order  = COALESCE(sqlc.narg('step_order')::int, step_order)
WHERE id = $1 AND org_id = $2
RETURNING id, control_id, org_id, title, description, difficulty,
          step_order, is_builtin, created_at;

-- name: DeleteCKMeasure :execrows
DELETE FROM ck_control_measures
WHERE id = $1 AND org_id = $2 AND is_builtin = FALSE;

-- name: SeedCKMeasure :exec
-- Idempotent (ON CONFLICT DO NOTHING). Aufrufer iteriert die Liste; jedes
-- Measure wird als is_builtin=TRUE eingefügt.
INSERT INTO ck_control_measures (control_id, org_id, title, description, difficulty, step_order, is_builtin)
VALUES ($1, $2, $3, $4, $5, $6, TRUE)
ON CONFLICT DO NOTHING;

-- ── Suppliers (NIS2 Art. 21 / DORA Art. 28) ─────────────────────────────────

-- name: CreateCKSupplier :one
INSERT INTO ck_suppliers (
    org_id, name, contact_name, contact_email, service_type, criticality,
    nis2_relevant, dora_relevant, contract_end, notes, sub_suppliers,
    data_location, exit_strategy_exists, assessment_status, last_assessment_at)
VALUES ($1, $2,
        NULLIF(sqlc.arg('contact_name')::text, ''),
        NULLIF(sqlc.arg('contact_email')::text, ''),
        NULLIF(sqlc.arg('service_type')::text, ''),
        $3, $4, $5, $6,
        NULLIF(sqlc.arg('notes')::text, ''),
        $7,
        NULLIF(sqlc.arg('data_location')::text, ''),
        $8, $9, $10)
RETURNING id, org_id, name, contact_name, contact_email, service_type,
          criticality, nis2_relevant, dora_relevant, contract_end, notes,
          sub_suppliers, data_location, exit_strategy_exists,
          assessment_status, last_assessment_at, created_at, updated_at;

-- name: GetCKSupplier :one
SELECT id, org_id, name, contact_name, contact_email, service_type,
       criticality, nis2_relevant, dora_relevant, contract_end, notes,
       sub_suppliers, data_location, exit_strategy_exists,
       assessment_status, last_assessment_at, created_at, updated_at
FROM ck_suppliers
WHERE id = $1 AND org_id = $2;

-- name: ListCKSuppliers :many
SELECT id, org_id, name, contact_name, contact_email, service_type,
       criticality, nis2_relevant, dora_relevant, contract_end, notes,
       sub_suppliers, data_location, exit_strategy_exists,
       assessment_status, last_assessment_at, created_at, updated_at
FROM ck_suppliers
WHERE org_id = $1
  AND (sqlc.narg('criticality')::text       IS NULL OR criticality       = sqlc.narg('criticality')::text)
  AND (sqlc.narg('assessment_status')::text IS NULL OR assessment_status = sqlc.narg('assessment_status')::text)
ORDER BY name ASC
LIMIT 1000;

-- name: UpdateCKSupplier :one
UPDATE ck_suppliers SET
  name                 = $3,
  contact_name         = NULLIF(sqlc.arg('contact_name')::text, ''),
  contact_email        = NULLIF(sqlc.arg('contact_email')::text, ''),
  service_type         = NULLIF(sqlc.arg('service_type')::text, ''),
  criticality          = $4,
  nis2_relevant        = $5,
  dora_relevant        = $6,
  contract_end         = $7,
  notes                = NULLIF(sqlc.arg('notes')::text, ''),
  sub_suppliers        = $8,
  data_location        = NULLIF(sqlc.arg('data_location')::text, ''),
  exit_strategy_exists = $9,
  assessment_status    = $10,
  last_assessment_at   = $11,
  updated_at           = NOW()
WHERE id = $1 AND org_id = $2
RETURNING id, org_id, name, contact_name, contact_email, service_type,
          criticality, nis2_relevant, dora_relevant, contract_end, notes,
          sub_suppliers, data_location, exit_strategy_exists,
          assessment_status, last_assessment_at, created_at, updated_at;

-- name: DeleteCKSupplier :execrows
DELETE FROM ck_suppliers WHERE id = $1 AND org_id = $2;

-- ── AI Systems (EU AI Act) ──────────────────────────────────────────────────

-- name: CreateCKAISystem :one
INSERT INTO ck_ai_systems (
    org_id, name, description, provider, use_case, affected_groups,
    autonomy_level, in_production_since, risk_class, classification_rationale)
VALUES ($1, $2,
        NULLIF(sqlc.arg('description')::text, ''),
        NULLIF(sqlc.arg('provider')::text, ''),
        NULLIF(sqlc.arg('use_case')::text, ''),
        NULLIF(sqlc.arg('affected_groups')::text, ''),
        $3, $4,
        NULLIF(sqlc.arg('risk_class')::text, ''),
        NULLIF(sqlc.arg('classification_rationale')::text, ''))
RETURNING id, org_id, name, description, provider, use_case, affected_groups,
          autonomy_level, in_production_since, status, risk_class,
          classification_rationale, classified_at, classified_by,
          created_at, updated_at;

-- name: ListCKAISystems :many
SELECT id, org_id, name, description, provider, use_case, affected_groups,
       autonomy_level, in_production_since, status, risk_class,
       classification_rationale, classified_at, classified_by,
       created_at, updated_at
FROM ck_ai_systems
WHERE org_id = $1
  AND (sqlc.narg('risk_class')::text IS NULL OR risk_class = sqlc.narg('risk_class')::text)
  AND (sqlc.narg('status')::text     IS NULL OR status     = sqlc.narg('status')::text)
ORDER BY name ASC
LIMIT 1000;

-- name: GetCKAISystem :one
SELECT id, org_id, name, description, provider, use_case, affected_groups,
       autonomy_level, in_production_since, status, risk_class,
       classification_rationale, classified_at, classified_by,
       created_at, updated_at
FROM ck_ai_systems
WHERE id = $1 AND org_id = $2;

-- name: UpdateCKAISystem :one
UPDATE ck_ai_systems SET
  name                     = $3,
  description              = NULLIF(sqlc.arg('description')::text, ''),
  provider                 = NULLIF(sqlc.arg('provider')::text, ''),
  use_case                 = NULLIF(sqlc.arg('use_case')::text, ''),
  affected_groups          = NULLIF(sqlc.arg('affected_groups')::text, ''),
  autonomy_level           = $4,
  in_production_since      = $5,
  status                   = $6,
  risk_class               = NULLIF(sqlc.arg('risk_class')::text, ''),
  classification_rationale = NULLIF(sqlc.arg('classification_rationale')::text, ''),
  classified_at            = COALESCE(sqlc.narg('classified_at')::timestamptz, classified_at),
  classified_by            = NULLIF(sqlc.arg('classified_by')::text, ''),
  updated_at               = NOW()
WHERE id = $1 AND org_id = $2
RETURNING id, org_id, name, description, provider, use_case, affected_groups,
          autonomy_level, in_production_since, status, risk_class,
          classification_rationale, classified_at, classified_by,
          created_at, updated_at;

-- name: DeleteCKAISystem :execrows
DELETE FROM ck_ai_systems WHERE id = $1 AND org_id = $2;

-- name: UpdateCKAISystemClassification :execrows
UPDATE ck_ai_systems SET
  risk_class               = $3,
  classification_rationale = $4,
  classified_by            = $5,
  classified_at            = NOW(),
  status                   = 'approved',
  updated_at               = NOW()
WHERE id = $1 AND org_id = $2;

-- ── Auditor Links ───────────────────────────────────────────────────────────

-- name: CreateCKAuditorLink :one
INSERT INTO ck_auditor_links (org_id, framework_id, token_hash, created_by, expires_at, max_uses)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, org_id, framework_id, created_by, expires_at, used_count, max_uses, created_at;

-- name: GetCKAuditorLinkByHash :one
SELECT id, org_id, framework_id, created_by, expires_at, used_count, max_uses,
       created_at, revoked_at
FROM ck_auditor_links
WHERE token_hash = $1;

-- name: GetCKAuditorLinkByID :one
SELECT id, org_id, framework_id, label, created_by, expires_at,
       last_accessed_at, access_count, revoked_at, created_at
FROM ck_auditor_links
WHERE id = $1 AND org_id = $2;

-- name: ListCKAuditorLinks :many
SELECT id, org_id, framework_id, label, created_by, expires_at,
       last_accessed_at, access_count, revoked_at, created_at
FROM ck_auditor_links
WHERE org_id = $1
ORDER BY created_at DESC;

-- name: RevokeCKAuditorLink :execrows
UPDATE ck_auditor_links
SET revoked_at = NOW()
WHERE id = $1 AND org_id = $2 AND revoked_at IS NULL;

-- name: UpdateCKAuditorLinkAccess :exec
UPDATE ck_auditor_links
SET access_count = access_count + 1,
    last_accessed_at = NOW()
WHERE id = $1;

-- name: IncrementCKAuditorLinkUsage :exec
UPDATE ck_auditor_links
SET used_count = used_count + 1
WHERE id = $1;

-- ── Resilience Tests (DORA Art. 24-27) ─────────────────────────────────────

-- name: CreateCKResilienceTest :one
INSERT INTO ck_resilience_tests (org_id, type, scope, provider, test_date, summary, remediation_status)
VALUES ($1, $2,
        NULLIF(sqlc.arg('scope')::text, ''),
        NULLIF(sqlc.arg('provider')::text, ''),
        $3,
        NULLIF(sqlc.arg('summary')::text, ''),
        $4)
RETURNING id, org_id, type, scope, provider, test_date, summary,
          remediation_status, attachment_url, created_at, updated_at;

-- name: ListCKResilienceTests :many
SELECT id, org_id, type, scope, provider, test_date, summary,
       remediation_status, attachment_url, created_at, updated_at
FROM ck_resilience_tests
WHERE org_id = $1
ORDER BY test_date DESC;

-- name: GetCKResilienceTest :one
SELECT id, org_id, type, scope, provider, test_date, summary,
       remediation_status, attachment_url, created_at, updated_at
FROM ck_resilience_tests
WHERE id = $1 AND org_id = $2;

-- name: UpdateCKResilienceTest :one
UPDATE ck_resilience_tests SET
  type               = $3,
  scope              = NULLIF(sqlc.arg('scope')::text, ''),
  provider           = NULLIF(sqlc.arg('provider')::text, ''),
  test_date          = $4,
  summary            = NULLIF(sqlc.arg('summary')::text, ''),
  remediation_status = $5,
  updated_at         = NOW()
WHERE id = $1 AND org_id = $2
RETURNING id, org_id, type, scope, provider, test_date, summary,
          remediation_status, attachment_url, created_at, updated_at;

-- name: DeleteCKResilienceTest :execrows
DELETE FROM ck_resilience_tests WHERE id = $1 AND org_id = $2;

-- name: UpdateCKResilienceTestAttachment :execrows
UPDATE ck_resilience_tests SET attachment_url = $3, updated_at = NOW()
WHERE id = $1 AND org_id = $2;

-- ── Audit Records ───────────────────────────────────────────────────────────

-- name: CreateCKAuditRecord :one
INSERT INTO ck_audit_records (org_id, title, scope, auditor, audit_date, findings, recommendations)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, org_id, title, scope, auditor, audit_date, status,
          findings, recommendations, created_at, updated_at;

-- name: ListCKAuditRecords :many
SELECT id, org_id, title, scope, auditor, audit_date, status,
       findings, recommendations, created_at, updated_at
FROM ck_audit_records
WHERE org_id = $1
ORDER BY audit_date DESC
LIMIT 1000;

-- name: GetCKAuditRecord :one
SELECT id, org_id, title, scope, auditor, audit_date, status,
       findings, recommendations, created_at, updated_at
FROM ck_audit_records
WHERE id = $1 AND org_id = $2;

-- name: UpdateCKAuditRecord :one
UPDATE ck_audit_records SET
  title           = $3,
  scope           = $4,
  auditor         = $5,
  audit_date      = $6,
  status          = $7,
  findings        = $8,
  recommendations = $9,
  updated_at      = NOW()
WHERE id = $1 AND org_id = $2
RETURNING id, org_id, title, scope, auditor, audit_date, status,
          findings, recommendations, created_at, updated_at;

-- ── Incidents ───────────────────────────────────────────────────────────────

-- name: CreateCKIncident :one
INSERT INTO ck_incidents (
    org_id, title, description, severity, discovered_at, affected_systems, breach_id,
    incident_type, reporting_obligation, notification_authority,
    deadline_4h, deadline_24h, deadline_72h, deadline_30d,
    affected_customers, financial_impact_estimate, is_major_incident
) VALUES ($1, $2, $3, $4, $5, $6, $7,
          $8, $9, $10,
          $11, $12, $13, $14,
          $15, $16, $17)
RETURNING id, org_id, title, description, severity, status,
          discovered_at, resolved_at, affected_systems, breach_id,
          incident_type, reporting_obligation, notification_authority,
          deadline_4h, deadline_24h, deadline_72h, deadline_30d,
          reported_4h_at, reported_24h_at, reported_72h_at, reported_30d_at,
          affected_customers, financial_impact_estimate, is_major_incident,
          supplier_id,
          notified_warn_24h, notified_warn_72h, notified_warn_30d,
          created_at, updated_at;

-- name: UpdateCKIncident :one
-- resolved_at-Logik:
--   - alter Status resolved/closed → behalten
--   - alter Status nicht, neuer ist  → NOW()
--   - alter Status nicht, neuer auch nicht → NULL
UPDATE ck_incidents SET
  title                     = $3,
  description               = $4,
  severity                  = $5,
  status                    = $6,
  affected_systems          = $7,
  resolved_at               = CASE
                               WHEN ck_incidents.status IN ('resolved','closed') THEN resolved_at
                               WHEN $6::text             IN ('resolved','closed') THEN NOW()
                               ELSE NULL
                              END,
  incident_type             = $8,
  reporting_obligation      = $9,
  notification_authority    = $10,
  affected_customers        = $11,
  financial_impact_estimate = $12,
  is_major_incident         = $13,
  updated_at                = NOW()
WHERE id = $1 AND org_id = $2
RETURNING id, org_id, title, description, severity, status,
          discovered_at, resolved_at, affected_systems, breach_id,
          incident_type, reporting_obligation, notification_authority,
          deadline_4h, deadline_24h, deadline_72h, deadline_30d,
          reported_4h_at, reported_24h_at, reported_72h_at, reported_30d_at,
          affected_customers, financial_impact_estimate, is_major_incident,
          supplier_id,
          notified_warn_24h, notified_warn_72h, notified_warn_30d,
          created_at, updated_at;

-- name: GetCKIncident :one
SELECT id, org_id, title, description, severity, status,
       discovered_at, resolved_at, affected_systems, breach_id,
       incident_type, reporting_obligation, notification_authority,
       deadline_4h, deadline_24h, deadline_72h, deadline_30d,
       reported_4h_at, reported_24h_at, reported_72h_at, reported_30d_at,
       affected_customers, financial_impact_estimate, is_major_incident,
       supplier_id,
       notified_warn_24h, notified_warn_72h, notified_warn_30d,
       created_at, updated_at
FROM ck_incidents
WHERE id = $1 AND org_id = $2;

-- name: ListCKIncidents :many
SELECT id, org_id, title, description, severity, status,
       discovered_at, resolved_at, affected_systems, breach_id,
       incident_type, reporting_obligation, notification_authority,
       deadline_4h, deadline_24h, deadline_72h, deadline_30d,
       reported_4h_at, reported_24h_at, reported_72h_at, reported_30d_at,
       affected_customers, financial_impact_estimate, is_major_incident,
       supplier_id,
       notified_warn_24h, notified_warn_72h, notified_warn_30d,
       created_at, updated_at
FROM ck_incidents
WHERE org_id = $1
ORDER BY discovered_at DESC;

-- name: ListCKIncidentsByType :many
SELECT id, org_id, title, description, severity, status,
       discovered_at, resolved_at, affected_systems, breach_id,
       incident_type, reporting_obligation, notification_authority,
       deadline_4h, deadline_24h, deadline_72h, deadline_30d,
       reported_4h_at, reported_24h_at, reported_72h_at, reported_30d_at,
       affected_customers, financial_impact_estimate, is_major_incident,
       supplier_id,
       notified_warn_24h, notified_warn_72h, notified_warn_30d,
       created_at, updated_at
FROM ck_incidents
WHERE org_id = $1 AND incident_type = $2 AND status NOT IN ('closed')
ORDER BY discovered_at DESC;

-- name: ListCKIncidentsBySupplier :many
SELECT id, org_id, title, description, severity, status,
       discovered_at, resolved_at, affected_systems, breach_id,
       incident_type, reporting_obligation, notification_authority,
       deadline_4h, deadline_24h, deadline_72h, deadline_30d,
       reported_4h_at, reported_24h_at, reported_72h_at, reported_30d_at,
       affected_customers, financial_impact_estimate, is_major_incident,
       supplier_id,
       notified_warn_24h, notified_warn_72h, notified_warn_30d,
       created_at, updated_at
FROM ck_incidents
WHERE org_id = $1 AND supplier_id = $2
ORDER BY discovered_at DESC;

-- name: CountCKIncidents :one
SELECT COUNT(*) FROM ck_incidents WHERE org_id = $1;

-- name: ListCKIncidentsPaged :many
SELECT id, org_id, title, description, severity, status,
       discovered_at, resolved_at, affected_systems, breach_id,
       incident_type, reporting_obligation, notification_authority,
       deadline_4h, deadline_24h, deadline_72h, deadline_30d,
       reported_4h_at, reported_24h_at, reported_72h_at, reported_30d_at,
       affected_customers, financial_impact_estimate, is_major_incident,
       supplier_id,
       notified_warn_24h, notified_warn_72h, notified_warn_30d,
       created_at, updated_at
FROM ck_incidents
WHERE org_id = $1
ORDER BY discovered_at DESC
LIMIT $2 OFFSET $3;

-- name: MarkCKIncidentDeadlineReported :one
-- Aufrufer wählt die Deadline-Spalte über sqlc.arg('deadline'). Nur die Spalte
-- des passenden Strings wird auf NOW() gesetzt; andere bleiben unverändert.
UPDATE ck_incidents SET
  reported_4h_at  = CASE WHEN sqlc.arg('deadline')::text = '4h'  THEN NOW() ELSE reported_4h_at  END,
  reported_24h_at = CASE WHEN sqlc.arg('deadline')::text = '24h' THEN NOW() ELSE reported_24h_at END,
  reported_72h_at = CASE WHEN sqlc.arg('deadline')::text = '72h' THEN NOW() ELSE reported_72h_at END,
  reported_30d_at = CASE WHEN sqlc.arg('deadline')::text = '30d' THEN NOW() ELSE reported_30d_at END,
  updated_at      = NOW()
WHERE id = $1 AND org_id = $2
RETURNING id, org_id, title, description, severity, status,
          discovered_at, resolved_at, affected_systems, breach_id,
          incident_type, reporting_obligation, notification_authority,
          deadline_4h, deadline_24h, deadline_72h, deadline_30d,
          reported_4h_at, reported_24h_at, reported_72h_at, reported_30d_at,
          affected_customers, financial_impact_estimate, is_major_incident,
          supplier_id,
          notified_warn_24h, notified_warn_72h, notified_warn_30d,
          created_at, updated_at;

-- name: MarkCKIncidentWarnNotified :exec
-- Analog zu MarkDeadlineReported, aber für die Warning-Flags.
UPDATE ck_incidents SET
  notified_warn_24h = CASE WHEN sqlc.arg('deadline')::text = '24h' THEN TRUE ELSE notified_warn_24h END,
  notified_warn_72h = CASE WHEN sqlc.arg('deadline')::text = '72h' THEN TRUE ELSE notified_warn_72h END,
  notified_warn_30d = CASE WHEN sqlc.arg('deadline')::text = '30d' THEN TRUE ELSE notified_warn_30d END,
  updated_at        = NOW()
WHERE id = $1 AND org_id = $2;

-- name: UpdateCKIncidentReportability :exec
UPDATE ck_incidents SET
  reporting_obligation       = $3,
  notification_authority     = $4,
  gdpr_notification_required = $5,
  reportability_answers      = $6,
  updated_at                 = NOW()
WHERE id = $1 AND org_id = $2;

-- ── Incident-Reports (PDFs) ─────────────────────────────────────────────────

-- name: SaveCKIncidentReport :one
INSERT INTO ck_incident_reports (org_id, incident_id, report_type, authority, pdf_data, metadata)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, org_id, incident_id, report_type, authority, generated_at;

-- name: ListCKIncidentReports :many
SELECT id, org_id, incident_id, report_type, authority, generated_at
FROM ck_incident_reports
WHERE org_id = $1 AND incident_id = $2
ORDER BY generated_at DESC;

-- name: GetCKIncidentReportPDF :one
SELECT pdf_data FROM ck_incident_reports
WHERE id = $1 AND org_id = $2;

-- name: ListCKRiskControls :many
-- Liefert Controls (volle Spaltenliste — consistent mit ListCKControls/GetCKControl),
-- die mit dem gegebenen Risk verknüpft sind.
SELECT c.id, c.framework_id, c.org_id, c.control_id, c.title,
       c.description, c.domain, c.evidence_type, c.weight,
       c.not_applicable, c.not_applicable_reason,
       c.manual_status, c.maturity_score, c.owner,
       c.last_reviewed_at, c.review_interval_days, c.next_review_due,
       c.last_reviewed_by, c.review_note, c.due_date
FROM ck_controls c
JOIN ck_risk_control_links l ON l.control_id = c.id AND l.org_id = c.org_id
WHERE l.risk_id = $1 AND l.org_id = $2
ORDER BY c.control_id ASC;

-- ── Control extras (Reviews + Overdue + Pagination) ───────────────────────

-- name: ListCKControlReviews :many
SELECT id, control_id, reviewed_by, review_note, status_at_review, reviewed_at
FROM ck_control_reviews
WHERE control_id = $1 AND org_id = $2
ORDER BY reviewed_at DESC;

-- name: ListCKOverdueControls :many
SELECT id, framework_id, org_id, control_id, title, description, domain,
       evidence_type, weight, not_applicable, not_applicable_reason,
       manual_status, maturity_score, owner,
       last_reviewed_at, review_interval_days, next_review_due,
       last_reviewed_by, review_note, due_date
FROM ck_controls
WHERE org_id = $1
  AND next_review_due IS NOT NULL
  AND next_review_due < NOW()
ORDER BY next_review_due ASC;

-- name: CountCKControls :one
SELECT COUNT(*) FROM ck_controls
WHERE framework_id = $1 AND org_id = $2;

-- name: ListCKControlsPaged :many
SELECT id, framework_id, org_id, control_id, title, description, domain,
       evidence_type, weight, not_applicable, not_applicable_reason,
       manual_status, maturity_score, owner,
       last_reviewed_at, review_interval_days, next_review_due,
       last_reviewed_by, review_note, due_date
FROM ck_controls
WHERE framework_id = $1 AND org_id = $2
ORDER BY control_id ASC
LIMIT $3 OFFSET $4;

-- ── Org Helpers (verwendet von vaktcomply; touchen organizations/users) ────
-- Auch wenn diese Tabellen nicht unter dem ck_-Prefix laufen, gehören sie zu
-- den verschiedenen Aggregaten, die der vaktcomply-Service benötigt
-- (Sector-Settings, Admin-Mailings, Cross-Org-Seeding).

-- name: ListAllOrgIDs :many
SELECT id FROM organizations ORDER BY created_at ASC;

-- name: GetCKOrgSector :one
SELECT COALESCE(sector, 'other') AS sector,
       COALESCE(federal_state, '') AS federal_state
FROM organizations
WHERE id = $1;

-- name: UpdateCKOrgSector :execrows
UPDATE organizations
SET sector = $2,
    federal_state = NULLIF(sqlc.arg('federal_state')::text, ''),
    updated_at = NOW()
WHERE id = $1;

-- name: GetCKOrgAdminEmails :many
SELECT u.email
FROM   org_members om
JOIN   users u  ON u.id  = om.user_id
JOIN   roles ro ON ro.id = om.role_id
WHERE  om.org_id = $1
  AND  ro.name   = 'Admin'
  AND  u.is_active = true;

-- ── Control Discovery (used by SecPulse auto-evidence + AI classifier) ────

-- name: FindCKPatchControls :many
SELECT id, framework_id, org_id, control_id, title, description, domain,
       evidence_type, weight, not_applicable, not_applicable_reason,
       manual_status, maturity_score, owner,
       last_reviewed_at, review_interval_days, next_review_due,
       last_reviewed_by, review_note, due_date
FROM ck_controls
WHERE org_id = $1
  AND (
    lower(title)  LIKE '%patch%'
    OR lower(title)  LIKE '%vulnerability%'
    OR lower(title)  LIKE '%update%'
    OR lower(domain) LIKE '%patch%'
  )
  AND not_applicable = false
LIMIT 10;

-- name: FindCKControlsByKeywords :many
-- Erwartet bereits %-umschlossene Keywords (z.B. "%firewall%").
SELECT id, framework_id, org_id, control_id, title, description, domain,
       evidence_type, weight, not_applicable, not_applicable_reason,
       manual_status, maturity_score, owner,
       last_reviewed_at, review_interval_days, next_review_due,
       last_reviewed_by, review_note, due_date
FROM ck_controls
WHERE org_id = $1
  AND not_applicable = false
  AND (lower(title) ILIKE ANY(sqlc.arg('patterns')::text[])
       OR lower(domain) ILIKE ANY(sqlc.arg('patterns')::text[]))
LIMIT 10;

-- name: FindCKControlByCode :one
SELECT id FROM ck_controls
WHERE org_id = $1 AND control_id = $2
LIMIT 1;

-- name: FindCKExpiringCerts :many
-- Sicherheitshinweis: sa.org_id war früher Bestandteil dieser Query, existiert
-- aber auf ck_supplier_answers nicht. Die Org-Eingrenzung läuft über den JOIN
-- auf ck_supplier_assessments.org_id.
SELECT sa.id AS answer_id, s.id AS supplier_id, s.name AS supplier_name,
       qq.question_text, sa.cert_expiry_date, sa.file_url
FROM ck_supplier_answers sa
JOIN ck_questionnaire_questions qq ON qq.id = sa.question_id
JOIN ck_supplier_assessments asm ON asm.id = sa.assessment_id
JOIN ck_suppliers s ON s.id = asm.supplier_id
WHERE asm.org_id = $1
  AND sa.cert_expiry_date IS NOT NULL
  AND sa.cert_expiry_date <= $2
  AND sa.file_url IS NOT NULL AND sa.file_url != '';

-- ── EU AI Act Stats (Aggregat fürs Dashboard) ────────────────────────────

-- name: ListCKAISystemsForStats :many
-- Minimale Projektion für GetEUAIActStats; nur risk_class + status.
SELECT COALESCE(risk_class, 'unclassified') AS risk_class, status
FROM ck_ai_systems
WHERE org_id = $1;

-- name: CountCKAISystemsWithoutDocs :one
SELECT COUNT(*) FROM ck_ai_systems s
WHERE s.org_id = $1
  AND NOT EXISTS (
    SELECT 1 FROM ck_ai_documentation d WHERE d.ai_system_id = s.id
  );

-- ── EU AI Act: Classifications + Documentation ─────────────────────────────

-- name: InsertCKAIClassification :one
INSERT INTO ck_ai_classifications
    (org_id, ai_system_id, risk_class, rationale, classified_by, wizard_answers)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id;

-- name: ListCKAIClassifications :many
SELECT id, org_id, ai_system_id, risk_class, rationale, classified_by,
       wizard_answers, classified_at
FROM ck_ai_classifications
WHERE org_id = $1 AND ai_system_id = $2
ORDER BY classified_at DESC;

-- name: NextCKAIDocumentationVersion :one
-- Returns MAX(version)+1 für die nächste Doku-Version (oder 1, wenn leer).
SELECT COALESCE(MAX(version), 0) + 1 AS next_version
FROM ck_ai_documentation
WHERE org_id = $1 AND ai_system_id = $2;

-- name: InsertCKAIDocumentation :one
INSERT INTO ck_ai_documentation
  (org_id, ai_system_id, version, system_description, intended_purpose,
   training_data, data_quality, performance_metrics, system_limits,
   risk_management, human_oversight, logging_audit_trail, authored_by, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING id, org_id, ai_system_id, version,
          system_description, intended_purpose, training_data, data_quality,
          performance_metrics, system_limits, risk_management, human_oversight,
          logging_audit_trail, authored_by, status, created_at, updated_at;

-- name: GetLatestCKAIDocumentation :one
SELECT id, org_id, ai_system_id, version,
       system_description, intended_purpose, training_data, data_quality,
       performance_metrics, system_limits, risk_management, human_oversight,
       logging_audit_trail, authored_by, status, created_at, updated_at
FROM ck_ai_documentation
WHERE org_id = $1 AND ai_system_id = $2
ORDER BY version DESC
LIMIT 1;

-- name: ListCKAIDocumentationVersions :many
SELECT id, org_id, ai_system_id, version,
       system_description, intended_purpose, training_data, data_quality,
       performance_metrics, system_limits, risk_management, human_oversight,
       logging_audit_trail, authored_by, status, created_at, updated_at
FROM ck_ai_documentation
WHERE org_id = $1 AND ai_system_id = $2
ORDER BY version DESC;

-- name: UpdateCKRiskTreatment :one
-- treatment_due_date semantics: Valid=true mit Time=zero → SQL NULL ist nicht
-- ausdrückbar. Aufrufer macht read-merge-write für „keep existing" Fall —
-- nil-Input wird vorab gegen die aktuelle Zeile gemappt. Hier wird die Spalte
-- IMMER überschrieben (Valid=false → NULL, Valid=true → der Wert).
UPDATE ck_risks SET
  treatment_option    = $3,
  treatment_plan      = $4,
  treatment_owner     = $5,
  treatment_status    = $6,
  residual_likelihood = $7,
  residual_impact     = $8,
  treatment_due_date  = $9,
  updated_at          = NOW()
WHERE id = $1 AND org_id = $2
RETURNING id, org_id, title, description, category,
          likelihood, impact, risk_score, owner, status, treatment, treatment_notes,
          treatment_option, treatment_plan, treatment_owner,
          treatment_due_date, treatment_status,
          residual_likelihood, residual_impact,
          created_at, updated_at;

-- ── SoA Applicability (S25-3) ────────────────────────────────────────────────

-- name: ListCKSoAEntries :many
SELECT
    c.id::text                                  AS control_id,
    f.name                                      AS framework_name,
    c.domain,
    c.title,
    COALESCE(c.soa_applicable, true)            AS applicable,
    COALESCE(c.manual_status, 'not_started')    AS status,
    COALESCE(c.soa_justification_yes, '')       AS just_yes,
    COALESCE(c.soa_justification_no,  '')       AS just_no
FROM ck_controls c
JOIN ck_frameworks f ON f.id = c.framework_id AND f.org_id = c.org_id
WHERE c.org_id = $1
ORDER BY f.name, c.domain, c.title;

-- name: UpdateCKSoAApplicability :exec
UPDATE ck_controls
SET soa_applicable        = $1,
    soa_justification_yes = $2,
    soa_justification_no  = $3
WHERE id = $4::uuid AND org_id = $5::uuid;

-- ── Org Member Role (S25-3) ──────────────────────────────────────────────────

-- name: GetCKOrgMemberRole :one
SELECT r.name AS role_name
FROM org_members om
JOIN roles r ON r.id = om.role_id
WHERE om.user_id = $1::uuid AND om.org_id = $2::uuid
LIMIT 1;

-- ── My Tasks (S25-3) ─────────────────────────────────────────────────────────

-- name: GetUserDisplayName :one
SELECT COALESCE(display_name, email) AS display_name
FROM users
WHERE id = $1::uuid;

-- name: ListCKMyTaskControls :many
SELECT id::text, title, COALESCE(manual_status, '') AS manual_status, framework_id::text
FROM ck_controls
WHERE org_id = $1::uuid
  AND owner = $2
  AND NOT not_applicable
ORDER BY control_id ASC
LIMIT 50;

-- name: ListCKMyTaskRisks :many
SELECT id::text, title, status
FROM ck_risks
WHERE org_id = $1::uuid
  AND owner = $2
  AND status NOT IN ('accepted', 'resolved')
ORDER BY created_at DESC
LIMIT 50;

-- ── Board Report + Executive Summary (s26-sqlc-vitals-4) ─────────────────────

-- name: GetBoardReportComplianceScoreRows :many
-- Liefert pro Framework: Gesamtanzahl Controls + Anzahl implemented Controls.
-- Für den gewichteten Score-Durchschnitt im Board-Report.
SELECT
    COUNT(c.id)::int                                                     AS total,
    COUNT(c.id) FILTER (WHERE c.manual_status = 'implemented')::int     AS implemented
FROM ck_frameworks f
LEFT JOIN ck_controls c ON c.framework_id = f.id AND c.org_id = f.org_id
WHERE f.org_id = $1::uuid
GROUP BY f.id;

-- name: GetCKPreviousScore :one
-- Letzter Score-Snapshot vor heute (für Board-Report Delta).
SELECT score FROM ck_score_history
WHERE org_id = $1::uuid AND recorded_at < NOW()::date
ORDER BY recorded_at DESC
LIMIT 1;

-- name: CountCKRecentIncidents :one
-- Anzahl Incidents der letzten 30 Tage für den Board-Report.
SELECT COUNT(*)::int FROM ck_incidents
WHERE org_id = $1::uuid AND created_at >= $2;

-- name: ListActiveOrgIDs :many
-- Alle nicht gelöschten Organisationen (für den täglichen Score-Snapshot-Job).
SELECT id::text FROM organizations;

-- name: GetExecutiveFrameworkScores :many
-- Pro Framework: Name, Gesamtanzahl Controls, Anzahl implemented.
-- Für Executive Summary PDF (Section 2 — Framework overview).
SELECT f.name,
       COUNT(c.id)::int                                                    AS total,
       COUNT(c.id) FILTER (WHERE c.manual_status = 'implemented')::int     AS implemented
FROM ck_frameworks f
LEFT JOIN ck_controls c ON c.framework_id = f.id AND c.org_id = f.org_id
WHERE f.org_id = $1::uuid
GROUP BY f.name
ORDER BY f.name;

-- name: GetExecutiveTopRisks :many
-- Top-5 offene Risiken nach Score (likelihood * impact) für Executive Summary.
SELECT title,
       (likelihood * impact)::int AS score,
       CASE
           WHEN (likelihood * impact) >= 15 THEN 'critical'
           WHEN (likelihood * impact) >= 9  THEN 'high'
           WHEN (likelihood * impact) >= 4  THEN 'medium'
           ELSE 'low'
       END AS severity
FROM ck_risks
WHERE org_id = $1::uuid AND status = 'open'
ORDER BY score DESC, updated_at DESC
LIMIT 5;

-- name: CountCKClosedControlsSince :one
-- Anzahl auf 'implemented' gesetzter Controls seit einem Zeitpunkt.
SELECT COUNT(*)::int FROM ck_controls
WHERE org_id = $1::uuid AND manual_status = 'implemented' AND updated_at >= $2;

-- name: CountCKIncidentsSince :one
-- Anzahl neu eröffneter Incidents seit einem Zeitpunkt.
SELECT COUNT(*)::int FROM ck_incidents
WHERE org_id = $1::uuid AND created_at >= $2;

-- name: CountSPResolvedFindingsSince :one
-- Anzahl auf 'resolved' gesetzter Findings seit einem Zeitpunkt.
SELECT COUNT(*)::int FROM vb_findings
WHERE org_id = $1::uuid AND status = 'resolved' AND updated_at >= $2;

-- ── iCal Deadlines (handler_ical.go — s26-sqlc-vitals-3) ────────────────────

-- name: ListCKICalMilestones :many
SELECT id::text, title, COALESCE(description, '') AS description, milestone_date
FROM ck_audit_milestones
WHERE org_id = $1::uuid
  AND status IN ('upcoming')
  AND milestone_date >= CURRENT_DATE
ORDER BY milestone_date
LIMIT 100;

-- name: ListCKICalCAPAs :many
SELECT id::text, title, due_date
FROM ck_capas
WHERE org_id = $1::uuid
  AND due_date IS NOT NULL
  AND status IN ('open', 'in_progress')
ORDER BY due_date
LIMIT 100;

-- name: ListCKICalExpiringEvidence :many
SELECT e.id::text, e.title, e.expires_at
FROM ck_evidence e
WHERE e.org_id = $1::uuid
  AND e.expires_at IS NOT NULL
  AND e.expires_at > NOW()
ORDER BY e.expires_at
LIMIT 50;

-- ── Policy Templates (handler_templates.go — s26-sqlc-vitals-3) ──────────────

-- name: ListCKPolicyTemplates :many
-- Optional category filter via sqlc.narg; NULL means no filter (return all).
SELECT id::text, category, name, description, content, tags,
       COALESCE(framework, '') AS framework, created_at::text AS created_at
FROM ck_policy_templates
WHERE (sqlc.narg('category')::text IS NULL OR category = sqlc.narg('category')::text)
ORDER BY category, name;

-- name: GetCKPolicyTemplateByID :one
SELECT id::text, category, name, description, content, tags,
       COALESCE(framework, '') AS framework, created_at::text AS created_at
FROM ck_policy_templates
WHERE id = $1::uuid;

-- ── Policy AI Generator context (service_policies.go — s26-sqlc-vitals-3) ────

-- name: ListCKTopControlsByFramework :many
-- Top-10 controls for a framework ordered by weight DESC.
-- Used by GeneratePolicyDraft to provide framework context to the AI prompt.
SELECT control_id, title
FROM ck_controls
WHERE framework_id = $1::uuid AND org_id = $2::uuid
ORDER BY weight DESC
LIMIT 10;

-- ── Framework Milestone Dedup (service_frameworks.go — s26-sqlc-vitals-3) ────

-- name: CountCKFrameworkMilestoneNotifs :one
-- Checks whether a milestone notification has already been sent for a given
-- deduplication key stored in user_notifications.module as "<frameworkID>:<threshold>".
SELECT COUNT(*) AS cnt
FROM user_notifications
WHERE org_id = $1::uuid
  AND type   = 'framework_milestone'
  AND module = $2;

-- ── Policy Acceptance (s26-sqlc-vitals-5 — DSGVO-sensitiv) ──────────────────

-- name: GetCKOrgName :one
-- Liest den Org-Namen für Akzeptanz-Kampagnen-Emails.
SELECT name FROM organizations WHERE id = $1::uuid;

-- name: CreateCKPolicyAcceptanceCampaign :one
INSERT INTO ck_policy_acceptance_campaigns (org_id, policy_id, name, message, deadline, created_by)
VALUES ($1::uuid, $2::uuid, $3, $4, $5::date, $6::uuid)
RETURNING id::text, org_id::text, policy_id::text, name,
          COALESCE(message, '') AS message,
          deadline::text,
          created_at;

-- name: ListCKPolicyAcceptanceCampaigns :many
SELECT id::text, org_id::text, policy_id::text, name,
       COALESCE(message, '') AS message,
       deadline::text,
       created_at
FROM ck_policy_acceptance_campaigns
WHERE org_id = $1::uuid AND policy_id = $2::uuid
ORDER BY created_at DESC;

-- name: CreateCKPolicyAcceptanceRequest :one
-- Legt einen Akzeptanz-Request an und gibt die neue UUID zurück.
INSERT INTO ck_policy_acceptance_requests (campaign_id, org_id, recipient_email, recipient_name, token_hash)
VALUES ($1::uuid, $2::uuid, $3, $4, $5)
RETURNING id::text;

-- name: MarkCKPolicyAcceptanceRequestSent :exec
UPDATE ck_policy_acceptance_requests SET sent_at = now() WHERE id = $1::uuid;

-- name: GetCKPolicyAcceptanceCampaignStats :one
SELECT
    COUNT(*)::int                          AS total,
    COUNT(accepted_at)::int                AS accepted,
    (COUNT(*) - COUNT(accepted_at))::int   AS pending
FROM ck_policy_acceptance_requests
WHERE campaign_id = $1::uuid;

-- name: ListCKPolicyAcceptanceRequests :many
SELECT id::text, campaign_id::text, recipient_email,
       COALESCE(recipient_name, '') AS recipient_name,
       accepted_at, sent_at, created_at
FROM ck_policy_acceptance_requests
WHERE campaign_id = $1::uuid
ORDER BY created_at ASC;

-- name: GetCKPolicyAcceptanceRequestByToken :one
-- Liest Request + Policy-Titel + Org-ID für den Token-basierten Accept-Flow.
SELECT
    par.id::text            AS id,
    par.campaign_id::text   AS campaign_id,
    par.recipient_email,
    COALESCE(par.recipient_name, '') AS recipient_name,
    par.accepted_at,
    par.sent_at,
    par.created_at,
    p.title                 AS policy_title,
    pac.org_id::text        AS org_id
FROM ck_policy_acceptance_requests par
JOIN ck_policy_acceptance_campaigns pac ON pac.id = par.campaign_id
JOIN ck_policies p ON p.id = pac.policy_id
WHERE par.token_hash = $1;

-- name: RecordCKPolicyAcceptance :execrows
-- Setzt accepted_at und accepted_ip. WHERE accepted_at IS NULL stellt
-- Idempotenz sicher (0 RowsAffected = bereits akzeptiert, kein Fehler im Aufrufer).
UPDATE ck_policy_acceptance_requests
SET accepted_at = now(), accepted_ip = $2
WHERE id = $1::uuid AND accepted_at IS NULL;

-- name: GetCKPolicyAcceptancePublicInfo :one
-- Öffentliche Info für das Accept-Portal (Token-basiert, kein Auth).
SELECT
    p.title                     AS policy_title,
    o.name                      AS org_name,
    COALESCE(pac.message, '')   AS message,
    pac.deadline::text          AS deadline,
    par.accepted_at
FROM ck_policy_acceptance_requests par
JOIN ck_policy_acceptance_campaigns pac ON pac.id = par.campaign_id
JOIN ck_policies p ON p.id = pac.policy_id
JOIN organizations o ON o.id = pac.org_id
WHERE par.token_hash = $1;

-- ── CI Evidence (from Vakt Scan CI webhook) ──────────────────────────────────

-- name: InsertCKCIEvidence :one
INSERT INTO ck_evidence
    (org_id, control_id, title, description, source, status,
     auto_source_type, auto_source_ref, auto_collected_at, collector_data)
VALUES
    ($1::uuid, NULL, $2, $3, 'ci_webhook', 'pending',
     'ci_webhook', $4, $5, $6::jsonb)
RETURNING id::text;


-- ── BCP / Notfallhandbuch (S60) ──────────────────────────────────────────────

-- name: CreateCKBCPPlan :one
INSERT INTO ck_bcp_plans (org_id, title, scope, version, status, owner)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListCKBCPPlans :many
SELECT * FROM ck_bcp_plans
WHERE org_id = $1
ORDER BY created_at DESC;

-- name: GetCKBCPPlan :one
SELECT * FROM ck_bcp_plans
WHERE id = $1 AND org_id = $2;

-- name: UpdateCKBCPPlan :one
UPDATE ck_bcp_plans
SET title = $3, scope = $4, version = $5, status = $6, owner = $7, updated_at = NOW()
WHERE id = $1 AND org_id = $2
RETURNING *;

-- name: DeleteCKBCPPlan :execrows
DELETE FROM ck_bcp_plans
WHERE id = $1 AND org_id = $2;

-- name: CreateCKBCPTest :one
INSERT INTO ck_bcp_tests (org_id, plan_id, test_date, test_type, outcome, findings)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListCKBCPTests :many
SELECT * FROM ck_bcp_tests
WHERE plan_id = $1 AND org_id = $2
ORDER BY test_date DESC;

-- name: GetLatestCKBCPTest :one
SELECT * FROM ck_bcp_tests
WHERE plan_id = $1 AND org_id = $2
ORDER BY test_date DESC
LIMIT 1;

-- ── Schutzbedarfsfeststellung (S60) ──────────────────────────────────────────

-- name: CreateCKProtectionNeedAssessment :one
INSERT INTO ck_protection_need_assessments (org_id, name, object_type, object_name)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListCKProtectionNeedAssessments :many
SELECT * FROM ck_protection_need_assessments
WHERE org_id = $1
ORDER BY created_at DESC;

-- name: GetCKProtectionNeedAssessment :one
SELECT * FROM ck_protection_need_assessments
WHERE id = $1 AND org_id = $2;

-- name: UpdateCKProtectionNeedAssessment :one
UPDATE ck_protection_need_assessments
SET confidentiality = $3, integrity = $4, availability = $5, overall = $6, updated_at = NOW()
WHERE id = $1 AND org_id = $2
RETURNING *;

-- name: FinalizeCKProtectionNeedAssessment :one
UPDATE ck_protection_need_assessments
SET status = 'finalized', finalized_at = NOW(), updated_at = NOW()
WHERE id = $1 AND org_id = $2
RETURNING *;

-- name: DeleteCKProtectionNeedAssessment :execrows
DELETE FROM ck_protection_need_assessments
WHERE id = $1 AND org_id = $2;

