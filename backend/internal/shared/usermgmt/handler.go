package usermgmt

import (
	"errors"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/audit"
)

// Handler handles HTTP requests for user-management endpoints.
type Handler struct {
	svc      *Service
	validate *validator.Validate
}

// newHandler creates a Handler backed by the given Service.
func newHandler(svc *Service) *Handler {
	return &Handler{
		svc:      svc,
		validate: validator.New(),
	}
}

// ---------------------------------------------------------------------------
// Admin handlers (authenticated, admin role required)
// ---------------------------------------------------------------------------

// ListUsers handles GET /admin/users.
func (h *Handler) ListUsers(c echo.Context) error {
	orgID := contextOrgID(c)

	users, err := h.svc.ListUsers(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("list users")
		return errResp(c, http.StatusInternalServerError, "failed to list users", "USERMGMT_LIST_FAILED")
	}
	return c.JSON(http.StatusOK, users)
}

// UpdateUserRole handles PATCH /admin/users/:id/role.
func (h *Handler) UpdateUserRole(c echo.Context) error {
	orgID := contextOrgID(c)
	callerID := contextUserID(c)
	userID := c.Param("id")
	if userID == "" {
		return errResp(c, http.StatusBadRequest, "user id is required", "USERMGMT_BAD_REQUEST")
	}

	var in UpdateRoleInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "USERMGMT_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, err.Error(), "USERMGMT_VALIDATION_ERROR")
	}

	_ = callerID // available for audit log extension

	if err := h.svc.UpdateUserRole(c.Request().Context(), orgID, userID, in.Role); err != nil {
		log.Warn().Err(err).Str("user_id", userID).Msg("update user role")
		return errResp(c, http.StatusBadRequest, "Rolle konnte nicht aktualisiert werden", "USERMGMT_ROLE_UPDATE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// RemoveUser handles DELETE /admin/users/:id.
func (h *Handler) RemoveUser(c echo.Context) error {
	orgID := contextOrgID(c)
	callerID := contextUserID(c)
	userID := c.Param("id")
	if userID == "" {
		return errResp(c, http.StatusBadRequest, "user id is required", "USERMGMT_BAD_REQUEST")
	}

	if userID == callerID {
		return errResp(c, http.StatusBadRequest, "cannot remove yourself", "USERMGMT_SELF_REMOVE")
	}

	if err := h.svc.RemoveUser(c.Request().Context(), orgID, userID); err != nil {
		log.Warn().Err(err).Str("user_id", userID).Msg("remove user")
		return errResp(c, http.StatusBadRequest, "Benutzer konnte nicht entfernt werden", "USERMGMT_REMOVE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ResetUserMFA handles POST /admin/users/:id/reset-mfa — the MFA break-glass
// (S131-R-H23). Admin-only; clears the target member's TOTP + recovery codes so a
// user locked out of MFA (lost authenticator AND recovery codes) can re-enrol.
func (h *Handler) ResetUserMFA(c echo.Context) error {
	orgID := contextOrgID(c)
	callerID := contextUserID(c)
	callerEmail, _ := c.Get("user_email").(string)
	userID := c.Param("id")
	if userID == "" {
		return errResp(c, http.StatusBadRequest, "user id is required", "USERMGMT_BAD_REQUEST")
	}

	if err := h.svc.ResetUserMFA(c.Request().Context(), orgID, userID); err != nil {
		if errors.Is(err, ErrUserNotInOrg) {
			return errResp(c, http.StatusNotFound, "user not found in organisation", "USERMGMT_NOT_FOUND")
		}
		log.Warn().Err(err).Str("user_id", userID).Msg("reset user mfa")
		return errResp(c, http.StatusInternalServerError, "MFA-Reset fehlgeschlagen", "USERMGMT_MFA_RESET_FAILED")
	}

	// Security-sensitive break-glass — record who reset whose MFA.
	audit.Write(c.Request().Context(), h.svc.db, audit.WriteEntry{
		OrgID:        orgID,
		UserID:       callerID,
		UserEmail:    callerEmail,
		Action:       "reset_mfa",
		ResourceType: "auth/mfa",
		ResourceID:   userID,
		IPAddress:    c.RealIP(),
	})
	return c.NoContent(http.StatusNoContent)
}

// ListInvitations handles GET /admin/invitations.
func (h *Handler) ListInvitations(c echo.Context) error {
	orgID := contextOrgID(c)

	invs, err := h.svc.ListInvitations(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("list invitations")
		return errResp(c, http.StatusInternalServerError, "failed to list invitations", "USERMGMT_LIST_FAILED")
	}
	return c.JSON(http.StatusOK, invs)
}

// CreateInvitation handles POST /admin/invitations.
func (h *Handler) CreateInvitation(c echo.Context) error {
	orgID := contextOrgID(c)
	callerEmail := contextUserEmail(c)

	var in InviteInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "USERMGMT_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, err.Error(), "USERMGMT_VALIDATION_ERROR")
	}

	inv, err := h.svc.CreateInvitation(c.Request().Context(), orgID, callerEmail, in)
	if err != nil {
		log.Error().Err(err).Msg("create invitation")
		return errResp(c, http.StatusInternalServerError, "failed to create invitation", "USERMGMT_CREATE_FAILED")
	}
	return c.JSON(http.StatusCreated, inv)
}

// RevokeInvitation handles DELETE /admin/invitations/:id.
func (h *Handler) RevokeInvitation(c echo.Context) error {
	orgID := contextOrgID(c)
	id := c.Param("id")
	if id == "" {
		return errResp(c, http.StatusBadRequest, "invitation id is required", "USERMGMT_BAD_REQUEST")
	}

	if err := h.svc.RevokeInvitation(c.Request().Context(), orgID, id); err != nil {
		log.Warn().Err(err).Str("id", id).Msg("revoke invitation")
		return errResp(c, http.StatusNotFound, "invitation not found", "USERMGMT_NOT_FOUND")
	}
	return c.NoContent(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Public handlers (no authentication required)
// ---------------------------------------------------------------------------

// GetInvitationInfo handles GET /invite/info?token=<raw>.
func (h *Handler) GetInvitationInfo(c echo.Context) error {
	rawToken := c.QueryParam("token")
	if rawToken == "" {
		return errResp(c, http.StatusBadRequest, "token is required", "USERMGMT_BAD_REQUEST")
	}

	inv, err := h.svc.GetInvitationByToken(c.Request().Context(), rawToken)
	if err != nil {
		log.Debug().Err(err).Msg("get invitation by token")
		return errResp(c, http.StatusNotFound, "invitation not found or expired", "USERMGMT_INVITE_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, inv)
}

// AcceptInvitation handles POST /invite/accept.
func (h *Handler) AcceptInvitation(c echo.Context) error {
	var in AcceptInviteInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "USERMGMT_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, err.Error(), "USERMGMT_VALIDATION_ERROR")
	}

	if err := h.svc.AcceptInvitation(c.Request().Context(), in); err != nil {
		log.Debug().Err(err).Msg("accept invitation")
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Einladung konnte nicht angenommen werden",
			"code":  "INVITE_ACCEPT_ERROR",
		})
	}
	return c.JSON(http.StatusCreated, map[string]string{
		"message": "Konto erstellt — bitte einloggen",
	})
}

// ---------------------------------------------------------------------------
// Context helpers
// ---------------------------------------------------------------------------

func contextOrgID(c echo.Context) string {
	v, _ := c.Get("org_id").(string)
	return v
}

func contextUserID(c echo.Context) string {
	v, _ := c.Get("user_id").(string)
	return v
}

// contextUserEmail reads the caller's email from context. The email is stored
// by the admin-role middleware (see routes.go). Falls back to an empty string
// when not set (public routes).
func contextUserEmail(c echo.Context) string {
	v, _ := c.Get("user_email").(string)
	return v
}

// errResp returns a standardised JSON error response.
func errResp(c echo.Context, code int, msg, errCode string) error {
	return c.JSON(code, map[string]string{
		"error": msg,
		"code":  errCode,
	})
}
