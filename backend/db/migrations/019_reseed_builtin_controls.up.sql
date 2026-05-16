-- Remove stale builtin controls so they are reseeded with correct IDs on next startup.
-- Evidence cascades on DELETE per FK constraint; no user data is lost in dev.
DELETE FROM ck_controls
WHERE framework_id IN (
    SELECT id FROM ck_frameworks WHERE is_builtin = true
);
