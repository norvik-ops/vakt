// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S74-1: IT-Grundschutz-Check-Workflow + S74-2: Grundschutz-Cockpit

package vaktcomply

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	bsi "github.com/matharnica/vakt/internal/modules/vaktcomply/bsi"
)

// ListBSITargetObjects handles GET /api/v1/vaktcomply/bsi/target-objects
func (h *Handler) ListBSITargetObjects(c echo.Context) error {
	objects, err := h.service.BSI.ListBSITargetObjects(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list bsi target objects")
		return errResp(c, http.StatusInternalServerError, "failed to list target objects", "CK_BSI_TO_LIST_FAILED")
	}
	if objects == nil {
		objects = []bsi.BSITargetObject{}
	}
	return c.JSON(http.StatusOK, objects)
}

// CreateBSITargetObject handles POST /api/v1/vaktcomply/bsi/target-objects
func (h *Handler) CreateBSITargetObject(c echo.Context) error {
	var in bsi.CreateBSITargetObjectInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	obj, err := h.service.BSI.CreateBSITargetObject(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create bsi target object")
		return errResp(c, http.StatusInternalServerError, "failed to create target object", "CK_BSI_TO_CREATE_FAILED")
	}
	return c.JSON(http.StatusCreated, obj)
}

// GetBSITargetObject handles GET /api/v1/vaktcomply/bsi/target-objects/:id
func (h *Handler) GetBSITargetObject(c echo.Context) error {
	id := c.Param("id")
	obj, err := h.service.BSI.GetBSITargetObject(c.Request().Context(), orgID(c), id)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "target object not found", "CK_BSI_TO_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("get bsi target object")
		return errResp(c, http.StatusInternalServerError, "failed to get target object", "CK_BSI_TO_GET_FAILED")
	}
	return c.JSON(http.StatusOK, obj)
}

// UpdateBSITargetObject handles PUT /api/v1/vaktcomply/bsi/target-objects/:id
func (h *Handler) UpdateBSITargetObject(c echo.Context) error {
	id := c.Param("id")
	var in bsi.UpdateBSITargetObjectInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	obj, err := h.service.BSI.UpdateBSITargetObject(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "target object not found", "CK_BSI_TO_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("update bsi target object")
		return errResp(c, http.StatusInternalServerError, "failed to update target object", "CK_BSI_TO_UPDATE_FAILED")
	}
	return c.JSON(http.StatusOK, obj)
}

