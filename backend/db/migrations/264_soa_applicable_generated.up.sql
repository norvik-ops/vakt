-- R1-20-02: Drei "Statement of Applicability"-Exporte, zwei Wahrheiten.
--
-- ck_controls trug die Anwendbarkeit einer Kontrolle in ZWEI Spalten:
--   not_applicable  (Migration 016, DEFAULT false)
--   soa_applicable  (Migration 113, DEFAULT true)
-- Beide bedeuten dasselbe, invers zueinander — und sie wurden von
-- verschiedenen Codepfaden geschrieben:
--   PATCH /vaktcomply/controls/:id       -> UpdateCKControl            -> not_applicable
--   Freigabe-Workflow                    -> ApplyCKApprovedControlStatus -> not_applicable
--   PATCH /vaktcomply/soa/:control_id    -> UpdateSoAApplicability     -> soa_applicable
--
-- Zwei Leser exportierten sie sogar unter demselben Alias `applicable`:
--   ListCKControlsForSoA  -> (NOT c.not_applicable) AS applicable  -> /frameworks/:id/soa.pdf
--   ListCKSoAEntries      -> c.soa_applicable       AS applicable  -> /vaktcomply/soa.csv
--
-- Live nachgestellt: PATCH /vaktcomply/soa/<id> {"applicable": false} antwortet
-- 204 und setzt nur soa_applicable. Danach steht in derselben Zeile
-- soa_applicable = false UND not_applicable = false. Die CSV sagt "Nein", die
-- PDF sagt im selben Moment "Ja". Fuer ISO 27001 Klausel 6.1.3 ist die
-- Anwendbarkeitserklaerung DER Nachweis; zwei Dokumente aus einem System, die
-- sich ueber dieselbe Kontrolle widersprechen, sind fuer einen Auditor
-- schlimmer als ein fehlendes Dokument.
--
-- Warum nicht "beide Schreibpfade ziehen einander nach": genau daraus ist der
-- Defekt entstanden, und der naechste Schreibpfad vergisst es wieder. Nach
-- ADR-0082 wird abgeleiteter Zustand, den mehr als ein Leser braucht, in der
-- Datenbank abgeleitet.
--
-- Warum not_applicable die Quelle wird und soa_applicable die Ableitung
-- (gemessener Nenner, nicht Geschmack):
--   not_applicable  2 produktive Schreiber, 13 direkte Lesestellen
--   soa_applicable  1 produktiver Schreiber, 1 Lesestelle
-- Entscheidend ist aber nicht die Mehrheit, sondern das Schema selbst:
-- Migration 254 hat ck_controls.status als GENERATED ALWAYS AS (...
-- not_applicable ...) angelegt. PostgreSQL verbietet, dass eine generierte
-- Spalte eine andere generierte Spalte liest ("cannot use generated column
-- ... in column generation expression", an PostgreSQL 16.14 nachgemessen).
-- Waere not_applicable die abgeleitete Spalte, muesste status gleich mit
-- umgebaut werden. Die Richtung ist damit vorgegeben, nicht gewaehlt.
--
-- Ab hier kann soa_applicable strukturell nicht mehr widersprechen: ein
-- Schreibversuch scheitert hart und sofort.

-- Schritt 1: bestehende Widersprueche aufloesen, BEVOR die Spalte abgeleitet wird.
--
-- Welche Seite gewinnt: die AUSSCHLIESSUNG gewinnt.
--
-- Begruendung: beide Spalten haben einen Vorgabewert, der "anwendbar" bedeutet
-- (not_applicable = false, soa_applicable = true). In einer widerspruechlichen
-- Zeile traegt deshalb genau eine Spalte einen aktiv gesetzten Wert — den
-- Ausschluss — und die andere nur ihren unberuehrten Vorgabewert. Ein
-- Ausschluss ist die dokumentierte Entscheidung, an der eine Begruendung
-- haengt (not_applicable_reason bzw. soa_justification_no); "anwendbar" ist
-- der Normalzustand, den niemand belegen muss.
--
-- Die Gegenrichtung waere die schlechtere Fehlerart: sie zoege eine bewusst
-- ausgeschlossene Kontrolle still wieder in den Geltungsbereich, und die SoA
-- behauptete Anwendbarkeit ohne die Begruendung, die ein Auditor erwartet.
-- Der Fehlerfall dieser Regel ist dagegen sichtbar: eine Kontrolle, die jemand
-- ueber die SoA-Seite wieder aufgenommen hatte, steht erneut als
-- ausgeschlossen da — samt alter Begruendung — und faellt beim naechsten
-- Durchsehen auf. Ohne Zeitstempel je Spalte ist nicht entscheidbar, welche
-- der beiden Eingaben die spaetere war; deshalb faellt die Wahl auf die
-- Richtung, deren Fehler auffaellt statt zu verschwinden.
UPDATE ck_controls
SET not_applicable = true
WHERE soa_applicable = false
  AND not_applicable = false;

-- Schritt 2: soa_applicable wird zur Ableitung von not_applicable.
ALTER TABLE ck_controls DROP COLUMN IF EXISTS soa_applicable;

ALTER TABLE ck_controls
  ADD COLUMN soa_applicable BOOLEAN NOT NULL
  GENERATED ALWAYS AS (NOT not_applicable) STORED;
