-- Sprint 19 / S19-1 + S19-6: NIS2-Self-Assessment-Wizard.
--
-- Zwei Tabellen:
--   1. nis2_anonymous_runs — Public-Wizard ohne Auth. Run wird via Magic-
--      Token referenziert, lebt 7 Tage in der DB (Cleanup-Job via Asynq).
--   2. ck_nis2_assessments — Sign-up-Migration: wenn anonymer User einen
--      Account erstellt mit gültigem Magic-Token, wird der Run hier in die
--      Org migriert (mit historischer Trend-Möglichkeit für Pro-Tier).

CREATE TABLE IF NOT EXISTS nis2_anonymous_runs (
    token         TEXT PRIMARY KEY,                       -- Magic-Link-Token, 32 hex chars
    answers       JSONB NOT NULL DEFAULT '{}'::jsonb,     -- {question_id: {value, comment}}
    score         INTEGER,                                 -- 0..100, NULL solange unfertig
    score_by_area JSONB,                                   -- {governance: 70, risk_mgmt: 40, ...}
    completed_at  TIMESTAMPTZ,                             -- gesetzt sobald alle 30 Fragen beantwortet
    referrer      TEXT,                                    -- HTTP-Referer (Marketing-Attribution, optional)
    user_agent    TEXT,                                    -- für Embedded-Mode-Tracking, optional
    ip_hash       TEXT,                                    -- sha256(ip), kein Klartext (DSGVO)
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at    TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '7 days'
);

CREATE INDEX IF NOT EXISTS idx_nis2_anon_expires
    ON nis2_anonymous_runs (expires_at);

-- ck_nis2_assessments lebt unter dem secvitals ck_-Prefix, weil die
-- Assessment-Antworten dort logisch zur Compliance-Story der Org gehören.
-- Bei Sign-up wird ein anonymer Run in diese Tabelle migriert.
CREATE TABLE IF NOT EXISTS ck_nis2_assessments (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id       UUID REFERENCES users(id) ON DELETE SET NULL,
    answers       JSONB NOT NULL,
    score         INTEGER NOT NULL,
    score_by_area JSONB NOT NULL,
    source        TEXT NOT NULL DEFAULT 'wizard',          -- 'wizard' | 'wizard_migrated_from_anonymous' | 'manual'
    completed_at  TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ck_nis2_assess_org_completed
    ON ck_nis2_assessments (org_id, completed_at DESC);