// DeleteBSITargetObject handles DELETE /api/v1/vaktcomply/bsi/target-objects/:id
func (h *Handler) DeleteBSITargetObject(c echo.Context) error {
	id := c.Param("id")
	if err := h.service.BSI.DeleteBSITargetObject(c.Request().Context(), orgID(c), id); err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "target object not found", "CK_BSI_TO_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("delete bsi target object")
		return errResp(c, http.StatusInternalServerError, "failed to delete target object", "CK_BSI_TO_DELETE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// AssignBausteinToTargetObject handles POST /api/v1/vaktcomply/bsi/target-objects/:id/modeling
func (h *Handler) AssignBausteinToTargetObject(c echo.Context) error {
	id := c.Param("id")
	var in bsi.AssignBausteinInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	if err := h.service.BSI.AssignBausteinToTargetObject(c.Request().Context(), orgID(c), id, in.BausteinID); err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "target object not found", "CK_BSI_TO_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("assign baustein")
		return errResp(c, http.StatusInternalServerError, "failed to assign baustein", "CK_BSI_ASSIGN_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// RemoveBausteinFromTargetObject handles DELETE /api/v1/vaktcomply/bsi/target-objects/:id/modeling/:bausteinId
func (h *Handler) RemoveBausteinFromTargetObject(c echo.Context) error {
	id := c.Param("id")
	bausteinID := c.Param("bausteinId")
	if err := h.service.BSI.RemoveBausteinFromTargetObject(c.Request().Context(), orgID(c), id, bausteinID); err != nil {
		log.Error().Err(err).Str("id", id).Msg("remove baustein")
		return errResp(c, http.StatusInternalServerError, "failed to remove baustein", "CK_BSI_REMOVE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// GetBSICheckSheet handles GET /api/v1/vaktcomply/bsi/target-objects/:id/check
func (h *Handler) GetBSICheckSheet(c echo.Context) error {
	id := c.Param("id")
	results, err := h.service.BSI.GetCheckSheet(c.Request().Context(), orgID(c), id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("get check sheet")
		return errResp(c, http.StatusInternalServerError, "failed to get check sheet", "CK_BSI_CHECK_FAILED")
	}
	if results == nil {
		results = []bsi.BSICheckResult{}
	}
	return c.JSON(http.StatusOK, results)
}

// SetBSICheckResult handles PUT /api/v1/vaktcomply/bsi/target-objects/:id/check/:anforderungId
func (h *Handler) SetBSICheckResult(c echo.Context) error {
	id := c.Param("id")
	anforderungID := c.Param("anforderungId")
	var in bsi.SetCheckResultInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	result, err := h.service.BSI.SetCheckResult(c.Request().Context(), orgID(c), id, anforderungID, in)
	if err != nil {
		if strings.Contains(err.Error(), "begruendung_required") {
			return errResp(c, http.StatusBadRequest, "Begründung ist bei Status 'entbehrlich' Pflicht", "BEGRUENDUNG_REQUIRED")
		}
		log.Error().Err(err).Str("id", id).Str("anforderung", anforderungID).Msg("set check result")
		return errResp(c, http.StatusInternalServerError, "failed to set check result", "CK_BSI_SET_RESULT_FAILED")
	}
	return c.JSON(http.StatusOK, result)
}

// BulkSetBSICheckResults handles POST /api/v1/vaktcomply/bsi/target-objects/:id/check/bulk
func (h *Handler) BulkSetBSICheckResults(c echo.Context) error {
	id := c.Param("id")
	var items []bsi.BulkCheckResultItem
	if err := c.Bind(&items); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.service.BSI.BulkSetCheckResults(c.Request().Context(), orgID(c), id, items); err != nil {
		if strings.Contains(err.Error(), "begruendung_required") {
			return errResp(c, http.StatusBadRequest, err.Error(), "BEGRUENDUNG_REQUIRED")
		}
		log.Error().Err(err).Str("id", id).Msg("bulk set check results")
		return errResp(c, http.StatusInternalServerError, "failed to bulk set check results", "CK_BSI_BULK_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// GetBSICheckSummary handles GET /api/v1/vaktcomply/bsi/target-objects/:id/check/summary
func (h *Handler) GetBSICheckSummary(c echo.Context) error {
	id := c.Param("id")
	summary, err := h.service.BSI.GetCheckSummary(c.Request().Context(), orgID(c), id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("get check summary")
		return errResp(c, http.StatusInternalServerError, "failed to get check summary", "CK_BSI_SUMMARY_FAILED")
	}
	return c.JSON(http.StatusOK, summary)
}

// GetBSICockpit handles GET /api/v1/vaktcomply/bsi/cockpit
func (h *Handler) GetBSICockpit(c echo.Context) error {
	cockpit, err := h.service.BSI.GetBSICockpit(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get bsi cockpit")
		return errResp(c, http.StatusInternalServerError, "failed to get BSI cockpit", "CK_BSI_COCKPIT_FAILED")
	}
	return c.JSON(http.StatusOK, cockpit)
}

// GetBSIGapReport handles GET /api/v1/vaktcomply/bsi/gap-report
func (h *Handler) GetBSIGapReport(c echo.Context) error {
	report, err := h.service.BSI.GetBSIGapReport(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get bsi gap report")
		return errResp(c, http.StatusInternalServerError, "failed to get BSI gap report", "CK_BSI_GAP_FAILED")
	}

	if c.QueryParam("format") == "csv" {
		csv := buildGapReportCSV(report)
		c.Response().Header().Set("Content-Type", "text/csv; charset=utf-8")
		c.Response().Header().Set("Content-Disposition", `attachment; filename="bsi-gap-report.csv"`)
		return c.String(http.StatusOK, csv)
	}

	return c.JSON(http.StatusOK, report)
}

func buildGapReportCSV(r bsi.BSIGapReport) string {
	var b strings.Builder
	b.WriteString("baustein_id,anforderung_id,anforderung_titel,zielobjekt,umsetzungsstatus,verantwortlicher,umsetzungsdatum\n")
	for _, g := range r.Gaps {
		b.WriteString(csvEscape(g.BausteinID) + "," +
			csvEscape(g.AnforderungID) + "," +
			csvEscape(g.AnforderungTitle) + "," +
			csvEscape(g.Zielobjekt) + "," +
			csvEscape(g.Umsetzungsstatus) + "," +
			csvEscape(g.Verantwortlicher) + "," +
			csvEscape(g.Umsetzungsdatum) + "\n")
	}
	return b.String()
}

func csvEscape(s string) string {
	if strings.ContainsAny(s, `",`+"\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}
