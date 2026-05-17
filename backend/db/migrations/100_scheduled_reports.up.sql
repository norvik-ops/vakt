CREATE TABLE scheduled_reports (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  report_type TEXT NOT NULL CHECK (report_type IN ('compliance','findings','risk')),
  schedule TEXT NOT NULL CHECK (schedule IN ('weekly','monthly','quarterly')),
  recipients TEXT[] NOT NULL DEFAULT '{}',
  format TEXT NOT NULL DEFAULT 'pdf' CHECK (format IN ('pdf','csv')),
  active BOOLEAN NOT NULL DEFAULT true,
  last_run_at TIMESTAMPTZ,
  next_run_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON scheduled_reports(org_id);
CREATE INDEX ON scheduled_reports(next_run_at) WHERE active = true;
