ALTER TABLE vb_assets
    DROP CONSTRAINT IF EXISTS vb_assets_type_check;

UPDATE vb_assets SET type = 'webapp'     WHERE type = 'web_app';
UPDATE vb_assets SET type = 'repository' WHERE type = 'repo';
-- Note: rows with type='database' have no prior equivalent; they remain as-is.

ALTER TABLE vb_assets
    ADD CONSTRAINT vb_assets_type_check
        CHECK (type IN ('server', 'container', 'webapp', 'repository'));
