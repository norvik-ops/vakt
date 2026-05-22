ALTER TABLE organizations
    DROP COLUMN IF EXISTS ai_model_override,
    DROP COLUMN IF EXISTS ai_base_url_override;
