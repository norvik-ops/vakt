// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ListBCPPlans handles GET /api/v1/vaktcomply/bcp/plans.
func (h *Handler) ListBCPPlans(c echo.Context) error {
	plans, err := h.service.ListBCPPlans(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list bcp plans")
		return errResp(c, http.StatusInternalServerError, "failed to list BCP plans", "CK_LIST_BCP_PLANS_FAILED")
	}
	if plans == nil {
		plans = []BCPPlan{}
	}
	return c.JSON(http.StatusOK, plans)
}

// CreateBCPPlan handles POST /api/v1/vaktcomply/bcp/plans.
func (h *Handler) CreateBCPPlan(c echo.Context) error {
	var in CreateBCPPlanInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	plan, err := h.service.CreateBCPPlan(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create bcp plan")
		return errResp(c, http.StatusInternalServerError, "failed to create BCP plan", "CK_CREATE_BCP_PLAN_FAILED")
	}
	return c.JSON(http.StatusCreated, plan)
}

// GetBCPPlan handles GET /api/v1/vaktcomply/bcp/plans/:id.
func (h *Handler) GetBCPPlan(c echo.Context) error {
	id := c.Param("id")
	plan, err := h.service.GetBCPPlan(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusNotFound, "BCP plan not found", "CK_BCP_PLAN_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, plan)
}

// UpdateBCPPlan handles PATCH /api/v1/vaktcomply/bcp/plans/:id.
func (h *Handler) UpdateBCPPlan(c echo.Context) error {
	id := c.Param("id")
	var in UpdateBCPPlanInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	plan, err := h.service.UpdateBCPPlan(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		log.Error().Err(err).Str("plan_id", id).Msg("update bcp plan")
		return errResp(c, http.StatusInternalServerError, "failed to update BCP plan", "CK_UPDATE_BCP_PLAN_FAILED")
	}
	return c.JSON(http.StatusOK, plan)
}

// DeleteBCPPlan handles DELETE /api/v1/vaktcomply/bcp/plans/:id.
func (h *Handler) DeleteBCPPlan(c echo.Context) error {
	id := c.Param("id")
	if err := h.service.DeleteBCPPlan(c.Request().Context(), orgID(c), id); err != nil {
		log.Error().Err(err).Str("plan_id", id).Msg("delete bcp plan")
		return errResp(c, http.StatusInternalServerError, "failed to delete BCP plan", "CK_DELETE_BCP_PLAN_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ListBCPTests handles GET /api/v1/vaktcomply/bcp/plans/:id/tests.
func (h *Handler) ListBCPTests(c echo.Context) error {
	planID := c.Param("id")
	tests, err := h.service.ListBCPTests(c.Request().Context(), orgID(c), planID)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "BCP plan not found", "CK_BCP_PLAN_NOT_FOUND")
		}
		log.Error().Err(err).Str("plan_id", planID).Msg("list bcp tests")
		return errResp(c, http.StatusInternalServerError, "failed to list BCP tests", "CK_LIST_BCP_TESTS_FAILED")
	}
	if tests == nil {
		tests = []BCPTest{}
	}
	return c.JSON(http.StatusOK, tests)
}

// AddBCPTest handles POST /api/v1/vaktcomply/bcp/plans/:id/tests.
func (h *Handler) AddBCPTest(c echo.Context) error {
	planID := c.Param("id")
	var in CreateBCPTestInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	test, err := h.service.AddBCPTest(c.Request().Context(), orgID(c), planID, in)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "BCP plan not found", "CK_BCP_PLAN_NOT_FOUND")
		}
		log.Error().Err(err).Str("plan_id", planID).Msg("add bcp test")
		return errResp(c, http.StatusInternalServerError, "failed to add BCP test", "CK_ADD_BCP_TEST_FAILED")
	}
	return c.JSON(http.StatusCreated, test)
}

// LinkBCPPlanAsEvidence handles POST /api/v1/vaktcomply/bcp/plans/:id/evidence.
// If a control_id is provided in the body, the BCP plan title is recorded as
// evidence on that control. If no control_id is provided, the request is a no-op
// and returns 200.
func (h *Handler) LinkBCPPlanAsEvidence(c echo.Context) error {
	planID := c.Param("id")
	var body LinkBCPPlanEvidenceInput
	// Bind is best-effort; an empty body is valid (no-op path).
	_ = c.Bind(&body)

	if body.ControlID == "" {
		// No control requested — return 200 silently.
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}

	plan, err := h.service.GetBCPPlan(c.Request().Context(), orgID(c), planID)
	if err != nil {
		return errResp(c, http.StatusNotFound, "BCP plan not found", "CK_BCP_PLAN_NOT_FOUND")
	}

	input := AddEvidenceInput{
		Title:       "BCP: " + plan.Title,
		Description: "BCP plan linked as compliance evidence (version " + plan.Version + ")",
		Source:      "bcp",
	}
	ev, err := h.service.AddEvidence(c.Request().Context(), orgID(c), body.ControlID, userID(c), input)
	if err != nil {
		log.Error().Err(err).Str("plan_id", planID).Str("control_id", body.ControlID).Msg("link bcp plan as evidence")
		return errResp(c, http.StatusInternalServerError, "failed to link BCP plan as evidence", "CK_LINK_BCP_EVIDENCE_FAILED")
	}
	return c.JSON(http.StatusCreated, ev)
}
