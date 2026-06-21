// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S76-2: CIA-Schutzbedarfsvererbung — HTTP handlers

package vaktcomply

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	bsi "github.com/matharnica/vakt/internal/modules/vaktcomply/bsi"
)

// ListBSIObjectDependencies handles GET /api/v1/vaktcomply/bsi/target-objects/:id/dependencies
func (h *Handler) ListBSIObjectDependencies(c echo.Context) error {
	id := c.Param("id")
	deps, err := h.service.BSI.ListBSIObjectDependencies(c.Request().Context(), orgID(c), id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("list bsi object dependencies")
		return errResp(c, http.StatusInternalServerError, "failed to list dependencies", "CK_BSI_DEP_LIST_FAILED")
	}
	if deps == nil {
		deps = []bsi.BSIObjectDependency{}
	}
	return c.JSON(http.StatusOK, deps)
}

// CreateBSIObjectDependency handles POST /api/v1/vaktcomply/bsi/target-objects/:id/dependencies
func (h *Handler) CreateBSIObjectDependency(c echo.Context) error {
	sourceID := c.Param("id")
	var in bsi.CreateBSIObjectDependencyInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	dep, err := h.service.BSI.CreateBSIObjectDependency(c.Request().Context(), orgID(c), sourceID, in)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "target object not found", "CK_BSI_TO_NOT_FOUND")
		}
		if errors.Is(err, bsi.ErrCycle) {
			return errResp(c, http.StatusUnprocessableEntity, "adding this dependency would create a cycle", "CK_BSI_DEP_CYCLE")
		}
		if errors.Is(err, bsi.ErrConflict) {
			return errResp(c, http.StatusConflict, "dependency already exists", "CK_BSI_DEP_CONFLICT")
		}
		log.Error().Err(err).Str("source_id", sourceID).Msg("create bsi object dependency")
		return errResp(c, http.StatusInternalServerError, "failed to create dependency", "CK_BSI_DEP_CREATE_FAILED")
	}
	return c.JSON(http.StatusCreated, dep)
}

// DeleteBSIObjectDependency handles DELETE /api/v1/vaktcomply/bsi/target-objects/:id/dependencies/:depId
func (h *Handler) DeleteBSIObjectDependency(c echo.Context) error {
	depID := c.Param("depId")
	err := h.service.BSI.DeleteBSIObjectDependency(c.Request().Context(), orgID(c), depID)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "dependency not found", "CK_BSI_DEP_NOT_FOUND")
		}
		log.Error().Err(err).Str("dep_id", depID).Msg("delete bsi object dependency")
		return errResp(c, http.StatusInternalServerError, "failed to delete dependency", "CK_BSI_DEP_DELETE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// UpdateBSIObjectProtectionOverride handles PUT /api/v1/vaktcomply/bsi/target-objects/:id/protection-override
func (h *Handler) UpdateBSIObjectProtectionOverride(c echo.Context) error {
	id := c.Param("id")
	var in bsi.UpdateBSIObjectProtectionOverrideInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	obj, err := h.service.BSI.UpdateBSIObjectProtectionOverride(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "target object not found", "CK_BSI_TO_NOT_FOUND")
		}
		if errors.Is(err, bsi.ErrOverrideReasonMissing) {
			return errResp(c, http.StatusUnprocessableEntity, err.Error(), "CK_BSI_OVERRIDE_REASON_REQUIRED")
		}
		log.Error().Err(err).Str("id", id).Msg("update bsi protection override")
		return errResp(c, http.StatusInternalServerError, "failed to update protection override", "CK_BSI_OVERRIDE_FAILED")
	}
	return c.JSON(http.StatusOK, obj)
}
