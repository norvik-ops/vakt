package github

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

var validate = validator.New()

// Handler handles HTTP requests for GitHub integrations.
type Handler struct {
	svc *Service
}

// NewHandler creates a new GitHub integration handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes wires GitHub integration routes under the provided echo group.
func RegisterRoutes(g *echo.Group, db *pgxpool.Pool, masterKey []byte) {
	svc := NewService(db, masterKey)
	h := NewHandler(svc)

	g.GET("", h.ListIntegrations)
	g.POST("", h.AddIntegration)
	g.DELETE("/:id", h.DeleteIntegration)
	g.POST("/:id/sync", h.SyncIntegration)
	g.GET("/:id/checks", h.ListCheckResults)
}

// ListIntegrations returns all GitHub integrations for the authenticated organisation.
func (h *Handler) ListIntegrations(c echo.Context) error {
	orgID := mustString(c, "org_id")
	integrations, err := h.svc.ListIntegrations(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("ListGitHubIntegrations failed")
		return serverError(c, err)
	}
	if integrations == nil {
		integrations = []Integration{}
	}
	return c.JSON(http.StatusOK, integrations)
}

// AddIntegration creates a new GitHub repository integration.
func (h *Handler) AddIntegration(c echo.Context) error {
	orgID := mustString(c, "org_id")

	var in AddIntegrationInput
	if err := c.Bind(&in); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := validate.Struct(in); err != nil {
		return validationError(c, err)
	}

	ig, err := h.svc.AddIntegration(c.Request().Context(), orgID, in)
	if err != nil {
		log.Error().Err(err).Msg("AddGitHubIntegration failed")
		return serverError(c, err)
	}
	return c.JSON(http.StatusCreated, ig)
}

// DeleteIntegration removes a GitHub integration.
func (h *Handler) DeleteIntegration(c echo.Context) error {
	orgID := mustString(c, "org_id")
	id := c.Param("id")
	if id == "" {
		return badRequest(c, "id is required")
	}

	if err := h.svc.DeleteIntegration(c.Request().Context(), orgID, id); err != nil {
		if err.Error() == "integration not found" {
			return c.JSON(http.StatusNotFound, errorResponse("integration not found", "NOT_FOUND"))
		}
		log.Error().Err(err).Msg("DeleteGitHubIntegration failed")
		return serverError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// SyncIntegration triggers an immediate compliance check sync for an integration.
func (h *Handler) SyncIntegration(c echo.Context) error {
	orgID := mustString(c, "org_id")
	id := c.Param("id")
	if id == "" {
		return badRequest(c, "id is required")
	}

	if err := h.svc.SyncIntegration(c.Request().Context(), orgID, id); err != nil {
		log.Error().Err(err).Msg("SyncGitHubIntegration failed")
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// ListCheckResults returns the latest check results for a GitHub integration.
func (h *Handler) ListCheckResults(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return badRequest(c, "id is required")
	}

	results, err := h.svc.ListCheckResults(c.Request().Context(), id)
	if err != nil {
		log.Error().Err(err).Msg("ListGitHubCheckResults failed")
		return serverError(c, err)
	}
	if results == nil {
		results = []StoredCheckResult{}
	}
	return c.JSON(http.StatusOK, results)
}

// --- helpers ---

func mustString(c echo.Context, key string) string {
	v, _ := c.Get(key).(string)
	return v
}

func badRequest(c echo.Context, msg string) error {
	return c.JSON(http.StatusBadRequest, errorResponse(msg, "BAD_REQUEST"))
}

func serverError(c echo.Context, err error) error {
	log.Error().Err(err).Msg("github operation failed")
	return c.JSON(http.StatusInternalServerError, errorResponse("internal server error", "INTERNAL_ERROR"))
}

func validationError(c echo.Context, err error) error {
	log.Error().Err(err).Msg("github validation error")
	return c.JSON(http.StatusUnprocessableEntity, errorResponse("validation error", "VALIDATION_ERROR"))
}

func errorResponse(msg, code string) map[string]string {
	return map[string]string{"error": msg, "code": code}
}
