// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S88-3: Gefährdungs-/Maßnahmen-Katalog HTTP handlers.

package vaktcomply

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ListThreatCatalog handles GET /api/v1/vaktcomply/threat-catalog
// Optional query filters: ?framework=ISO27001&asset_type=data&cia=confidentiality
func (h *Handler) ListThreatCatalog(c echo.Context) error {
	items := h.service.ListThreatCatalog(ThreatCatalogFilter{
		Framework: c.QueryParam("framework"),
		AssetType: c.QueryParam("asset_type"),
		CIA:       c.QueryParam("cia"),
	})
	return c.JSON(http.StatusOK, items)
}

// CreateRiskFromCatalog handles POST /api/v1/vaktcomply/threat-catalog/create-risk
func (h *Handler) CreateRiskFromCatalog(c echo.Context) error {
	var in CreateRiskFromCatalogInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, err.Error(), "CK_VALIDATION_ERROR")
	}
	risk, err := h.service.CreateRiskFromCatalog(c.Request().Context(), orgID(c), in, userID(c))
	if err != nil {
		log.Warn().Err(err).Str("catalog_id", in.CatalogID).Msg("create risk from catalog")
		return errResp(c, http.StatusBadRequest, err.Error(), "CK_THREAT_CATALOG_FAILED")
	}
	return c.JSON(http.StatusCreated, risk)
}
