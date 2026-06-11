// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S74-3: Risikobewertung BSI 200-3 Handler

package vaktcomply

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ListBSIThreats handles GET /api/v1/vaktcomply/bsi/threats
func (h *Handler) ListBSIThreats(c echo.Context) error {
	threats, err := h.service.ListBSIThreats(c.Request().Context())
	if err != nil {
		log.Error().Err(err).Msg("list bsi threats")
		return errResp(c, http.StatusInternalServerError, "failed to list threats", "CK_BSI_THREATS_FAILED")
	}
	if threats == nil {
		threats = []BSIThreat{}
	}
	return c.JSON(http.StatusOK, threats)
}

// ListBSIRisks handles GET /api/v1/vaktcomply/bsi/target-objects/:id/risks
func (h *Handler) ListBSIRisks(c echo.Context) error {
	id := c.Param("id")
	risks, err := h.service.ListBSIRisks(c.Request().Context(), orgID(c), id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("list bsi risks")
		return errResp(c, http.StatusInternalServerError, "failed to list risks", "CK_BSI_RISK_LIST_FAILED")
	}
	if risks == nil {
		risks = []BSIRiskAssessment{}
	}
	return c.JSON(http.StatusOK, risks)
}

// CreateBSIRisk handles POST /api/v1/vaktcomply/bsi/target-objects/:id/risks
func (h *Handler) CreateBSIRisk(c echo.Context) error {
	id := c.Param("id")
	var in CreateBSIRiskInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	risk, err := h.service.CreateBSIRisk(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("create bsi risk")
		return errResp(c, http.StatusInternalServerError, "failed to create risk", "CK_BSI_RISK_CREATE_FAILED")
	}
	return c.JSON(http.StatusCreated, risk)
}

// UpdateBSIRisk handles PUT /api/v1/vaktcomply/bsi/target-objects/:id/risks/:riskId
func (h *Handler) UpdateBSIRisk(c echo.Context) error {
	id := c.Param("id")
	riskID := c.Param("riskId")
	var in UpdateBSIRiskInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	risk, err := h.service.UpdateBSIRisk(c.Request().Context(), orgID(c), id, riskID, in)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "risk not found", "CK_BSI_RISK_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Str("riskId", riskID).Msg("update bsi risk")
		return errResp(c, http.StatusInternalServerError, "failed to update risk", "CK_BSI_RISK_UPDATE_FAILED")
	}
	return c.JSON(http.StatusOK, risk)
}

// DeleteBSIRisk handles DELETE /api/v1/vaktcomply/bsi/target-objects/:id/risks/:riskId
func (h *Handler) DeleteBSIRisk(c echo.Context) error {
	id := c.Param("id")
	riskID := c.Param("riskId")
	if err := h.service.DeleteBSIRisk(c.Request().Context(), orgID(c), id, riskID); err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "risk not found", "CK_BSI_RISK_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Str("riskId", riskID).Msg("delete bsi risk")
		return errResp(c, http.StatusInternalServerError, "failed to delete risk", "CK_BSI_RISK_DELETE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// GetBSIRiskSummary handles GET /api/v1/vaktcomply/bsi/target-objects/:id/risks/summary
func (h *Handler) GetBSIRiskSummary(c echo.Context) error {
	id := c.Param("id")
	summary, err := h.service.GetBSIRiskSummary(c.Request().Context(), orgID(c), id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("get bsi risk summary")
		return errResp(c, http.StatusInternalServerError, "failed to get risk summary", "CK_BSI_RISK_SUMMARY_FAILED")
	}
	return c.JSON(http.StatusOK, summary)
}
