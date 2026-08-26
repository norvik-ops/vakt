-- Zurueck auf den Zustand von Migration 113: soa_applicable als gewoehnliche,
-- frei beschreibbare Spalte mit Vorgabewert true.
--
-- DATENVERLUST in dieser Richtung, und zwar genau einer:
-- Die up-Migration hat widerspruechliche Zeilen zusammengefuehrt (Ausschluss
-- gewinnt). Welche Zeilen das waren, ist danach nicht mehr rekonstruierbar —
-- die Spalte trug den Widerspruch, und der ist aufgeloest. Ein down stellt
-- deshalb einen widerspruchsfreien Zustand her, nicht den urspruenglichen:
-- Zeilen, die vor der up-Migration soa_applicable = false bei
-- not_applicable = false trugen, kommen als "ausgeschlossen auf beiden
-- Spalten" zurueck, nicht als Widerspruch.
--
-- Die Werte selbst gehen nicht verloren: soa_applicable ist aus
-- not_applicable vollstaendig ableitbar und wird unten genau so vorbelegt.

ALTER TABLE ck_controls DROP COLUMN IF EXISTS soa_applicable;

ALTER TABLE ck_controls
  ADD COLUMN soa_applicable BOOLEAN NOT NULL DEFAULT true;

UPDATE ck_controls SET soa_applicable = NOT not_applicable;
