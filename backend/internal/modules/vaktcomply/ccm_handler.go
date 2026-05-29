package vaktcomply

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ListCCMChecks handles GET /ccm/checks.
func (h *Handler) ListCCMChecks(c echo.Context) error {
	checks, err := h.service.ListCCMChecks(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list ccm checks")
		return errResp(c, http.StatusInternalServerError, "failed to list CCM checks", "CCM_LIST_FAILED")
	}
	if checks == nil {
		checks = []CCMCheck{}
	}
	return c.JSON(http.StatusOK, checks)
}

// CreateCCMCheck handles POST /ccm/checks.
func (h *Handler) CreateCCMCheck(c echo.Context) error {
	var in CreateCCMCheckInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CCM_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}

	check, err := h.service.CreateCCMCheck(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create ccm check")
		return errResp(c, http.StatusInternalServerError, "failed to create CCM check", "CCM_CREATE_FAILED")
	}
	return c.JSON(http.StatusCreated, check)
}

// DeleteCCMCheck handles DELETE /ccm/checks/:id.
func (h *Handler) DeleteCCMCheck(c echo.Context) error {
	id := c.Param("id")
	if err := h.service.DeleteCCMCheck(c.Request().Context(), orgID(c), id); err != nil {
		log.Error().Err(err).Str("id", id).Msg("delete ccm check")
		return errResp(c, http.StatusInternalServerError, "failed to delete CCM check", "CCM_DELETE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ToggleCCMCheck handles PATCH /ccm/checks/:id/toggle.
func (h *Handler) ToggleCCMCheck(c echo.Context) error {
	id := c.Param("id")

	var in ToggleCCMCheckInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CCM_BAD_REQUEST")
	}

	if err := h.service.ToggleCCMCheck(c.Request().Context(), orgID(c), id, in.Enabled); err != nil {
		log.Error().Err(err).Str("id", id).Msg("toggle ccm check")
		return errResp(c, http.StatusInternalServerError, "failed to toggle CCM check", "CCM_TOGGLE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// TriggerCCMCheck handles POST /ccm/checks/:id/run — runs a check immediately.
func (h *Handler) TriggerCCMCheck(c echo.Context) error {
	id := c.Param("id")

	result, err := h.service.RunCCMCheck(c.Request().Context(), orgID(c), id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("trigger ccm check")
		return errResp(c, http.StatusInternalServerError, "failed to run CCM check", "CCM_RUN_FAILED")
	}
	return c.JSON(http.StatusOK, result)
}

// ListCCMResults handles GET /ccm/checks/:id/results.
func (h *Handler) ListCCMResults(c echo.Context) error {
	id := c.Param("id")

	results, err := h.service.ListCCMResults(c.Request().Context(), orgID(c), id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("list ccm results")
		return errResp(c, http.StatusInternalServerError, "failed to list CCM results", "CCM_RESULTS_FAILED")
	}
	if results == nil {
		results = []CCMResult{}
	}
	return c.JSON(http.StatusOK, results)
}
