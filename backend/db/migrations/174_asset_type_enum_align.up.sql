-- Align vb_assets.type CHECK constraint with OpenAPI spec and frontend type system.
-- Old values: server, container, webapp, repository
-- New values: server, container, web_app, database, repo
--
-- The old values 'webapp' and 'repository' were renamed when the frontend and
-- OpenAPI spec were updated but the DB constraint was not (contract drift since
-- migration 005). 'database' is a new type added in the frontend type system.

ALTER TABLE vb_assets
    DROP CONSTRAINT IF EXISTS vb_assets_type_check;

-- Migrate existing rows to the new canonical values before adding the constraint.
UPDATE vb_assets SET type = 'web_app'  WHERE type = 'webapp';
UPDATE vb_assets SET type = 'repo'     WHERE type = 'repository';

ALTER TABLE vb_assets
    ADD CONSTRAINT vb_assets_type_check
        CHECK (type IN ('server', 'container', 'web_app', 'database', 'repo'));
