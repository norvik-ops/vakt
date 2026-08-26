-- R1-14b-A2: die Evidenz-Versionierung war beworben, aber nie gebaut.
--
-- Vorhanden waren Tabelle (Migration 109), Spalte ck_evidence.version, Query
-- ListCKEvidenceHistory und Route GET /evidence/:id/history — nur kein
-- Schreiber. ck_evidence_history blieb im gesamten Betrieb leer, version stand
-- dauerhaft auf 1, der Endpunkt lieferte immer []. Das Produktversprechen in
-- CLAUDE.md lautet woertlich "stores versioned evidence".
--
-- Die Nachweise entstehen und aendern sich an vielen Stellen: manueller
-- Upload, rund 30 Collector-Aufrufstellen, CI-Webhook, Demo-Seed,
-- Freigabe-Pfad (ReviewCKEvidence), Zuordnung verwaister Auto-Evidenz
-- (evidence_auto). Die Versionierung sitzt deshalb als Trigger auf
-- ck_evidence und gilt fuer jeden Produzenten, auch fuer kuenftige.
--
-- Wer hat geaendert (changed_by):
-- Die Anwendung reicht keine Nutzer-Identitaet in die Datenbank durch, ein
-- Trigger kann sie also nicht frei erfragen. Zwei Faelle traegt die Zeile
-- selbst, und das sind die beiden, auf die es bei einem Audit ankommt:
--   · Anlegen  -> uploaded_by (wer hat den Nachweis eingebracht)
--   · Freigabe -> reviewed_by (wer hat ihn freigegeben)
-- Alle uebrigen Aenderungen schreiben changed_by = NULL. Die Spalte ist
-- nullable; eine Fassung ohne Urheber ist ehrlicher als gar keine Fassung.
--
-- Abgrenzung: rein verwaltende Schreibvorgaenge erzeugen keine Fassung.
-- MarkCKEvidenceExpiryNotified setzt nur expiry_notified_at und updated_at;
-- wuerde das eine Fassung erzeugen, fluteten die Ablauf-Cronlaeufe die
-- Historie und zaehlten version taeglich hoch, ohne dass sich am Nachweis
-- etwas geaendert haette.

CREATE OR REPLACE FUNCTION ck_evidence_content_changed(o ck_evidence, n ck_evidence)
RETURNS boolean
LANGUAGE sql
IMMUTABLE
AS $$
    SELECT o.title            IS DISTINCT FROM n.title
        OR o.description      IS DISTINCT FROM n.description
        OR o.status           IS DISTINCT FROM n.status
        OR o.file_path        IS DISTINCT FROM n.file_path
        OR o.file_size        IS DISTINCT FROM n.file_size
        OR o.control_id       IS DISTINCT FROM n.control_id
        OR o.source           IS DISTINCT FROM n.source
        OR o.collector_data   IS DISTINCT FROM n.collector_data
        OR o.expires_at       IS DISTINCT FROM n.expires_at;
$$;

-- Ursprungsfassung beim Anlegen.
CREATE OR REPLACE FUNCTION ck_evidence_history_insert()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO ck_evidence_history
        (evidence_id, org_id, changed_by, title, description, status, file_url, change_note)
    VALUES
        (NEW.id, NEW.org_id, NEW.uploaded_by, NEW.title, NEW.description,
         NEW.status, NEW.file_path, 'created');
    RETURN NULL;
END;
$$;

-- Neue Fassung bei jeder inhaltlichen Aenderung; version zaehlt mit.
CREATE OR REPLACE FUNCTION ck_evidence_history_update()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    actor UUID;
    note  TEXT;
BEGIN
    IF NOT ck_evidence_content_changed(OLD, NEW) THEN
        RETURN NEW;
    END IF;

    NEW.version := OLD.version + 1;

    IF OLD.status IS DISTINCT FROM NEW.status THEN
        actor := NEW.reviewed_by;
        note  := 'status: ' || COALESCE(OLD.status, '') || ' -> ' || COALESCE(NEW.status, '');
    ELSE
        actor := NULL;
        note  := 'updated';
    END IF;

    INSERT INTO ck_evidence_history
        (evidence_id, org_id, changed_by, title, description, status, file_url, change_note)
    VALUES
        (NEW.id, NEW.org_id, actor, NEW.title, NEW.description,
         NEW.status, NEW.file_path, note);

    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_ck_evidence_history_insert ON ck_evidence;
CREATE TRIGGER trg_ck_evidence_history_insert
    AFTER INSERT ON ck_evidence
    FOR EACH ROW
    EXECUTE FUNCTION ck_evidence_history_insert();

DROP TRIGGER IF EXISTS trg_ck_evidence_history_update ON ck_evidence;
CREATE TRIGGER trg_ck_evidence_history_update
    BEFORE UPDATE ON ck_evidence
    FOR EACH ROW
    EXECUTE FUNCTION ck_evidence_history_update();

-- Bestandsdaten: fuer jeden bereits vorhandenen Nachweis eine
-- Ursprungsfassung nachtragen, damit die Historie nicht bei denen leer
-- bleibt, die vor dieser Migration entstanden sind. Der aktuelle Zustand ist
-- alles, was rekonstruierbar ist — die verlorenen Zwischenstaende sind
-- verloren, das kann diese Migration nicht heilen.
INSERT INTO ck_evidence_history
    (evidence_id, org_id, changed_by, changed_at, title, description, status, file_url, change_note)
SELECT e.id, e.org_id, e.uploaded_by, e.created_at, e.title, e.description,
       e.status, e.file_path, 'created'
  FROM ck_evidence e
 WHERE NOT EXISTS (
     SELECT 1 FROM ck_evidence_history h WHERE h.evidence_id = e.id
 );
