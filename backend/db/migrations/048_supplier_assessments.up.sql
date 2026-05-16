-- 048_supplier_assessments.up.sql
CREATE TABLE ck_supplier_assessments (
  id               UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
  org_id           UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  supplier_id      UUID        NOT NULL REFERENCES ck_suppliers(id) ON DELETE CASCADE,
  questionnaire_id UUID        NOT NULL REFERENCES ck_questionnaires(id),
  token_hash       TEXT        UNIQUE NOT NULL,
  expires_at       TIMESTAMPTZ NOT NULL,
  status           TEXT        NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending','in_progress','submitted','reviewed')),
  submitted_at     TIMESTAMPTZ,
  submitted_by_ip  TEXT,
  user_agent       TEXT,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE ck_supplier_answers (
  id             UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
  assessment_id  UUID        NOT NULL REFERENCES ck_supplier_assessments(id) ON DELETE CASCADE,
  question_id    UUID        NOT NULL REFERENCES ck_questionnaire_questions(id) ON DELETE CASCADE,
  answer_text    TEXT,
  answer_bool    BOOLEAN,
  answer_options JSONB,
  file_url       TEXT,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (assessment_id, question_id)
);
CREATE INDEX idx_ck_supplier_assessments_token ON ck_supplier_assessments(token_hash);
CREATE INDEX idx_ck_supplier_assessments_org ON ck_supplier_assessments(org_id);
CREATE INDEX idx_ck_supplier_answers_assessment ON ck_supplier_answers(assessment_id);
