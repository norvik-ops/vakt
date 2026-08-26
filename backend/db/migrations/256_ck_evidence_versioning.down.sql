DROP TRIGGER IF EXISTS trg_ck_evidence_history_update ON ck_evidence;
DROP TRIGGER IF EXISTS trg_ck_evidence_history_insert ON ck_evidence;
DROP FUNCTION IF EXISTS ck_evidence_history_update();
DROP FUNCTION IF EXISTS ck_evidence_history_insert();
DROP FUNCTION IF EXISTS ck_evidence_content_changed(ck_evidence, ck_evidence);
