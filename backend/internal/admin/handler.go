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

	"github.com/sechealth-app/sechealth/internal/shared/notify"
)

// Handler holds HTTP handler methods for admin endpoints.
type Handler struct {
	service  *Service
	validate *validator.Validate
}

// NewHandler constructs an admin Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{
		service:  service,
		validate: validator.New(),
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

	return c.JSON(http.StatusOK, map[string]interface{}{
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

	return c.JSON(http.StatusOK, map[string]interface{}{
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
		log.Error().Err(err).Str("org_id", orgID).Str("email", input.Email).Msg("invite user failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to invite user",
			"code":  "ADMIN_INVITE_ERROR",
		})
	}

	return c.JSON(http.StatusCreated, map[string]string{
		"message": "invitation created",
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
	return c.JSON(http.StatusOK, map[string]interface{}{
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

	return c.JSON(http.StatusOK, map[string]interface{}{
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

// CreateManagedOrg handles POST /api/v1/admin/organizations.
func (h *Handler) CreateManagedOrg(c echo.Context) error {
	mspOrgID, _ := c.Get("org_id").(string)
	userID, _ := c.Get("user_id").(string)

	var input CreateManagedOrgInput
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

	org, err := h.service.MSP.CreateManagedOrg(c.Request().Context(), mspOrgID, userID, input)
	if err != nil {
		log.Error().Err(err).Str("msp_org_id", mspOrgID).Msg("create managed org failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to create managed organization",
			"code":  "MSP_CREATE_ORG_ERROR",
		})
	}

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"data": org,
	})
}

// ListManagedOrgs handles GET /api/v1/admin/organizations.
func (h *Handler) ListManagedOrgs(c echo.Context) error {
	mspOrgID, _ := c.Get("org_id").(string)

	orgs, err := h.service.MSP.ListManagedOrgs(c.Request().Context(), mspOrgID)
	if err != nil {
		log.Error().Err(err).Str("msp_org_id", mspOrgID).Msg("list managed orgs failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to list managed organizations",
			"code":  "MSP_LIST_ORGS_ERROR",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": orgs,
	})
}

// DeleteManagedOrg handles DELETE /api/v1/admin/organizations/:id.
func (h *Handler) DeleteManagedOrg(c echo.Context) error {
	mspOrgID, _ := c.Get("org_id").(string)
	targetOrgID := c.Param("id")

	if err := h.service.MSP.DeleteManagedOrg(c.Request().Context(), mspOrgID, targetOrgID); err != nil {
		if isNotManagedByError(err) {
			return c.JSON(http.StatusForbidden, map[string]string{
				"error": "organization is not managed by your MSP account",
				"code":  "MSP_FORBIDDEN",
			})
		}
		log.Error().Err(err).
			Str("msp_org_id", mspOrgID).
			Str("target_org_id", targetOrgID).
			Msg("delete managed org failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to schedule organization deletion",
			"code":  "MSP_DELETE_ORG_ERROR",
		})
	}

	return c.JSON(http.StatusAccepted, map[string]string{
		"message": "organization deletion scheduled",
	})
}

// GetOrgBranding handles GET /api/v1/admin/organizations/:id/branding.
func (h *Handler) GetOrgBranding(c echo.Context) error {
	orgID := c.Param("id")

	branding, err := h.service.MSP.GetOrgBranding(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("get org branding failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve organization branding",
			"code":  "MSP_GET_BRANDING_ERROR",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": branding,
	})
}

// UpdateOrgBranding handles PUT /api/v1/admin/organizations/:id/branding.
func (h *Handler) UpdateOrgBranding(c echo.Context) error {
	mspOrgID, _ := c.Get("org_id").(string)
	targetOrgID := c.Param("id")

	var input BrandingInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "ADMIN_BAD_REQUEST",
		})
	}

	if err := h.service.MSP.UpdateOrgBranding(c.Request().Context(), mspOrgID, targetOrgID, input); err != nil {
		if isNotManagedByError(err) {
			return c.JSON(http.StatusForbidden, map[string]string{
				"error": "organization is not managed by your MSP account",
				"code":  "MSP_FORBIDDEN",
			})
		}
		log.Error().Err(err).
			Str("msp_org_id", mspOrgID).
			Str("target_org_id", targetOrgID).
			Msg("update org branding failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to update organization branding",
			"code":  "MSP_UPDATE_BRANDING_ERROR",
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "branding updated",
	})
}

// GetCurrentOrg handles GET /api/v1/admin/org.
// Returns the caller's own organisation record, including trust center settings.
func (h *Handler) GetCurrentOrg(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)

	org, err := h.service.MSP.repo.GetCurrentOrg(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("get current org failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve organization",
			"code":  "ADMIN_ORG_ERROR",
		})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"data": org})
}

// UpdateTrustCenter handles PUT /api/v1/admin/trust-center.
type UpdateTrustCenterInput struct {
	Enabled     bool   `json:"enabled"`
	Description string `json:"description"`
	Contact     string `json:"contact"`
}

func (h *Handler) UpdateTrustCenter(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	var in UpdateTrustCenterInput
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid input",
			"code":  "ADMIN_BAD_REQUEST",
		})
	}
	if err := h.service.MSP.repo.UpdateOrgTrustCenter(c.Request().Context(), orgID, in.Enabled, in.Description, in.Contact); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("update trust center failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
			"code":  "ADMIN_TRUST_CENTER_ERROR",
		})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// isNotManagedByError returns true when the service error signals an MSP ownership violation.
func isNotManagedByError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	needle := "is not managed by"
	for i := 0; i+len(needle) <= len(msg); i++ {
		if msg[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// derefString safely dereferences a string pointer for CSV output.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

