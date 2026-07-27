-- Key-Rotation für cloud_integrations schlug für JEDE Zeile fehl (SA23-D3).
--
-- cmd/rotate-key/rotate.go::rotateCloudIntegrationsExec schreibt seit seiner
-- Einführung:
--
--     UPDATE cloud_integrations SET config = $1::jsonb, updated_at = now()
--     WHERE id = $2::uuid
--
-- cloud_integrations (Migration 096) hat aber nie eine updated_at-Spalte —
-- nur created_at. Jeder Rotationslauf scheiterte an SQLSTATE 42703 (column
-- "updated_at" does not exist), sobald er auch nur eine AWS- oder
-- Azure-Integration vorfand: kein Provider-Secret in cloud_integrations.config
-- konnte je unter einem neuen Master-Key rotiert werden.
--
-- Die Spalte wird ergänzt statt der toten Referenz entfernt zu werden, weil
-- Key-Rotation vollständig sein muss (CLAUDE.md-Vorgabe): ein Secret, das die
-- Rotation überspringt, bleibt unter dem alten Key verschlüsselt — genau der
-- Zustand, den ein Rotationslauf beheben soll.
ALTER TABLE cloud_integrations
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

COMMENT ON COLUMN cloud_integrations.updated_at IS
    'Zuletzt geändert (Config-Update oder Key-Rotation). Bestehende Zeilen '
    'starten bei NOW() der Migration, nicht bei ihrem tatsächlichen created_at.';
