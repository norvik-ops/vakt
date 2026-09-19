-- Rueckbau nur sicher, solange keine Zeile 'subjects_notified' traegt; ein Rollback
-- mit solchen Zeilen wuerde an der engeren CHECK scheitern (bewusst: Datenverlust
-- vermeiden statt still umbiegen).
ALTER TABLE po_breaches DROP CONSTRAINT IF EXISTS po_breaches_status_check;
ALTER TABLE po_breaches ADD CONSTRAINT po_breaches_status_check
  CHECK (status IN ('open','authority_notified','closed'));
