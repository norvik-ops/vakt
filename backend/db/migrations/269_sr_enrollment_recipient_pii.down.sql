-- Rücknahme von ADR-0088: die beiden additiven PII-Spalten wieder entfernen.
ALTER TABLE sr_campaign_enrollments
    DROP COLUMN full_name,
    DROP COLUMN email;
