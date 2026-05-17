-- name: GetUserModulePermissions :many
SELECT * FROM user_module_permissions WHERE org_id = $1 AND user_id = $2;

-- name: UpsertUserModulePermission :exec
INSERT INTO user_module_permissions (org_id, user_id, module, can_read, can_write)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (org_id, user_id, module)
DO UPDATE SET can_read = $4, can_write = $5, updated_at = now();

-- name: DeleteUserModulePermissions :exec
DELETE FROM user_module_permissions WHERE org_id = $1 AND user_id = $2;
