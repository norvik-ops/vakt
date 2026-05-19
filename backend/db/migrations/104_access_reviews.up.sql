CREATE TABLE ck_access_review_campaigns (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id         UUID NOT NULL,
  title          TEXT NOT NULL,
  description    TEXT,
  status         TEXT NOT NULL DEFAULT 'draft',
  reviewer_email TEXT NOT NULL,
  scope          TEXT,
  due_date       TIMESTAMPTZ,
  completed_at   TIMESTAMPTZ,
  created_by     TEXT,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON ck_access_review_campaigns(org_id, created_at DESC);

CREATE TABLE ck_access_review_items (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  campaign_id      UUID NOT NULL REFERENCES ck_access_review_campaigns(id) ON DELETE CASCADE,
  org_id           UUID NOT NULL,
  user_email       TEXT NOT NULL,
  access_level     TEXT NOT NULL,
  decision         TEXT NOT NULL DEFAULT 'pending',
  reviewer_comment TEXT,
  decided_at       TIMESTAMPTZ,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON ck_access_review_items(campaign_id);
CREATE INDEX ON ck_access_review_items(org_id);
