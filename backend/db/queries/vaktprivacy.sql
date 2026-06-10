-- SecPrivacy queries — sqlc migration started in v0.6.x (ADR-0005, inkrementell).
--
-- Migrationsstand:
--   ✅ po_processing_activities — sqlc (this file) [renamed from po_processing_activities in migration 193]
--   ✅ po_dpias       — sqlc (this file)
--   ✅ po_avvs        — sqlc (this file)
--   ⏳ po_breaches    — embedded SQL
--   ⏳ po_dsr         — embedded SQL (Portal-Endpoints + DTO-Filtering)
--
-- po_dsr ist als Letztes geplant (öffentliches Portal + DSGVO-sensitive DTO).

-- ── VVT (Verzeichnis von Verarbeitungstätigkeiten) — Art. 30 DSGVO ──────────

-- name: ListPPVVT :many
SELECT id, org_id, name, purpose, legal_basis,
       data_categories, data_subjects, recipients,
       retention_period, third_country_transfer,
       safeguards, responsible_person,
       status, created_at, updated_at
FROM po_processing_activities
WHERE org_id = $1
ORDER BY created_at DESC;

-- name: GetPPVVT :one
SELECT id, org_id, name, purpose, legal_basis,
       data_categories, data_subjects, recipients,
       retention_period, third_country_transfer,
       safeguards, responsible_person,
       status, created_at, updated_at
FROM po_processing_activities
WHERE id = $1 AND org_id = $2;

-- name: CreatePPVVT :one
INSERT INTO po_processing_activities
  (org_id, name, purpose, legal_basis, data_categories, data_subjects,
   recipients, retention_period, third_country_transfer, safeguards, responsible_person)
VALUES
  ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING id, org_id, name, purpose, legal_basis,
          data_categories, data_subjects, recipients,
          retention_period, third_country_transfer,
          safeguards, responsible_person,
          status, created_at, updated_at;

-- name: UpdatePPVVT :one
UPDATE po_processing_activities SET
  name = $3, purpose = $4, legal_basis = $5,
  data_categories = $6, data_subjects = $7, recipients = $8,
  retention_period = $9, third_country_transfer = $10,
  safeguards = $11, responsible_person = $12, status = $13,
  updated_at = NOW()
WHERE id = $1 AND org_id = $2
RETURNING id, org_id, name, purpose, legal_basis,
          data_categories, data_subjects, recipients,
          retention_period, third_country_transfer,
          safeguards, responsible_person,
          status, created_at, updated_at;

-- name: DeletePPVVT :exec
DELETE FROM po_processing_activities WHERE id = $1 AND org_id = $2;

-- ── DPIA (Data Protection Impact Assessment) — Art. 35 DSGVO ────────────────

-- name: ListPPDPIAs :many
SELECT id, org_id, vvt_entry_id, title, description,
       necessity_assessment, risk_assessment, mitigation_measures,
       residual_risk, dpo_consultation, status,
       reviewed_by, reviewed_at, created_at, updated_at
FROM po_dpias
WHERE org_id = $1
ORDER BY created_at DESC
LIMIT 500;

-- name: GetPPDPIA :one
SELECT id, org_id, vvt_entry_id, title, description,
       necessity_assessment, risk_assessment, mitigation_measures,
       residual_risk, dpo_consultation, status,
       reviewed_by, reviewed_at, created_at, updated_at
FROM po_dpias
WHERE id = $1 AND org_id = $2;

-- name: CreatePPDPIA :one
INSERT INTO po_dpias
  (org_id, vvt_entry_id, title, description,
   necessity_assessment, risk_assessment, mitigation_measures,
   residual_risk, dpo_consultation)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, org_id, vvt_entry_id, title, description,
          necessity_assessment, risk_assessment, mitigation_measures,
          residual_risk, dpo_consultation, status,
          reviewed_by, reviewed_at, created_at, updated_at;

-- name: UpdatePPDPIA :one
-- vvt_entry_id is intentionally not in the SET list: the link to the originating
-- VVT entry is set at creation and never re-pointed by an update — DSGVO Art. 35
-- ties a DPIA to a specific processing activity. Use Delete + Create if the
-- target activity changes.
UPDATE po_dpias SET
  title               = $3,
  description         = $4,
  necessity_assessment = $5,
  risk_assessment     = $6,
  mitigation_measures = $7,
  residual_risk       = $8,
  dpo_consultation    = $9,
  updated_at          = NOW()
WHERE id = $1 AND org_id = $2
RETURNING id, org_id, vvt_entry_id, title, description,
          necessity_assessment, risk_assessment, mitigation_measures,
          residual_risk, dpo_consultation, status,
          reviewed_by, reviewed_at, created_at, updated_at;

-- name: ApprovePPDPIA :one
UPDATE po_dpias SET
  status      = 'approved',
  reviewed_by = $3,
  reviewed_at = NOW(),
  updated_at  = NOW()
WHERE id = $1 AND org_id = $2
RETURNING id, org_id, vvt_entry_id, title, description,
          necessity_assessment, risk_assessment, mitigation_measures,
          residual_risk, dpo_consultation, status,
          reviewed_by, reviewed_at, created_at, updated_at;

