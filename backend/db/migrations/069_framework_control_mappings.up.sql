-- DO NOT USE TRANSACTIONS — consistent with project pattern (see migration 037)
CREATE TABLE IF NOT EXISTS ck_framework_control_mappings (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  source_framework TEXT NOT NULL,
  source_control_code TEXT NOT NULL,
  target_framework TEXT NOT NULL,
  target_control_code TEXT NOT NULL,
  mapping_type TEXT NOT NULL DEFAULT 'equivalent', -- 'equivalent' | 'partial' | 'informative'
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(source_framework, source_control_code, target_framework, target_control_code)
);
CREATE INDEX IF NOT EXISTS idx_cfm_source ON ck_framework_control_mappings(source_framework, source_control_code);
CREATE INDEX IF NOT EXISTS idx_cfm_target ON ck_framework_control_mappings(target_framework, target_control_code);
