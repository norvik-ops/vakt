-- OIDC state tracking for OAuth2 authorization code flow
CREATE TABLE auth_oidc_states (
    state       TEXT PRIMARY KEY,
    provider    TEXT NOT NULL,
    redirect_to TEXT,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