-- name: DeletePPDPIA :exec
DELETE FROM po_dpias WHERE id = $1 AND org_id = $2;

-- ── AVV (Auftragsverarbeitungsverträge) — Art. 28 DSGVO ─────────────────────

-- name: ListPPAVVs :many
SELECT id, org_id, processor_name, service_description,
       contract_date, review_date, status, notes,
       template_id, body, scc_module, scc_annex_i, scc_annex_ii, scc_annex_iii,
       created_at, updated_at
FROM po_avvs
WHERE org_id = $1
ORDER BY created_at DESC
LIMIT 500;

-- name: GetPPAVV :one
SELECT id, org_id, processor_name, service_description,
       contract_date, review_date, status, notes,
       template_id, body, scc_module, scc_annex_i, scc_annex_ii, scc_annex_iii,
       created_at, updated_at
FROM po_avvs
WHERE id = $1 AND org_id = $2;

-- name: CreatePPAVV :one
INSERT INTO po_avvs
  (org_id, processor_name, service_description, contract_date, review_date, notes)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, org_id, processor_name, service_description,
          contract_date, review_date, status, notes,
          template_id, body, scc_module, scc_annex_i, scc_annex_ii, scc_annex_iii,
          created_at, updated_at;

-- name: CreatePPAVVWithBody :one
INSERT INTO po_avvs
  (org_id, processor_name, service_description, template_id, body, status)
VALUES ($1, $2, $3, $4, $5, 'active')
RETURNING id, org_id, processor_name, service_description,
          contract_date, review_date, status, notes,
          template_id, body, scc_module, scc_annex_i, scc_annex_ii, scc_annex_iii,
          created_at, updated_at;

-- name: UpdatePPAVV :one
UPDATE po_avvs SET
  processor_name      = $3,
  service_description = $4,
  contract_date       = $5,
  review_date         = $6,
  status              = $7,
  notes               = $8,
  updated_at          = NOW()
WHERE id = $1 AND org_id = $2
RETURNING id, org_id, processor_name, service_description,
          contract_date, review_date, status, notes,
          template_id, body, scc_module, scc_annex_i, scc_annex_ii, scc_annex_iii,
          created_at, updated_at;

-- name: UpdatePPAVVBody :exec
UPDATE po_avvs SET
  template_id = $3,
  body        = $4,
  updated_at  = NOW()
WHERE id = $1 AND org_id = $2;

-- name: UpdatePPAVVSCC :exec
UPDATE po_avvs SET
  scc_module     = $3,
  scc_annex_i    = $4,
  scc_annex_ii   = $5,
  scc_annex_iii  = $6,
  updated_at     = NOW()
WHERE id = $1 AND org_id = $2;

-- name: DeletePPAVV :exec
DELETE FROM po_avvs WHERE id = $1 AND org_id = $2;

-- name: ListExpiringPPAVVs :many
SELECT id, org_id, processor_name, service_description,
       contract_date, review_date, status, notes,
       template_id, body, scc_module, scc_annex_i, scc_annex_ii, scc_annex_iii,
       created_at, updated_at
FROM po_avvs
WHERE review_date IS NOT NULL
  AND review_date <= $1
  AND status = 'active'
ORDER BY review_date ASC;

-- name: MarkExpiredPPAVVs :execrows
UPDATE po_avvs SET
  status     = 'expired',
  updated_at = NOW()
WHERE status = 'active'
  AND review_date IS NOT NULL
  AND review_date < CURRENT_DATE;

-- ── Breaches (Datenpannen) — Art. 33/34 DSGVO ───────────────────────────────

-- name: ListPPBreaches :many
SELECT id, org_id, title, description, discovered_at,
       authority_deadline_at, authority_notified_at,
       subjects_notification_required, subjects_notified_at,
       affected_count, data_categories, status, created_at, updated_at
FROM po_breaches
WHERE org_id = $1
ORDER BY discovered_at DESC;

-- name: ListPPBreachesPaged :many
SELECT id, org_id, title, description, discovered_at,
       authority_deadline_at, authority_notified_at,
       subjects_notification_required, subjects_notified_at,
       affected_count, data_categories, status, created_at, updated_at
FROM po_breaches
WHERE org_id = $1
ORDER BY discovered_at DESC
LIMIT $2 OFFSET $3;

-- name: CountPPBreaches :one
SELECT COUNT(*) FROM po_breaches WHERE org_id = $1;

-- name: GetPPBreach :one
SELECT id, org_id, title, description, discovered_at,
       authority_deadline_at, authority_notified_at,
       subjects_notification_required, subjects_notified_at,
       affected_count, data_categories, status, created_at, updated_at
FROM po_breaches
WHERE id = $1 AND org_id = $2;

-- name: CreatePPBreach :one
INSERT INTO po_breaches
  (org_id, title, description, discovered_at, authority_deadline_at,
   subjects_notification_required, affected_count, data_categories)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, org_id, title, description, discovered_at,
          authority_deadline_at, authority_notified_at,
          subjects_notification_required, subjects_notified_at,
          affected_count, data_categories, status, created_at, updated_at;

