CREATE TABLE totp_secrets (
    user_id      UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    secret       TEXT    NOT NULL,
    enabled      BOOLEAN NOT NULL DEFAULT false,
    backup_codes TEXT[]  NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
