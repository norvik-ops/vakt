-- ESK-12: ck_bcp_plans.rto_hours / rpo_hours / schutzbedarfsklasse duerfen "nicht
-- festgelegt" ausdruecken koennen.
--
-- WARUM: Migration 216 hat die drei Spalten NOT NULL DEFAULT 72/24/2 angelegt.
-- Einen Schreibpfad gab es bis zu diesem Commit NICHT (das einzige INSERT nannte
-- 6 Spalten, das einzige UPDATE 5, die Input-Structs fuehrten die Felder nicht).
-- Seit PR #75 die Spalten in die SELECTs aufgenommen hat, liefert die API fuer
-- JEDEN Plan jeder Organisation 72/24/2 — Werte, die wie kuratierte
-- BSI-200-4-Angaben aussehen und von echten nicht zu unterscheiden sind.
-- RTO/RPO ist Audit-Evidenz; eine Konstante, die als planbezogene Angabe
-- auftritt, ist eine falsche Aussage in einem Dokument, das ein Pruefer liest.
--
-- NULL ist hier die ehrliche Antwort: "fuer diesen Plan noch nicht festgelegt"
-- ist in einem Compliance-Werkzeug ein echter, anzeigenswerter Zustand — und der
-- einzige, der nicht behauptet, jemand haette 72 Stunden entschieden.
ALTER TABLE ck_bcp_plans
    ALTER COLUMN rto_hours           DROP DEFAULT,
    ALTER COLUMN rto_hours           DROP NOT NULL,
    ALTER COLUMN rpo_hours           DROP DEFAULT,
    ALTER COLUMN rpo_hours           DROP NOT NULL,
    ALTER COLUMN schutzbedarfsklasse DROP DEFAULT,
    ALTER COLUMN schutzbedarfsklasse DROP NOT NULL;

-- Der Bestand wird auf NULL zurueckgesetzt, weil JEDER heute gespeicherte Wert
-- dieser drei Spalten beweisbar ein Migrations-Default ist und keine Aussage:
-- vor diesem Commit konnte kein Aufrufer sie setzen (kein Feld im Input-Struct,
-- keine Spalte im INSERT/UPDATE, kein Raw-SQL daneben — gegengegrept in ESK-12).
-- Stehen zu lassen hiesse, die erfundenen Werte fuer Bestandsplaene dauerhaft
-- festzuschreiben; genau die Aussage, die dieser Fix beseitigt. Zurueckgenommen
-- wird das von der down-Migration (sie fuellt NULL wieder mit 72/24/2).
UPDATE ck_bcp_plans SET rto_hours = NULL, rpo_hours = NULL, schutzbedarfsklasse = NULL;

-- Der CHECK schutzbedarfsklasse IN (1,2,3) aus Migration 216 bleibt unveraendert
-- bestehen; NULL passiert einen CHECK (unknown ist nicht false).

-- last_tested_at wird dagegen NICHT geleert, sondern aus den vorhandenen
-- ck_bcp_tests-Eintraegen NACHGETRAGEN. Der Unterschied zu den drei Spalten
-- oben ist der Beleg: 72/24/2 hat niemand behauptet, wohl aber wurden diese
-- Tests tatsaechlich als Datensatz angelegt. Ein Plan mit protokolliertem Test,
-- der "nie getestet" meldet, waere die Luege in die andere Richtung. Plaene ohne
-- Testeintrag bleiben NULL (MAX ueber die leere Menge ist NULL).
UPDATE ck_bcp_plans p
SET last_tested_at = (
        SELECT MAX(t.test_date) FROM ck_bcp_tests t
        WHERE t.plan_id = p.id AND t.org_id = p.org_id
    )
WHERE p.last_tested_at IS NULL;
