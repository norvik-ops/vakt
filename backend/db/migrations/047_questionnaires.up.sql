CREATE TABLE ck_questionnaires (
  id              UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
  org_id          UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  name            TEXT        NOT NULL,
  description     TEXT,
  is_template     BOOLEAN     NOT NULL DEFAULT false,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE ck_questionnaire_questions (
  id               UUID    PRIMARY KEY DEFAULT uuid_generate_v4(),
  questionnaire_id UUID    NOT NULL REFERENCES ck_questionnaires(id) ON DELETE CASCADE,
  order_idx        INT     NOT NULL DEFAULT 0,
  question_text    TEXT    NOT NULL,
  question_type    TEXT    NOT NULL CHECK (question_type IN ('yes_no','multiple_choice','free_text','file_upload')),
  options          JSONB,
  required         BOOLEAN NOT NULL DEFAULT true,
  control_id       UUID    REFERENCES ck_controls(id) ON DELETE SET NULL,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_ck_questionnaire_questions_q ON ck_questionnaire_questions(questionnaire_id, order_idx);
CREATE INDEX idx_ck_questionnaires_org_id ON ck_questionnaires (org_id);
CREATE UNIQUE INDEX idx_ck_questionnaires_org_name_template ON ck_questionnaires (org_id, name, is_template);
