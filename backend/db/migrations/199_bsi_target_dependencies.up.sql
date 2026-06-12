CREATE TABLE ck_bsi_target_object_dependencies (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    source_id       UUID NOT NULL REFERENCES ck_bsi_target_objects(id) ON DELETE CASCADE,
    target_id       UUID NOT NULL REFERENCES ck_bsi_target_objects(id) ON DELETE CASCADE,
    dependency_type TEXT NOT NULL CHECK (dependency_type IN (
                        'runs_on',
                        'located_in',
                        'connected_to',
                        'processes_for'
                    )),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, source_id, target_id, dependency_type),
    CHECK (source_id <> target_id)
);

CREATE INDEX idx_bsi_obj_deps_org       ON ck_bsi_target_object_dependencies(org_id);
CREATE INDEX idx_bsi_obj_deps_source    ON ck_bsi_target_object_dependencies(source_id);
CREATE INDEX idx_bsi_obj_deps_target    ON ck_bsi_target_object_dependencies(target_id);
