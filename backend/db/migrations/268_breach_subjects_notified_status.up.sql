-- R1-36c-04: die DSGVO-Breach-Statuskette (Art. 33/34) braucht den Zustand
-- 'subjects_notified' (Art. 34: Benachrichtigung der Betroffenen). Die urspruengliche
-- CHECK-Constraint (Migration 014) kannte nur open/authority_notified/closed, sodass
-- die Art.-34-Stufe nicht persistierbar war. Additiv: die erlaubten Werte werden nur
-- ERWEITERT, bestehende Zeilen bleiben gueltig.
ALTER TABLE po_breaches DROP CONSTRAINT IF EXISTS po_breaches_status_check;
ALTER TABLE po_breaches ADD CONSTRAINT po_breaches_status_check
  CHECK (status IN ('open','authority_notified','subjects_notified','closed'));
