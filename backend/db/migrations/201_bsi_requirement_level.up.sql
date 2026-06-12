-- S76-1: Add requirement_level to ck_controls for BSI Stufen (Basis/Standard/Erhöht).
-- Nullable to remain compatible with non-BSI frameworks.
ALTER TABLE ck_controls
    ADD COLUMN requirement_level TEXT CHECK (requirement_level IN ('basis','standard','erhoeht'));
