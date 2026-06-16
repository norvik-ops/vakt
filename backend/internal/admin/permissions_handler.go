// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package admin

import (
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/auth"
)

// UserModulePermission mirrors what sqlc would generate from the
// user_module_permissions table (migration 086).
type UserModulePermission struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	UserID    string    `json:"user_id"`
	Module    string    `json:"module"`
	CanRead   bool      `json:"can_read"`
	CanWrite  bool      `json:"can_write"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ModulePermissionInput is one entry in the PUT request body.
type ModulePermissionInput struct {
	Module   string `json:"module"   validate:"required,oneof=vaktscan vaktcomply vaktvault vaktaware vaktprivacy vakthr"`
	CanRead  bool   `json:"can_read"`
	CanWrite bool   `json:"can_write"`
}

// UpdatePermissionsInput is the full request body for PUT /admin/users/:user_id/permissions.
type UpdatePermissionsInput struct {
	Permissions []ModulePermissionInput `json:"permissions" validate:"required,dive"`
}

// PermissionsHandler holds HTTP handler methods for the user module permissions endpoints.
type PermissionsHandler struct {
	db       *pgxpool.Pool
	validate *validator.Validate
	rdb      *redis.Client // optional — used to invalidate the module-permission cache (S90-4)
}

// WithRedis attaches a Redis client so permission updates invalidate the
// per-user module-permission cache immediately (S90-4). No-op if never called.
func (h *PermissionsHandler) WithRedis(rdb *redis.Client) *PermissionsHandler {
	h.rdb = rdb
	return h
}

// NewPermissionsHandler constructs a PermissionsHandler backed by the given pool.
func NewPermissionsHandler(db *pgxpool.Pool) *PermissionsHandler {
	return &PermissionsHandler{
		db:       db,
		validate: validator.New(),
	}
}

// GetPermissions handles GET /api/v1/admin/users/:user_id/permissions.
// Returns the current module-level permissions for the given user within the caller's org.
func (h *PermissionsHandler) GetPermissions(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	targetUserID := c.Param("user_id")

	rows, err := h.db.Query(c.Request().Context(), `
		SELECT id::text, org_id::text, user_id::text, module,
		       can_read, can_write, created_at, updated_at
		FROM user_module_permissions
		WHERE org_id = $1::uuid AND user_id = $2::uuid
		ORDER BY module ASC`,
		orgID, targetUserID)
	if err != nil {
		log.Error().Err(err).
			Str("org_id", orgID).
			Str("user_id", targetUserID).
			Msg("get user module permissions failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve permissions",
			"code":  "ADMIN_PERMISSIONS_ERROR",
		})
	}
	defer rows.Close()

	perms := make([]UserModulePermission, 0)
	for rows.Next() {
		var p UserModulePermission
		if err := rows.Scan(
			&p.ID, &p.OrgID, &p.UserID, &p.Module,
			&p.CanRead, &p.CanWrite, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			log.Error().Err(err).Msg("scan user module permission row failed")
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to read permissions",
				"code":  "ADMIN_PERMISSIONS_ERROR",
			})
		}
		perms = append(perms, p)
	}
	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("iterate user module permission rows failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve permissions",
			"code":  "ADMIN_PERMISSIONS_ERROR",
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"data": perms,
	})
}

// UpdatePermissions handles PUT /api/v1/admin/users/:user_id/permissions.
// Replaces all module permissions for the given user within the caller's org.
// This is a Pro feature — the route must be registered with features.Require().
func (h *PermissionsHandler) UpdatePermissions(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	targetUserID := c.Param("user_id")

	var input UpdatePermissionsInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "ADMIN_BAD_REQUEST",
		})
	}
	if err := h.validate.Struct(input); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": err.Error(),
			"code":  "ADMIN_VALIDATION_ERROR",
		})
	}

	ctx := c.Request().Context()

	tx, err := h.db.Begin(ctx)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Str("user_id", targetUserID).
			Msg("begin transaction for update permissions failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to update permissions",
			"code":  "ADMIN_PERMISSIONS_ERROR",
		})
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	// Delete all existing permissions for this user in this org, then upsert
	// the full set provided in the request body. This is an intentional
	// replace-all semantic so the caller always sends the complete desired state.
	_, err = tx.Exec(ctx, `
		DELETE FROM user_module_permissions
		WHERE org_id = $1::uuid AND user_id = $2::uuid`,
		orgID, targetUserID)
	if err != nil {
		log.Error().Err(err).
			Str("org_id", orgID).
			Str("user_id", targetUserID).
			Msg("delete existing module permissions failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to update permissions",
			"code":  "ADMIN_PERMISSIONS_ERROR",
		})
	}

	for _, perm := range input.Permissions {
		_, err = tx.Exec(ctx, `
			INSERT INTO user_module_permissions (org_id, user_id, module, can_read, can_write)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5)
			ON CONFLICT (org_id, user_id, module)
			DO UPDATE SET can_read = $4, can_write = $5, updated_at = now()`,
			orgID, targetUserID, perm.Module, perm.CanRead, perm.CanWrite)
		if err != nil {
			log.Error().Err(err).
				Str("org_id", orgID).
				Str("user_id", targetUserID).
				Str("module", perm.Module).
				Msg("upsert module permission failed")
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to update permissions",
				"code":  "ADMIN_PERMISSIONS_ERROR",
			})
		}
	}

	if err = tx.Commit(ctx); err != nil {
		log.Error().Err(err).
			Str("org_id", orgID).
			Str("user_id", targetUserID).
			Msg("commit update permissions transaction failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to update permissions",
			"code":  "ADMIN_PERMISSIONS_ERROR",
		})
	}

	// S90-4: drop the cached permission state so the change takes effect
	// immediately rather than after the cache TTL.
	auth.InvalidateModulePermissions(ctx, h.rdb, orgID, targetUserID)

	return c.JSON(http.StatusOK, map[string]string{
		"message": "permissions updated",
	})
}
