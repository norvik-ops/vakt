package auditor

import (
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// Handler handles HTTP requests for auditor invite management.
type Handler struct {
	service  *Service
	validate *validator.Validate
}

// NewHandler constructs a new auditor Handler.
func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{
		service:  NewService(db),
		validate: validator.New(),
	}
}

// RegisterRoutes wires the authenticated admin routes for auditor invite management.
// The provided group must already be behind the user auth middleware.
//
//	POST   /auditor/invites       — create invite
//	GET    /auditor/invites       — list invites
//	DELETE /auditor/invites/:id   — revoke invite
func RegisterRoutes(g *echo.Group, db *pgxpool.Pool) {
	h := NewHandler(db)
	g.POST("/invites", h.CreateInvite)
	g.GET("/invites", h.ListInvites)
	g.DELETE("/invites/:id", h.RevokeInvite)
}

// RegisterPublicRoutes wires the unauthenticated route for accepting an invite.
//
//	POST /auditor/accept/:token — accept invite, returns session token
func RegisterPublicRoutes(g *echo.Group, db *pgxpool.Pool) {
	h := NewHandler(db)
	g.POST("/accept/:token", h.AcceptInvite)
}

// CreateInvite handles POST /auditor/invites.
func (h *Handler) CreateInvite(c echo.Context) error {
	orgID := contextOrgID(c)
	userID := contextUserID(c)

	var in CreateInviteInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "AUDITOR_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusBadRequest, err.Error(), "AUDITOR_VALIDATION_ERROR")
	}

	inv, rawToken, err := h.service.CreateInvite(c.Request().Context(), orgID, userID, in)
	if err != nil {
		log.Error().Err(err).Msg("create auditor invite")
		return errResp(c, http.StatusInternalServerError, "failed to create invite", "AUDITOR_CREATE_FAILED")
	}

	inviteURL := fmt.Sprintf("/auditor/accept/%s", rawToken)
	return c.JSON(http.StatusCreated, map[string]any{
		"invite":     inv,
		"token":      rawToken,
		"invite_url": inviteURL,
	})
}

// ListInvites handles GET /auditor/invites.
func (h *Handler) ListInvites(c echo.Context) error {
	orgID := contextOrgID(c)

	invites, err := h.service.ListInvites(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("list auditor invites")
		return errResp(c, http.StatusInternalServerError, "failed to list invites", "AUDITOR_LIST_FAILED")
	}

	return c.JSON(http.StatusOK, invites)
}

// RevokeInvite handles DELETE /auditor/invites/:id.
func (h *Handler) RevokeInvite(c echo.Context) error {
	orgID := contextOrgID(c)
	id := c.Param("id")
	if id == "" {
		return errResp(c, http.StatusBadRequest, "invite id is required", "AUDITOR_BAD_REQUEST")
	}

	if err := h.service.RevokeInvite(c.Request().Context(), orgID, id); err != nil {
		log.Error().Err(err).Str("id", id).Msg("revoke auditor invite")
		return errResp(c, http.StatusNotFound, "invite not found", "AUDITOR_NOT_FOUND")
	}

	return c.NoContent(http.StatusNoContent)
}

// AcceptInvite handles POST /auditor/accept/:token (public, no auth).
func (h *Handler) AcceptInvite(c echo.Context) error {
	token := c.Param("token")
	if token == "" {
		return errResp(c, http.StatusBadRequest, "token is required", "AUDITOR_BAD_REQUEST")
	}

	sessionToken, err := h.service.AcceptInvite(c.Request().Context(), token)
	if err != nil {
		log.Debug().Err(err).Msg("accept auditor invite")
		return errResp(c, http.StatusUnauthorized, "invalid or expired invite token", "AUDITOR_INVALID_INVITE")
	}

	return c.JSON(http.StatusOK, map[string]string{
		"session_token": sessionToken,
		"message":       "Auditor-Zugang aktiviert. Verwende diesen Token als Bearer-Token in deinen API-Anfragen.",
	})
}

// contextOrgID reads org_id set by auth middleware.
func contextOrgID(c echo.Context) string {
	v, _ := c.Get("org_id").(string)
	return v
}

// contextUserID reads user_id set by auth middleware.
func contextUserID(c echo.Context) string {
	v, _ := c.Get("user_id").(string)
	return v
}

// errResp returns a standardised JSON error response.
func errResp(c echo.Context, code int, msg, errCode string) error {
	return c.JSON(code, map[string]string{
		"error": msg,
		"code":  errCode,
	})
}
