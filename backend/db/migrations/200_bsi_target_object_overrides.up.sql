ALTER TABLE ck_bsi_target_objects
    ADD COLUMN override_c      TEXT CHECK (override_c IN ('normal', 'hoch', 'sehr_hoch')),
    ADD COLUMN override_i      TEXT CHECK (override_i IN ('normal', 'hoch', 'sehr_hoch')),
    ADD COLUMN override_a      TEXT CHECK (override_a IN ('normal', 'hoch', 'sehr_hoch')),
    ADD COLUMN override_reason TEXT,
    ADD COLUMN override_effect TEXT CHECK (override_effect IN ('kumulation', 'verteilung'));
