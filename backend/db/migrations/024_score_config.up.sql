CREATE TABLE IF NOT EXISTS score_config (
    org_id           UUID PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    base_score       INT NOT NULL DEFAULT 70,
    crit_penalty     INT NOT NULL DEFAULT 5,
    crit_penalty_cap INT NOT NULL DEFAULT 30,
    high_penalty     INT NOT NULL DEFAULT 2,
    high_penalty_cap INT NOT NULL DEFAULT 10,
    breach_penalty   INT NOT NULL DEFAULT 20,
    breach_penalty_cap INT NOT NULL DEFAULT 20,
    fw_bonus         INT NOT NULL DEFAULT 10,
    fw_bonus_cap     INT NOT NULL DEFAULT 30,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
