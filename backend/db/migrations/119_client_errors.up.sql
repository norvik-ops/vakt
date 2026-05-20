-- Self-hosted Frontend-Error-Tracking.
-- Frontend ErrorBoundary postet Errors hierher; Admins sehen sie unter
-- /admin/errors. Keine Telemetrie nach außen — alle Daten bleiben lokal.
CREATE TABLE client_errors (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID REFERENCES organizations(id) ON DELETE SET NULL,
    user_id         UUID REFERENCES users(id) ON DELETE SET NULL,
    message         TEXT NOT NULL,
    stack           TEXT,
    component_stack TEXT,
    url             TEXT,
    user_agent      TEXT,
    trace_id        TEXT,
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX client_errors_org_idx        ON client_errors(org_id, occurred_at DESC);
CREATE INDEX client_errors_occurred_idx   ON client_errors(occurred_at DESC);
