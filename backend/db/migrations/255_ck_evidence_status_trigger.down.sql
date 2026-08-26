DROP TRIGGER IF EXISTS trg_ck_evidence_status ON ck_evidence;
DROP FUNCTION IF EXISTS ck_evidence_status_trigger();
DROP FUNCTION IF EXISTS ck_refresh_evidence_status(UUID);
