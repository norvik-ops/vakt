-- Auditor-Einladungen (zeitlich begrenzt, token-basiert)
CREATE TABLE IF NOT EXISTS auditor_invites (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email         TEXT NOT NULL,
    token_hash    TEXT NOT NULL UNIQUE,
    invited_by    UUID REFERENCES users(id) ON DELETE SET NULL,
    expires_at    TIMESTAMPTZ NOT NULL,
    accepted_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS auditor_invites_org_idx ON auditor_invites(org_id);
CREATE INDEX IF NOT EXISTS auditor_invites_token_idx ON auditor_invites(token_hash);

-- Aktive Auditor-Sessions (kurzlebige Tokens mit eingeschränkten Claims)
CREATE TABLE IF NOT EXISTS auditor_sessions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    invite_id     UUID NOT NULL REFERENCES auditor_invites(id) ON DELETE CASCADE,
    token_hash    TEXT NOT NULL UNIQUE,
    auditor_email TEXT NOT NULL,
    expires_at    TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
