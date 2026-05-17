-- Index for GetRiskTrend (org_id, status, created_at for date-range queries)
CREATE INDEX IF NOT EXISTS idx_vb_findings_trend
  ON vb_findings(org_id, status, created_at)
  WHERE status = 'open';

-- Index for dashboard top-5 risks; risk_score is a STORED generated column
-- (likelihood * impact) on ck_risks.
CREATE INDEX IF NOT EXISTS idx_ck_risks_score
  ON ck_risks(org_id, risk_score DESC);
