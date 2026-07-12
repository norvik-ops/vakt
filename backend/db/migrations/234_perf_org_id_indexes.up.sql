-- PERF-01 (Sprint 125 follow-up): index every org_id column that lacked one.
-- In a multi-tenant app every module query filters by org_id; without an index
-- each becomes a sequential scan that scales with the whole table, not the org.
-- Plain (org_id) btree only — composite (org_id, filter) indexes are deliberately
-- NOT added here without verified query patterns (unused indexes cost writes).
-- No CONCURRENTLY (useless inside golang-migrate's transaction, CLAUDE.md); tables
-- are small on current instances so the brief build lock is acceptable.

CREATE INDEX IF NOT EXISTS idx_auditor_sessions_org_id ON auditor_sessions (org_id);
CREATE INDEX IF NOT EXISTS idx_backup_log_org_id ON backup_log (org_id);
CREATE INDEX IF NOT EXISTS idx_ck_ai_classifications_org_id ON ck_ai_classifications (org_id);
CREATE INDEX IF NOT EXISTS idx_ck_ai_documentation_org_id ON ck_ai_documentation (org_id);
CREATE INDEX IF NOT EXISTS idx_ck_audit_program_findings_org_id ON ck_audit_program_findings (org_id);
CREATE INDEX IF NOT EXISTS idx_ck_bcp_tests_org_id ON ck_bcp_tests (org_id);
CREATE INDEX IF NOT EXISTS idx_ck_control_changelog_org_id ON ck_control_changelog (org_id);
CREATE INDEX IF NOT EXISTS idx_ck_control_exceptions_org_id ON ck_control_exceptions (org_id);
CREATE INDEX IF NOT EXISTS idx_ck_control_reviews_org_id ON ck_control_reviews (org_id);
CREATE INDEX IF NOT EXISTS idx_ck_control_tasks_org_id ON ck_control_tasks (org_id);
CREATE INDEX IF NOT EXISTS idx_ck_evidence_history_org_id ON ck_evidence_history (org_id);
CREATE INDEX IF NOT EXISTS idx_ck_incident_reports_org_id ON ck_incident_reports (org_id);
CREATE INDEX IF NOT EXISTS idx_ck_nis2_reports_org_id ON ck_nis2_reports (org_id);
CREATE INDEX IF NOT EXISTS idx_ck_policy_acceptance_campaigns_org_id ON ck_policy_acceptance_campaigns (org_id);
CREATE INDEX IF NOT EXISTS idx_ck_policy_acceptance_requests_org_id ON ck_policy_acceptance_requests (org_id);
CREATE INDEX IF NOT EXISTS idx_ck_policy_versions_org_id ON ck_policy_versions (org_id);
CREATE INDEX IF NOT EXISTS idx_ck_reviews_org_id ON ck_reviews (org_id);
CREATE INDEX IF NOT EXISTS idx_hr_access_concept_versions_org_id ON hr_access_concept_versions (org_id);
CREATE INDEX IF NOT EXISTS idx_hr_access_roles_org_id ON hr_access_roles (org_id);
CREATE INDEX IF NOT EXISTS idx_hr_checklist_runs_org_id ON hr_checklist_runs (org_id);
CREATE INDEX IF NOT EXISTS idx_hr_checklists_org_id ON hr_checklists (org_id);
CREATE INDEX IF NOT EXISTS idx_hr_mover_templates_org_id ON hr_mover_templates (org_id);
CREATE INDEX IF NOT EXISTS idx_login_history_org_id ON login_history (org_id);
CREATE INDEX IF NOT EXISTS idx_refresh_sessions_org_id ON refresh_sessions (org_id);
CREATE INDEX IF NOT EXISTS idx_sessions_org_id ON sessions (org_id);
CREATE INDEX IF NOT EXISTS idx_so_access_log_org_id ON so_access_log (org_id);
CREATE INDEX IF NOT EXISTS idx_so_environments_org_id ON so_environments (org_id);
CREATE INDEX IF NOT EXISTS idx_so_rotation_policies_org_id ON so_rotation_policies (org_id);
CREATE INDEX IF NOT EXISTS idx_so_scan_results_org_id ON so_scan_results (org_id);
CREATE INDEX IF NOT EXISTS idx_so_share_links_org_id ON so_share_links (org_id);
CREATE INDEX IF NOT EXISTS idx_sr_assignments_org_id ON sr_assignments (org_id);
CREATE INDEX IF NOT EXISTS idx_sr_campaign_enrollments_org_id ON sr_campaign_enrollments (org_id);
CREATE INDEX IF NOT EXISTS idx_sr_completions_org_id ON sr_completions (org_id);
CREATE INDEX IF NOT EXISTS idx_sr_events_org_id ON sr_events (org_id);
CREATE INDEX IF NOT EXISTS idx_sr_landing_pages_org_id ON sr_landing_pages (org_id);
CREATE INDEX IF NOT EXISTS idx_sr_target_groups_org_id ON sr_target_groups (org_id);
CREATE INDEX IF NOT EXISTS idx_sr_targets_org_id ON sr_targets (org_id);
CREATE INDEX IF NOT EXISTS idx_sr_templates_org_id ON sr_templates (org_id);
CREATE INDEX IF NOT EXISTS idx_sr_training_modules_org_id ON sr_training_modules (org_id);
CREATE INDEX IF NOT EXISTS idx_vb_finding_suppressions_org_id ON vb_finding_suppressions (org_id);
CREATE INDEX IF NOT EXISTS idx_vb_sboms_org_id ON vb_sboms (org_id);
CREATE INDEX IF NOT EXISTS idx_vb_scan_schedules_org_id ON vb_scan_schedules (org_id);
