-- 150_disable_rls_theater.up.sql
--
-- Migration 012 enabled Row Level Security on eight module tables with
-- policies referencing the session variable `app.current_org_id`. The
-- application never set that variable — and because no FORCE was applied,
-- the App-User (as table owner) bypassed RLS entirely anyway. The setup
-- *looked* like defense-in-depth but provided none.
--
-- Three options were considered (see ADR-0042):
--   A) wire `SET LOCAL app.current_org_id` into every pgxpool acquisition
--   B) remove the RLS scaffolding and own the truth (App-layer enforcement)
--   C) keep selective tables under FORCE RLS
--
-- We chose B. The app already enforces org-isolation on every read/write
-- path; the RLS layer added zero protection while suggesting otherwise to
-- auditors and pen-testers. Honest is better than aspirational.
--
-- Should we later want true defense-in-depth, a fresh migration can re-
-- introduce RLS with FORCE and a verified connection-init that sets the
-- session var.  At that point we'll also revisit the 22+ other org-keyed
-- tables that Migration 012 left unprotected (audit_log, ck_risks, hr_*,
-- po_*, sr_* …).

-- Migration 122 renamed every pg_* table to sr_* (Secreflex prefix); the
-- RLS state followed the rename. We target the post-122 table names here.
-- IF EXISTS keeps the migration idempotent for any partial DB.
DROP POLICY IF EXISTS vb_assets_org      ON vb_assets;
DROP POLICY IF EXISTS vb_findings_org    ON vb_findings;
DROP POLICY IF EXISTS ck_frameworks_org  ON ck_frameworks;
DROP POLICY IF EXISTS ck_controls_org    ON ck_controls;
DROP POLICY IF EXISTS ck_evidence_org    ON ck_evidence;
DROP POLICY IF EXISTS so_projects_org    ON so_projects;
DROP POLICY IF EXISTS so_secrets_org     ON so_secrets;
DROP POLICY IF EXISTS pg_campaigns_org   ON sr_campaigns;

ALTER TABLE IF EXISTS vb_assets     DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS vb_findings   DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS ck_frameworks DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS ck_controls   DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS ck_evidence   DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS so_projects   DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS so_secrets    DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS sr_campaigns  DISABLE ROW LEVEL SECURITY;
