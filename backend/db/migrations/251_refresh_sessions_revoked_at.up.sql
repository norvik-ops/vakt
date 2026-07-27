-- DSGVO-Art.-20-Export von sessions.json war für JEDEN Nutzer leer (SA23-D2).
--
-- internal/shared/account/account.go::ExportUserData liest:
--
--     SELECT id::text, device_hint, created_at, last_used, expires_at, revoked_at
--     FROM refresh_sessions WHERE user_id = $1::uuid ORDER BY created_at
--
-- refresh_sessions (Migration 117) hat aber nie eine revoked_at-Spalte. Diese
-- Query scheiterte deshalb bei jedem Export an SQLSTATE 42703 — der
-- best-effort-Wrapper in ExportUserData fängt den Fehler ab und schreibt
-- sessions.json als leeres Array, ohne den Export insgesamt fehlschlagen zu
-- lassen. Ein Nutzer, der seine Daten nach Art. 20 DSGVO exportiert, bekam
-- also nie seine aktiven Sitzungen zu sehen — unabhängig davon, wie viele
-- es tatsächlich gab.
--
-- Die Spalte wird ergänzt statt der Query zu vereinfachen, weil sessions.json
-- vollständig sein muss. Der Revoke-Weg bleibt unverändert ein DELETE (siehe
-- den Kommentar in account.go bei AnonymizeAndDeactivate: die kanonische
-- Session-Revocation überall sonst — Logout, Passwort-Reset, service.go — ist
-- ein DELETE, kein UPDATE ... SET revoked_at). Eine Zeile, die noch in
-- refresh_sessions existiert, ist per Definition nicht widerrufen; die neue
-- Spalte bleibt für bestehenden Code deshalb dauerhaft NULL und dient
-- ausschließlich dazu, dass der Export dieselbe Spaltenliste wie api_keys
-- (das bereits ein echtes revoked_at führt) lesen kann, ohne 42703.
ALTER TABLE refresh_sessions
    ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ;

COMMENT ON COLUMN refresh_sessions.revoked_at IS
    'Für den DSGVO-Art.-20-Export (sessions.json). Bleibt NULL: eine Zeile, '
    'die noch existiert, ist per Definition aktiv — Revocation ist überall '
    'ein DELETE, kein Soft-Revoke dieser Spalte.';
