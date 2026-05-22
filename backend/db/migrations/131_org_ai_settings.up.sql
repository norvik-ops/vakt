-- S32-3: Per-Org AI Model Override.
-- Allows org admins to select the Ollama model and (Pro) a custom OpenAI-compatible
-- endpoint via the Org-Settings page instead of relying solely on ENV config.
-- NULL = use system default from VAKT_AI_MODEL / VAKT_AI_BASE_URL.
ALTER TABLE organizations
    ADD COLUMN IF NOT EXISTS ai_model_override    TEXT NULL,
    ADD COLUMN IF NOT EXISTS ai_base_url_override TEXT NULL;
