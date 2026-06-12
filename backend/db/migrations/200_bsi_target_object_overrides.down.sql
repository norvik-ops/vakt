ALTER TABLE ck_bsi_target_objects
    DROP COLUMN IF EXISTS override_c,
    DROP COLUMN IF EXISTS override_i,
    DROP COLUMN IF EXISTS override_a,
    DROP COLUMN IF EXISTS override_reason,
    DROP COLUMN IF EXISTS override_effect;
