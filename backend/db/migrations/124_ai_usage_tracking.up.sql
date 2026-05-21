-- Sprint 15 S15-2: AI-Usage-Tracking pro Org.
-- Persistiert Token-Counts und (geschätzte) Kosten je AI-Call, damit Admins
-- Quotas durchsetzen können (VAKT_AI_DAILY_TOKEN_LIMIT_PER_ORG) und damit das
-- UI später eine Cost-Visibility hat ("diesen Monat: 142k Tokens, ~3,42 €").
--
-- Schema-Design:
--  - org_id: required, jeder Call gehört zu einer Org (auch system-level Calls
--    laufen über eine technische Org).
--  - model: das verwendete Modell (qwen2.5:3b, gpt-4o-mini, …). Macht Filterung
--    nach Provider trivial.
--  - tokens_in / tokens_out: Token-Counts. Bei lokalem Ollama exakt verfügbar,
--    bei manchen Providern nur geschätzt (dann markiert via NULL — bewusst
--    nicht 0).
--  - cost_micro_eur: Mikrocent EUR (skaliert auf int8, vermeidet float-
--    Rundungsfehler bei Aggregation). Lokales Modell → 0; Cloud-LLMs → Preis
--    pro Token aus Config.
--  - duration_ms: Wallclock vom Request bis Response, hilft bei Latenz-Tracking.
--  - status: ok | rate_limited | timeout | provider_error | cache_hit.

CREATE TABLE IF NOT EXISTS ai_usage (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    model           TEXT NOT NULL,
    tokens_in       INTEGER,
    tokens_out      INTEGER,
    cost_micro_eur  BIGINT NOT NULL DEFAULT 0,
    duration_ms     INTEGER,
    status          TEXT NOT NULL,
    request_id      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Quota-Lookup: tagesaggregat pro Org. Index optimiert das WHERE created_at >= today.
CREATE INDEX IF NOT EXISTS idx_ai_usage_org_created
    ON ai_usage (org_id, created_at DESC);

-- Provider-Auswertung (welches Modell wie viel kostet diesen Monat).
CREATE INDEX IF NOT EXISTS idx_ai_usage_org_model_created
    ON ai_usage (org_id, model, created_at DESC);
