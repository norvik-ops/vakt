// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// GetISMSScope handles GET /api/v1/vaktcomply/isms-scope.
func (h *Handler) GetISMSScope(c echo.Context) error {
	scope, err := h.service.GetCurrentISMSScope(c.Request().Context(), orgID(c))
	if err != nil {
		if isNotFound(err) {
			return c.JSON(http.StatusOK, nil)
		}
		log.Error().Err(err).Msg("get isms scope")
		return errResp(c, http.StatusInternalServerError, "failed to get ISMS scope", "CK_GET_ISMS_SCOPE_FAILED")
	}
	return c.JSON(http.StatusOK, scope)
}

// CreateOrUpdateISMSScope handles POST /api/v1/vaktcomply/isms-scope.
func (h *Handler) CreateOrUpdateISMSScope(c echo.Context) error {
	var in CreateISMSScopeInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	scope, err := h.service.CreateOrVersionISMSScope(c.Request().Context(), orgID(c), userID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create isms scope")
		return errResp(c, http.StatusInternalServerError, "failed to save ISMS scope", "CK_CREATE_ISMS_SCOPE_FAILED")
	}
	return c.JSON(http.StatusCreated, scope)
}

// ListISMSScopeVersions handles GET /api/v1/vaktcomply/isms-scope/versions.
func (h *Handler) ListISMSScopeVersions(c echo.Context) error {
	versions, err := h.service.ListISMSScopeVersions(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list isms scope versions")
		return errResp(c, http.StatusInternalServerError, "failed to list ISMS scope versions", "CK_LIST_ISMS_SCOPE_VERSIONS_FAILED")
	}
	if versions == nil {
		versions = []ISMSScope{}
	}
	return c.JSON(http.StatusOK, versions)
}

// ApproveISMSScope handles POST /api/v1/vaktcomply/isms-scope/approve.
func (h *Handler) ApproveISMSScope(c echo.Context) error {
	var body struct {
		ID string `json:"id"`
	}
	if err := c.Bind(&body); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if body.ID == "" {
		return errResp(c, http.StatusBadRequest, "id is required", "CK_BAD_REQUEST")
	}
	userRole, _ := c.Get("role").(string)
	scope, err := h.service.ApproveISMSScope(c.Request().Context(), orgID(c), body.ID, userID(c), userRole)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "ISMS scope not found", "CK_ISMS_SCOPE_NOT_FOUND")
		}
		log.Error().Err(err).Msg("approve isms scope")
		return errResp(c, http.StatusForbidden, err.Error(), "CK_ISMS_SCOPE_APPROVE_FORBIDDEN")
	}
	return c.JSON(http.StatusOK, scope)
}

// ExportISMSScopePDF handles GET /api/v1/vaktcomply/isms-scope/export-pdf.
func (h *Handler) ExportISMSScopePDF(c echo.Context) error {
	return c.JSON(http.StatusNotImplemented, map[string]string{"message": "PDF export coming soon"})
}
