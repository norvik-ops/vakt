-- Email templates
CREATE TABLE pg_templates (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    subject     TEXT NOT NULL,
    from_name   TEXT NOT NULL,
    from_email  TEXT NOT NULL,
    html_body   TEXT NOT NULL,
    attack_type TEXT NOT NULL DEFAULT 'phishing' CHECK (attack_type IN ('phishing','vishing','usb','smishing')),
    is_preset   BOOLEAN NOT NULL DEFAULT FALSE,
    created_by  UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Target groups
CREATE TABLE pg_target_groups (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    source     TEXT NOT NULL DEFAULT 'manual' CHECK (source IN ('manual','csv','active_directory')),
    ad_ou      TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Individual targets within groups
CREATE TABLE pg_targets (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id       UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    group_id     UUID NOT NULL REFERENCES pg_target_groups(id) ON DELETE CASCADE,
    email        TEXT NOT NULL,
    first_name   TEXT NOT NULL DEFAULT '',
    last_name    TEXT NOT NULL DEFAULT '',
    department   TEXT NOT NULL DEFAULT '',
    is_bounced   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(group_id, email)
);

-- Landing pages
CREATE TABLE pg_landing_pages (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    html_content TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Campaigns
CREATE TABLE pg_campaigns (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','scheduled','running','completed','aborted')),
    template_id     UUID REFERENCES pg_templates(id) ON DELETE SET NULL,
    group_id        UUID REFERENCES pg_target_groups(id) ON DELETE SET NULL,
    landing_page_id UUID REFERENCES pg_landing_pages(id) ON DELETE SET NULL,
    from_name       TEXT NOT NULL,
    from_email      TEXT NOT NULL,
    subject         TEXT NOT NULL,
    scheduled_at    TIMESTAMPTZ,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    recurrence      TEXT CHECK (recurrence IN ('none','monthly','quarterly')),
    next_run_at     TIMESTAMPTZ,
    track_opens     BOOLEAN NOT NULL DEFAULT TRUE,
    betriebsrat_mode BOOLEAN NOT NULL DEFAULT FALSE,
    created_by      UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Tracking events
CREATE TABLE pg_events (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id       UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    campaign_id  UUID NOT NULL REFERENCES pg_campaigns(id) ON DELETE CASCADE,
    target_id    UUID REFERENCES pg_targets(id) ON DELETE SET NULL,
    department   TEXT,
    type         TEXT NOT NULL CHECK (type IN ('open','click','form_submission')),
    tracking_token TEXT NOT NULL,
    ip_address   TEXT,
    user_agent   TEXT,
    occurred_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pg_events_campaign_id ON pg_events(campaign_id);
CREATE INDEX idx_pg_events_target_id   ON pg_events(target_id) WHERE target_id IS NOT NULL;
CREATE INDEX idx_pg_targets_group_id   ON pg_targets(group_id);
CREATE INDEX idx_pg_campaigns_org_id   ON pg_campaigns(org_id, status);
CREATE INDEX idx_pg_events_token       ON pg_events(tracking_token);
