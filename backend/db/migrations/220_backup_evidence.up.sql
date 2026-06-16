-- S88-2: Backup-/Restore-Nachweis-Workflow (ISO 27001:2022 A.8.13, BSI DER.4)
-- Documentation-first: this is a NACHWEIS registry, not a backup operator.

CREATE TABLE ck_backup_jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL,
    name            TEXT NOT NULL,
    source          TEXT NOT NULL DEFAULT '',
    destination     TEXT NOT NULL DEFAULT '',
    frequency       TEXT NOT NULL DEFAULT 'daily'
                        CHECK (frequency IN ('hourly','daily','weekly','monthly')),
    encrypted       BOOLEAN NOT NULL DEFAULT TRUE,
    last_success_at TIMESTAMPTZ,
    last_status     TEXT NOT NULL DEFAULT 'unknown'
                        CHECK (last_status IN ('unknown','success','failed')),
    -- Restore-test must be no older than this many days to count as fresh.
    restore_max_age_days INT NOT NULL DEFAULT 365 CHECK (restore_max_age_days > 0),
    notes           TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_ck_backup_jobs_org_id ON ck_backup_jobs (org_id);

CREATE TABLE ck_backup_restore_tests (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id         UUID NOT NULL,
    job_id         UUID NOT NULL REFERENCES ck_backup_jobs(id) ON DELETE CASCADE,
    tested_at      DATE NOT NULL,
    result         TEXT NOT NULL DEFAULT 'success'
                       CHECK (result IN ('success','partial','failed')),
    rto_target_hours INT NOT NULL DEFAULT 0,
    rto_actual_hours INT NOT NULL DEFAULT 0,
    tester         TEXT NOT NULL DEFAULT '',
    notes          TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_ck_backup_restore_tests_org_id ON ck_backup_restore_tests (org_id);
CREATE INDEX idx_ck_backup_restore_tests_job_id ON ck_backup_restore_tests (job_id);