-- name: UpdatePPBreach :one
UPDATE po_breaches SET
  title                          = $3,
  description                    = $4,
  subjects_notification_required = $5,
  affected_count                 = $6,
  data_categories                = $7,
  updated_at                     = NOW()
WHERE id = $1 AND org_id = $2
RETURNING id, org_id, title, description, discovered_at,
          authority_deadline_at, authority_notified_at,
          subjects_notification_required, subjects_notified_at,
          affected_count, data_categories, status, created_at, updated_at;

-- name: UpdatePPBreachStatus :exec
UPDATE po_breaches SET status = $3, updated_at = NOW()
WHERE id = $1 AND org_id = $2;

-- name: MarkPPBreachAuthorityNotified :exec
UPDATE po_breaches SET
  authority_notified_at = NOW(),
  updated_at            = NOW()
WHERE id = $1 AND org_id = $2;

-- name: DeletePPBreach :exec
DELETE FROM po_breaches WHERE id = $1 AND org_id = $2;

-- ── DSR (Data Subject Requests) — Art. 15-21 DSGVO ──────────────────────────

-- name: ListPPDSRs :many
SELECT id, org_id, requester_name, requester_email, type, description,
       status, due_date, received_at, completed_at, notes,
       token_hash, source, portal_locale, submitted_ip, verify_token_hash,
       created_at, updated_at
FROM po_dsr
WHERE org_id = $1
ORDER BY received_at DESC
LIMIT 500;

-- name: GetPPDSR :one
SELECT id, org_id, requester_name, requester_email, type, description,
       status, due_date, received_at, completed_at, notes,
       token_hash, source, portal_locale, submitted_ip, verify_token_hash,
       created_at, updated_at
FROM po_dsr
WHERE id = $1 AND org_id = $2;

-- name: CreatePPDSR :one
INSERT INTO po_dsr
  (org_id, requester_name, requester_email, type, description, due_date)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, org_id, requester_name, requester_email, type, description,
          status, due_date, received_at, completed_at, notes,
          token_hash, source, portal_locale, submitted_ip, verify_token_hash,
          created_at, updated_at;

-- name: UpdatePPDSR :one
UPDATE po_dsr SET
  status       = $3,
  notes        = $4,
  completed_at = $5,
  updated_at   = NOW()
WHERE id = $1 AND org_id = $2
RETURNING id, org_id, requester_name, requester_email, type, description,
          status, due_date, received_at, completed_at, notes,
          token_hash, source, portal_locale, submitted_ip, verify_token_hash,
          created_at, updated_at;

-- name: DeletePPDSR :exec
DELETE FROM po_dsr WHERE id = $1 AND org_id = $2;

-- name: CreatePortalPPDSR :one
INSERT INTO po_dsr
  (org_id, requester_name, requester_email, type, description, due_date,
   source, portal_locale, token_hash, verify_token_hash, submitted_ip)
VALUES
  ($1, $2, $3, $4, $5, $6, 'portal', $7, $8, $9, $10)
RETURNING id;

-- name: GetPPDSRByTokenHash :one
SELECT id, org_id, requester_name, requester_email, type, description,
       status, due_date, received_at, completed_at, notes,
       token_hash, source, portal_locale, submitted_ip, verify_token_hash,
       created_at, updated_at
FROM po_dsr
WHERE token_hash = $1;

-- name: ExecutePPDSRErasure :one
UPDATE po_dsr SET
  status       = 'completed',
  completed_at = NOW(),
  notes        = COALESCE(notes, '') || E'\n\n--- Erasure executed ---\n' || $3,
  updated_at   = NOW()
WHERE id = $1 AND org_id = $2
  AND type = 'erasure'
RETURNING id, org_id, requester_name, requester_email, type, description,
          status, due_date, received_at, completed_at, notes,
          token_hash, source, portal_locale, submitted_ip, verify_token_hash,
          created_at, updated_at;

-- ── DSR Portal Settings (on organizations table) ────────────────────────────

-- name: GetOrgByDSRSlug :one
SELECT id, name, dsr_dpo_email, dsr_portal_intro, dsr_portal_enabled
FROM organizations
WHERE dsr_portal_slug = $1;

-- name: GetDSRPortalSettings :one
SELECT dsr_portal_enabled, dsr_portal_slug, dsr_dpo_email, dsr_portal_intro
FROM organizations
WHERE id = $1;

-- name: UpdateDSRPortalSettings :exec
UPDATE organizations SET
  dsr_portal_enabled = $2,
  dsr_portal_slug    = $3,
  dsr_dpo_email      = $4,
  dsr_portal_intro   = $5
WHERE id = $1;

-- ── Paged list helpers ──────────────────────────────────────────────────────

-- name: ListPPVVTPaged :many
SELECT id, org_id, name, purpose, legal_basis,
       data_categories, data_subjects, recipients,
       retention_period, third_country_transfer,
       safeguards, responsible_person,
       status, created_at, updated_at
FROM po_processing_activities
WHERE org_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountPPVVT :one
SELECT COUNT(*) FROM po_processing_activities WHERE org_id = $1;
