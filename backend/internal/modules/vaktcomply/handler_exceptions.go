package vaktcomply

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ListControlExceptions handles GET /api/v1/vaktcomply/exceptions[?control_id=:id]
// Without control_id, returns all exceptions for the organisation.
func (h *Handler) ListControlExceptions(c echo.Context) error {
	controlID := c.QueryParam("control_id")
	if controlID == "" {
		exceptions, err := h.service.ListAllControlExceptions(c.Request().Context(), orgID(c))
		if err != nil {
			log.Error().Err(err).Msg("list all control exceptions")
			return errResp(c, http.StatusInternalServerError, "failed to list exceptions", "CK_LIST_EXCEPTIONS_FAILED")
		}
		return c.JSON(http.StatusOK, exceptions)
	}
	if _, err := uuid.Parse(controlID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid control_id", "CK_INVALID_ID")
	}
	exceptions, err := h.service.ListControlExceptions(c.Request().Context(), orgID(c), controlID)
	if err != nil {
		log.Error().Err(err).Str("control_id", controlID).Msg("list control exceptions")
		return errResp(c, http.StatusInternalServerError, "failed to list exceptions", "CK_LIST_EXCEPTIONS_FAILED")
	}
	return c.JSON(http.StatusOK, exceptions)
}

// CreateControlException handles POST /api/v1/vaktcomply/controls/:controlId/exceptions
func (h *Handler) CreateControlException(c echo.Context) error {
	controlID := c.Param("controlId")
	if _, err := uuid.Parse(controlID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid control ID", "CK_INVALID_ID")
	}
	var in CreateControlExceptionInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	exception, err := h.service.CreateControlException(c.Request().Context(), orgID(c), controlID, in, userID(c))
	if err != nil {
		log.Error().Err(err).Str("control_id", controlID).Msg("create control exception")
		return errResp(c, http.StatusInternalServerError, "failed to create exception", "CK_CREATE_EXCEPTION_FAILED")
	}
	return c.JSON(http.StatusCreated, exception)
}

// UpdateControlException handles PUT /api/v1/vaktcomply/exceptions/:id
func (h *Handler) UpdateControlException(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid exception ID", "CK_INVALID_ID")
	}
	var in UpdateControlExceptionInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	exception, err := h.service.UpdateControlException(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResp(c, http.StatusNotFound, "exception not found", "CK_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("update control exception")
		return errResp(c, http.StatusInternalServerError, "failed to update exception", "CK_UPDATE_EXCEPTION_FAILED")
	}
	return c.JSON(http.StatusOK, exception)
}

// DeleteControlException handles DELETE /api/v1/vaktcomply/exceptions/:id
func (h *Handler) DeleteControlException(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid exception ID", "CK_INVALID_ID")
	}
	if err := h.service.DeleteControlException(c.Request().Context(), orgID(c), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResp(c, http.StatusNotFound, "exception not found", "CK_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("delete control exception")
		return errResp(c, http.StatusInternalServerError, "failed to delete exception", "CK_DELETE_EXCEPTION_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}
