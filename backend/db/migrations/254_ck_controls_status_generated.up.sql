-- R1-14b-A1 / R1-20-A12: ck_controls.status hatte keinen einzigen Schreiber.
--
-- Migration 153 hat die Spalte nur deshalb angelegt, weil Export-Queries
-- bereits c.status lasen und mit SQLSTATE 42703 scheiterten. Sie kam mit
-- DEFAULT 'missing' und blieb danach fuer immer auf diesem Wert: alle
-- Schreibpfade (UpdateCKControl, BulkUpdateCKControlStatus,
-- ApplyCKApprovedControlStatus, nis2wizard.AutoMapToControls, demoseed)
-- schreiben ck_controls.manual_status bzw. ck_controls.not_applicable.
--
-- Folge: jeder Leser von c.status meldete 'missing' fuer die gesamte
-- Organisation — controls.csv und gap_analysis.html im Audit-Paket, die
-- KPI-Kennzahl kpi_compliance_score, der KI-Kontext und der
-- Kontrolltest-Erinnerungsjob.
--
-- Statt einen weiteren Schreiber nachzuruesten (der bei jedem neuen
-- Schreibpfad wieder vergessen werden kann) wird status zur berechneten
-- Spalte. Damit kann sie strukturell nicht mehr von manual_status
-- abweichen, und alle Leser sind ohne Codeaenderung korrekt — auch die,
-- die spaeter dazukommen.
--
-- Abbildung auf das Vokabular, das die Leser erwarten
-- ('implemented' | 'in_progress' | 'missing' | 'not_applicable'):
--   not_applicable = true              -> not_applicable
--   manual_status  = 'implemented'     -> implemented   (UI, Freigabe-Workflow, NIS2-Wizard)
--   manual_status  = 'in_progress'     -> in_progress   (UI, Freigabe-Workflow)
--   manual_status  = 'partial'         -> in_progress   (NIS2-Wizard-Dialekt)
--   manual_status  = 'partially_implemented' -> in_progress (Dashboard-Dialekt)
--   sonst (NULL, '', 'not_implemented', 'missing') -> missing
--
-- Nicht abgebildet ist der Nachweis-Stand: ein Control mit Evidenz, aber
-- ohne gepflegten Umsetzungsstand bleibt 'missing'. Diese Achse liegt in
-- ck_controls.evidence_status und im Readiness-Report; eine berechnete
-- Spalte darf keine andere Tabelle lesen.
--
-- Kein Schreibpfad nennt status in einem INSERT oder UPDATE (geprueft ueber
-- db/queries/*.sql und alle Roh-SQL-Stellen), die Spalte kann deshalb ohne
-- Anpassung der Schreiber berechnet werden.

ALTER TABLE ck_controls DROP COLUMN IF EXISTS status;

ALTER TABLE ck_controls
  ADD COLUMN status TEXT NOT NULL
  GENERATED ALWAYS AS (
    CASE
      WHEN not_applicable THEN 'not_applicable'
      WHEN manual_status = 'implemented' THEN 'implemented'
      WHEN manual_status IN ('in_progress', 'partial', 'partially_implemented') THEN 'in_progress'
      ELSE 'missing'
    END
  ) STORED;

-- Migration 111 hat einen Index auf (framework_id, manual_status); die
-- Leser filtern durchgaengig ueber status. Der Index wird nachgezogen.
CREATE INDEX IF NOT EXISTS idx_ck_controls_org_status
  ON ck_controls (org_id, status);
