// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ── BIA Processes ─────────────────────────────────────────────────────────────

// ListBIAProcesses handles GET /api/v1/vaktcomply/bia/processes.
func (h *Handler) ListBIAProcesses(c echo.Context) error {
	processes, err := h.service.ListBIAProcesses(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list bia processes")
		return errResp(c, http.StatusInternalServerError, "failed to list BIA processes", "CK_LIST_BIA_FAILED")
	}
	if processes == nil {
		processes = []BIAProcess{}
	}
	return c.JSON(http.StatusOK, processes)
}

// CreateBIAProcess handles POST /api/v1/vaktcomply/bia/processes.
func (h *Handler) CreateBIAProcess(c echo.Context) error {
	var in CreateBIAProcessInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	p, err := h.service.CreateBIAProcess(c.Request().Context(), orgID(c), in)
	if err != nil {
		if err == ErrRPOExceedsRTO || err == ErrMBCOOutOfRange {
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": err.Error(), "code": "VALIDATION_ERROR"})
		}
		log.Error().Err(err).Msg("create bia process")
		return errResp(c, http.StatusInternalServerError, "failed to create BIA process", "CK_CREATE_BIA_FAILED")
	}
	return c.JSON(http.StatusCreated, p)
}

// GetBIAProcess handles GET /api/v1/vaktcomply/bia/processes/:id.
func (h *Handler) GetBIAProcess(c echo.Context) error {
	p, err := h.service.GetBIAProcess(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		return errResp(c, http.StatusNotFound, "BIA process not found", "CK_BIA_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, p)
}

// UpdateBIAProcess handles PUT /api/v1/vaktcomply/bia/processes/:id.
func (h *Handler) UpdateBIAProcess(c echo.Context) error {
	var in UpdateBIAProcessInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	p, err := h.service.UpdateBIAProcess(c.Request().Context(), orgID(c), c.Param("id"), in)
	if err != nil {
		if err == ErrRPOExceedsRTO || err == ErrMBCOOutOfRange {
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": err.Error(), "code": "VALIDATION_ERROR"})
		}
		log.Error().Err(err).Str("id", c.Param("id")).Msg("update bia process")
		return errResp(c, http.StatusInternalServerError, "failed to update BIA process", "CK_UPDATE_BIA_FAILED")
	}
	return c.JSON(http.StatusOK, p)
}

// DeleteBIAProcess handles DELETE /api/v1/vaktcomply/bia/processes/:id.
func (h *Handler) DeleteBIAProcess(c echo.Context) error {
	if err := h.service.DeleteBIAProcess(c.Request().Context(), orgID(c), c.Param("id")); err != nil {
		log.Error().Err(err).Str("id", c.Param("id")).Msg("delete bia process")
		return errResp(c, http.StatusInternalServerError, "failed to delete BIA process", "CK_DELETE_BIA_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// GetBIASummary handles GET /api/v1/vaktcomply/bia/summary.
func (h *Handler) GetBIASummary(c echo.Context) error {
	summary, err := h.service.GetBIASummary(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get bia summary")
		return errResp(c, http.StatusInternalServerError, "failed to get BIA summary", "CK_BIA_SUMMARY_FAILED")
	}
	return c.JSON(http.StatusOK, summary)
}

// ── Recovery Plans ─────────────────────────────────────────────────────────────

// ListRecoveryPlans handles GET /api/v1/vaktcomply/bcm/recovery-plans.
func (h *Handler) ListRecoveryPlans(c echo.Context) error {
	biaID := c.QueryParam("bia_id")
	var plans []RecoveryPlan
	var err error
	if biaID != "" {
		plans, err = h.service.ListRecoveryPlansByBIAProcess(c.Request().Context(), orgID(c), biaID)
	} else {
		plans, err = h.service.ListRecoveryPlans(c.Request().Context(), orgID(c))
	}
	if err != nil {
		log.Error().Err(err).Msg("list recovery plans")
		return errResp(c, http.StatusInternalServerError, "failed to list recovery plans", "CK_LIST_RECOVERY_PLANS_FAILED")
	}
	if plans == nil {
		plans = []RecoveryPlan{}
	}
	return c.JSON(http.StatusOK, plans)
}

// CreateRecoveryPlan handles POST /api/v1/vaktcomply/bcm/recovery-plans.
func (h *Handler) CreateRecoveryPlan(c echo.Context) error {
	var in CreateRecoveryPlanInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	plan, err := h.service.CreateRecoveryPlan(c.Request().Context(), orgID(c), in)
	if err != nil {
		if err == ErrRTORequired || err == ErrStepsOrderInvalid {
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": err.Error(), "code": "VALIDATION_ERROR"})
		}
		log.Error().Err(err).Msg("create recovery plan")
		return errResp(c, http.StatusInternalServerError, "failed to create recovery plan", "CK_CREATE_RECOVERY_PLAN_FAILED")
	}
	return c.JSON(http.StatusCreated, plan)
}

// GetRecoveryPlan handles GET /api/v1/vaktcomply/bcm/recovery-plans/:id.
func (h *Handler) GetRecoveryPlan(c echo.Context) error {
	plan, err := h.service.GetRecoveryPlan(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		return errResp(c, http.StatusNotFound, "recovery plan not found", "CK_RECOVERY_PLAN_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, plan)
}

// UpdateRecoveryPlan handles PUT /api/v1/vaktcomply/bcm/recovery-plans/:id.
func (h *Handler) UpdateRecoveryPlan(c echo.Context) error {
	var in UpdateRecoveryPlanInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	plan, err := h.service.UpdateRecoveryPlan(c.Request().Context(), orgID(c), c.Param("id"), in)
	if err != nil {
		if err == ErrRTORequired || err == ErrStepsOrderInvalid {
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": err.Error(), "code": "VALIDATION_ERROR"})
		}
		log.Error().Err(err).Str("id", c.Param("id")).Msg("update recovery plan")
		return errResp(c, http.StatusInternalServerError, "failed to update recovery plan", "CK_UPDATE_RECOVERY_PLAN_FAILED")
	}
	return c.JSON(http.StatusOK, plan)
}

// DeleteRecoveryPlan handles DELETE /api/v1/vaktcomply/bcm/recovery-plans/:id.
func (h *Handler) DeleteRecoveryPlan(c echo.Context) error {
	if err := h.service.DeleteRecoveryPlan(c.Request().Context(), orgID(c), c.Param("id")); err != nil {
		log.Error().Err(err).Str("id", c.Param("id")).Msg("delete recovery plan")
		return errResp(c, http.StatusInternalServerError, "failed to delete recovery plan", "CK_DELETE_RECOVERY_PLAN_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ── Emergency Contacts ────────────────────────────────────────────────────────

// ListEmergencyContacts handles GET /api/v1/vaktcomply/bcm/emergency-contacts.
func (h *Handler) ListEmergencyContacts(c echo.Context) error {
	contacts, err := h.service.ListEmergencyContacts(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list emergency contacts")
		return errResp(c, http.StatusInternalServerError, "failed to list emergency contacts", "CK_LIST_EMERGENCY_CONTACTS_FAILED")
	}
	if contacts == nil {
		contacts = []EmergencyContact{}
	}
	return c.JSON(http.StatusOK, contacts)
}

// CreateEmergencyContact handles POST /api/v1/vaktcomply/bcm/emergency-contacts.
func (h *Handler) CreateEmergencyContact(c echo.Context) error {
	var in CreateEmergencyContactInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	contact, err := h.service.CreateEmergencyContact(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create emergency contact")
		return errResp(c, http.StatusInternalServerError, "failed to create emergency contact", "CK_CREATE_EMERGENCY_CONTACT_FAILED")
	}
	return c.JSON(http.StatusCreated, contact)
}

// UpdateEmergencyContact handles PUT /api/v1/vaktcomply/bcm/emergency-contacts/:id.
func (h *Handler) UpdateEmergencyContact(c echo.Context) error {
	var in UpdateEmergencyContactInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	contact, err := h.service.UpdateEmergencyContact(c.Request().Context(), orgID(c), c.Param("id"), in)
	if err != nil {
		log.Error().Err(err).Str("id", c.Param("id")).Msg("update emergency contact")
		return errResp(c, http.StatusInternalServerError, "failed to update emergency contact", "CK_UPDATE_EMERGENCY_CONTACT_FAILED")
	}
	return c.JSON(http.StatusOK, contact)
}

// DeleteEmergencyContact handles DELETE /api/v1/vaktcomply/bcm/emergency-contacts/:id.
func (h *Handler) DeleteEmergencyContact(c echo.Context) error {
	if err := h.service.DeleteEmergencyContact(c.Request().Context(), orgID(c), c.Param("id")); err != nil {
		log.Error().Err(err).Str("id", c.Param("id")).Msg("delete emergency contact")
		return errResp(c, http.StatusInternalServerError, "failed to delete emergency contact", "CK_DELETE_EMERGENCY_CONTACT_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// GetBCMReadinessScore handles GET /api/v1/vaktcomply/bcm/readiness-score.
func (h *Handler) GetBCMReadinessScore(c echo.Context) error {
	score, err := h.service.GetBCMReadinessScore(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get bcm readiness score")
		return errResp(c, http.StatusInternalServerError, "failed to get BCM readiness score", "CK_BCM_SCORE_FAILED")
	}
	return c.JSON(http.StatusOK, score)
}
