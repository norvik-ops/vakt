-- Rueckbau von 252: NULL bedeutet "nicht festgelegt"; NOT NULL kann das nicht
-- ausdruecken, also muessen die nicht festgelegten Plaene wieder die
-- Migrations-Defaults aus 216 tragen. Das ist der Informationsverlust, den die
-- up-Migration behebt — beim Rueckbau ist er unvermeidlich.
UPDATE ck_bcp_plans SET rto_hours           = 72 WHERE rto_hours           IS NULL;
UPDATE ck_bcp_plans SET rpo_hours           = 24 WHERE rpo_hours           IS NULL;
UPDATE ck_bcp_plans SET schutzbedarfsklasse = 2  WHERE schutzbedarfsklasse IS NULL;

ALTER TABLE ck_bcp_plans
    ALTER COLUMN rto_hours           SET NOT NULL,
    ALTER COLUMN rto_hours           SET DEFAULT 72,
    ALTER COLUMN rpo_hours           SET NOT NULL,
    ALTER COLUMN rpo_hours           SET DEFAULT 24,
    ALTER COLUMN schutzbedarfsklasse SET NOT NULL,
    ALTER COLUMN schutzbedarfsklasse SET DEFAULT 2;
