-- Fälschungssichere Freigabe von Richtlinien.
--
-- Bisher stand das „Wer" einer Freigabe nur in last_updated_by: Freitext, den
-- der Client mitschickt. Wer und wann eine Richtlinie freigegeben hat, ließ sich
-- deshalb nicht belegen. Diese zwei Spalten schreibt ausschließlich der Server
-- (policy.Service.UpdatePolicy) beim Statuswechsel auf 'active': approved_by ist
-- die authentifizierte Nutzer-ID, approved_at der Zeitpunkt. Verlässt eine
-- Richtlinie 'active', leert der Server beide Werte wieder.
--
-- Bestehende Zeilen bleiben NULL. Für bereits aktive Richtlinien gibt es keine
-- belegbare Freigabe, also wird auch keine erfunden.
--
-- ON DELETE SET NULL: Wird ein Konto gelöscht, bleibt die Richtlinie lesbar.
-- Die Identität der Freigabe steht zusätzlich im Audit-Log (Aktion 'approve').
ALTER TABLE ck_policies
    ADD COLUMN IF NOT EXISTS approved_by UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS approved_at TIMESTAMPTZ NULL;
