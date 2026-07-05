// Package admin provides admin panel endpoints for audit logs, user management,
// and module status.
package admin

import (
	"encoding/csv"
	"net/http"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/logsafe"
	"github.com/matharnica/vakt/internal/shared/notify"
)

// Handler holds HTTP handler methods for admin endpoints.
type Handler struct {
	service     *Service
	validate    *validator.Validate
	Permissions *PermissionsHandler
}

// NewHandler constructs an admin Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{
		service:     service,
		validate:    validator.New(),
		Permissions: NewPermissionsHandler(service.db),
	}
}

// ListAuditLogs handles GET /api/v1/admin/audit-logs.
// Supports ?page=1&limit=25&user_id=&action=&resource_type= and ?format=csv.
func (h *Handler) ListAuditLogs(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)

	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	userFilter := c.QueryParam("user_id")
	actionFilter := c.QueryParam("action")
	resourceFilter := c.QueryParam("resource_type")

	logs, total, err := h.service.ListAuditLogs(
		c.Request().Context(), orgID, page, limit, userFilter, actionFilter, resourceFilter,
	)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("list audit logs failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve audit logs",
			"code":  "ADMIN_AUDIT_ERROR",
		})
	}

	// CSV export
	if c.QueryParam("format") == "csv" {
		c.Response().Header().Set("Content-Disposition", `attachment; filename="audit-logs.csv"`)
		c.Response().Header().Set("Content-Type", "text/csv")
		w := csv.NewWriter(c.Response().Writer)
		if err := w.Write([]string{
			"id", "org_id", "user_id", "action", "resource_type",
			"resource_id", "ip_address", "timestamp",
		}); err != nil {
			return err
		}
		for _, l := range logs {
			row := []string{
				l.ID, l.OrgID,
				derefString(l.UserID),
				l.Action, l.ResourceType,
				derefString(l.ResourceID),
				derefString(l.IPAddress),
				l.Timestamp.String(),
			}
			if err := w.Write(row); err != nil {
				return err
			}
		}
		w.Flush()
		return w.Error()
	}

	return c.JSON(http.StatusOK, map[string]any{
		"data":  logs,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// ListUsers handles GET /api/v1/admin/users.
func (h *Handler) ListUsers(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)

	members, err := h.service.ListUsers(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("list users failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve users",
			"code":  "ADMIN_USERS_ERROR",
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"data": members,
	})
}

// InviteUser handles POST /api/v1/admin/users/invite.
func (h *Handler) InviteUser(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	inviterID, _ := c.Get("user_id").(string)

	var input InviteInput
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

	if err := h.service.InviteUser(c.Request().Context(), orgID, inviterID, input); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Str("email_redacted", logsafe.RedactEmail(input.Email)).Msg("invite user failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to invite user",
			"code":  "ADMIN_INVITE_ERROR",
		})
	}

	return c.JSON(http.StatusCreated, map[string]string{
		"message": "invitation created",
	})
}

// CreateUser handles POST /api/v1/admin/users.
// Creates a user directly (no email invite, no SMTP required). The user is
// immediately active. Admin sees the initial password; it cannot be retrieved again.
func (h *Handler) CreateUser(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	creatorID, _ := c.Get("user_id").(string)

	var input CreateUserInput
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

	result, err := h.service.CreateUser(c.Request().Context(), orgID, creatorID, input)
	if err != nil {
		if err.Error() == "create user: email already exists" {
			return c.JSON(http.StatusConflict, map[string]string{
				"error": "email already in use",
				"code":  "ADMIN_USER_EXISTS",
			})
		}
		log.Error().Err(err).Str("org_id", orgID).Str("email_redacted", logsafe.RedactEmail(input.Email)).Msg("create user failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to create user",
			"code":  "ADMIN_CREATE_USER_ERROR",
		})
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"user_id": result.UserID,
		"email":   input.Email,
		"role":    input.Role,
	})
}

// UpdateUserRole handles PATCH /api/v1/admin/users/:id/role.
func (h *Handler) UpdateUserRole(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	targetUserID := c.Param("id")

	var input RoleUpdateInput
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

	if err := h.service.UpdateUserRole(c.Request().Context(), orgID, targetUserID, input); err != nil {
		if err.Error() == "user not found in org" {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "user not found",
				"code":  "ADMIN_USER_NOT_FOUND",
			})
		}
		log.Error().Err(err).Str("target_user_id", targetUserID).Msg("update user role failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to update role",
			"code":  "ADMIN_ROLE_ERROR",
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "role updated",
	})
}

// ListModules handles GET /api/v1/admin/modules.
func (h *Handler) ListModules(c echo.Context) error {
	modules := h.service.ListModules()
	return c.JSON(http.StatusOK, map[string]any{
		"data": modules,
	})
}

// ListNotificationChannels handles GET /api/v1/admin/notifications/channels.
func (h *Handler) ListNotificationChannels(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)

	channels, err := h.service.ListNotificationChannels(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("list notification channels failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve notification channels",
			"code":  "ADMIN_NOTIFY_CHANNELS_ERROR",
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"data": channels,
	})
}

// CreateNotificationChannel handles POST /api/v1/admin/notifications/channels.
func (h *Handler) CreateNotificationChannel(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)

	var input notify.CreateChannelInput
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

	ch, err := h.service.CreateNotificationChannel(c.Request().Context(), orgID, input)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("create notification channel failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to create notification channel",
			"code":  "ADMIN_NOTIFY_CREATE_ERROR",
		})
	}

	return c.JSON(http.StatusCreated, ch)
}

// DeleteNotificationChannel handles DELETE /api/v1/admin/notifications/channels/:id.
func (h *Handler) DeleteNotificationChannel(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	channelID := c.Param("id")

	if err := h.service.DeleteNotificationChannel(c.Request().Context(), orgID, channelID); err != nil {
		if err.Error() == "notification channel not found" {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "notification channel not found",
				"code":  "ADMIN_NOTIFY_NOT_FOUND",
			})
		}
		log.Error().Err(err).Str("channel_id", channelID).Msg("delete notification channel failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to delete notification channel",
			"code":  "ADMIN_NOTIFY_DELETE_ERROR",
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "notification channel deleted",
	})
}
