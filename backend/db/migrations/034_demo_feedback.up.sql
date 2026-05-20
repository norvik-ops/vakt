CREATE TABLE IF NOT EXISTS demo_feedback (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    rating     SMALLINT    NOT NULL CHECK (rating BETWEEN 1 AND 5),
    message    TEXT        NOT NULL,
    name       TEXT,
    email      TEXT,
    page       TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
