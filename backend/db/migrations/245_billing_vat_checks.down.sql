-- ACHTUNG: Das verwirft Steuer-NACHWEISE, keine Betriebsdaten.
--
-- Die Zeilen belegen, dass und wann die USt-IdNr. eines Kunden geprueft wurde — genau
-- das, was bei einer Betriebspruefung zu Reverse-Charge-Umsaetzen verlangt wird. Sie
-- sind aus keiner anderen Quelle rekonstruierbar: Lexware fuehrt keine Historie, und
-- VIES beantwortet nur die Gegenwart, nie "war diese Nummer im Maerz gueltig?".
--
-- Vor einem Down auf einer Instanz mit echten Verkaeufen sichern:
--   COPY (SELECT * FROM billing_vat_checks) TO '/tmp/vat_checks_backup.csv' CSV HEADER;

DROP TABLE IF EXISTS billing_vat_checks;
