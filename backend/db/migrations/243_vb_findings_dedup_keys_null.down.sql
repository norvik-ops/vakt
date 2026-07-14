-- Zurueck auf Leerstrings.
--
-- Das stellt den Zustand wieder her, in dem die partiellen Dedup-Indexe fuer JEDEN
-- Fund griffen — also den Zustand, in dem ein Scan mit zwei Funden nichts speichert.
-- Es ist bewusst trotzdem hier: Eine Down-Migration, die nicht laeuft, ist schlimmer
-- als eine, die den alten (kaputten) Zustand ehrlich wiederherstellt.
--
-- ACHTUNG: Das kann selbst an den Unique-Indexen scheitern, wenn inzwischen mehrere
-- Funde ohne Template/RawID existieren — genau die, die es vorher nicht geben
-- konnte. Wer hierher zurueck muss, muss die ueberzaehligen Zeilen vorher loeschen.
UPDATE vb_findings SET template_id = '' WHERE template_id IS NULL;
UPDATE vb_findings SET raw_id      = '' WHERE raw_id      IS NULL;
