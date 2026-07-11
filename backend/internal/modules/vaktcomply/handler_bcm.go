package vaktcomply

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	bcm "github.com/matharnica/vakt/internal/modules/vaktcomply/bcm"
	"github.com/rs/zerolog/log"
)

func (h *Handler) ListBCPPlans(c echo.Context) error {
	plans, err := h.service.BCM.ListBCPPlans(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list bcp plans")
		return errResp(c, http.StatusInternalServerError, "failed to list BCP plans", "CK_LIST_BCP_PLANS_FAILED")
	}
	if plans == nil {
		plans = []bcm.BCPPlan{}
	}
	return c.JSON(http.StatusOK, plans)
}

// CreateBCPPlan handles POST /api/v1/vaktcomply/bcp/plans.
func (h *Handler) CreateBCPPlan(c echo.Context) error {
	var in bcm.CreateBCPPlanInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	plan, err := h.service.BCM.CreateBCPPlan(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create bcp plan")
		return errResp(c, http.StatusInternalServerError, "failed to create BCP plan", "CK_CREATE_BCP_PLAN_FAILED")
	}
	return c.JSON(http.StatusCreated, plan)
}

// GetBCPPlan handles GET /api/v1/vaktcomply/bcp/plans/:id.
func (h *Handler) GetBCPPlan(c echo.Context) error {
	id := c.Param("id")
	plan, err := h.service.BCM.GetBCPPlan(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusNotFound, "BCP plan not found", "CK_BCP_PLAN_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, plan)
}

// UpdateBCPPlan handles PATCH /api/v1/vaktcomply/bcp/plans/:id.
func (h *Handler) UpdateBCPPlan(c echo.Context) error {
	id := c.Param("id")
	var in bcm.UpdateBCPPlanInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	plan, err := h.service.BCM.UpdateBCPPlan(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		log.Error().Err(err).Str("plan_id", id).Msg("update bcp plan")
		return errResp(c, http.StatusInternalServerError, "failed to update BCP plan", "CK_UPDATE_BCP_PLAN_FAILED")
	}
	return c.JSON(http.StatusOK, plan)
}

// DeleteBCPPlan handles DELETE /api/v1/vaktcomply/bcp/plans/:id.
func (h *Handler) DeleteBCPPlan(c echo.Context) error {
	id := c.Param("id")
	if err := h.service.BCM.DeleteBCPPlan(c.Request().Context(), orgID(c), id); err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "BCP plan not found", "CK_BCP_PLAN_NOT_FOUND")
		}
		log.Error().Err(err).Str("plan_id", id).Msg("delete bcp plan")
		return errResp(c, http.StatusInternalServerError, "failed to delete BCP plan", "CK_DELETE_BCP_PLAN_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ListBCPTests handles GET /api/v1/vaktcomply/bcp/plans/:id/tests.
func (h *Handler) ListBCPTests(c echo.Context) error {
	planID := c.Param("id")
	tests, err := h.service.BCM.ListBCPTests(c.Request().Context(), orgID(c), planID)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "BCP plan not found", "CK_BCP_PLAN_NOT_FOUND")
		}
		log.Error().Err(err).Str("plan_id", planID).Msg("list bcp tests")
		return errResp(c, http.StatusInternalServerError, "failed to list BCP tests", "CK_LIST_BCP_TESTS_FAILED")
	}
	if tests == nil {
		tests = []bcm.BCPTest{}
	}
	return c.JSON(http.StatusOK, tests)
}

// AddBCPTest handles POST /api/v1/vaktcomply/bcp/plans/:id/tests.
func (h *Handler) AddBCPTest(c echo.Context) error {
	planID := c.Param("id")
	var in bcm.CreateBCPTestInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	test, err := h.service.BCM.AddBCPTest(c.Request().Context(), orgID(c), planID, in)
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
	var body bcm.LinkBCPPlanEvidenceInput
	// Bind is best-effort; an empty body is valid (no-op path).
	_ = c.Bind(&body)

	if body.ControlID == "" {
		// No control requested — return 200 silently.
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}

	plan, err := h.service.BCM.GetBCPPlan(c.Request().Context(), orgID(c), planID)
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

// ── BIA Processes ─────────────────────────────────────────────────────────────

// ListBIAProcesses handles GET /api/v1/vaktcomply/bia/processes.
func (h *Handler) ListBIAProcesses(c echo.Context) error {
	processes, err := h.service.BCM.ListBIAProcesses(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list bia processes")
		return errResp(c, http.StatusInternalServerError, "failed to list BIA processes", "CK_LIST_BIA_FAILED")
	}
	if processes == nil {
		processes = []bcm.BIAProcess{}
	}
	return c.JSON(http.StatusOK, processes)
}

// CreateBIAProcess handles POST /api/v1/vaktcomply/bia/processes.
func (h *Handler) CreateBIAProcess(c echo.Context) error {
	var in bcm.CreateBIAProcessInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	p, err := h.service.BCM.CreateBIAProcess(c.Request().Context(), orgID(c), in)
	if err != nil {
		if err == bcm.ErrRPOExceedsRTO || err == bcm.ErrMBCOOutOfRange {
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": err.Error(), "code": "VALIDATION_ERROR"})
		}
		log.Error().Err(err).Msg("create bia process")
		return errResp(c, http.StatusInternalServerError, "failed to create BIA process", "CK_CREATE_BIA_FAILED")
	}
	return c.JSON(http.StatusCreated, p)
}

// GetBIAProcess handles GET /api/v1/vaktcomply/bia/processes/:id.
func (h *Handler) GetBIAProcess(c echo.Context) error {
	p, err := h.service.BCM.GetBIAProcess(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		return errResp(c, http.StatusNotFound, "BIA process not found", "CK_BIA_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, p)
}

// UpdateBIAProcess handles PUT /api/v1/vaktcomply/bia/processes/:id.
func (h *Handler) UpdateBIAProcess(c echo.Context) error {
	var in bcm.UpdateBIAProcessInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	p, err := h.service.BCM.UpdateBIAProcess(c.Request().Context(), orgID(c), c.Param("id"), in)
	if err != nil {
		if err == bcm.ErrRPOExceedsRTO || err == bcm.ErrMBCOOutOfRange {
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": err.Error(), "code": "VALIDATION_ERROR"})
		}
		log.Error().Err(err).Str("id", c.Param("id")).Msg("update bia process")
		return errResp(c, http.StatusInternalServerError, "failed to update BIA process", "CK_UPDATE_BIA_FAILED")
	}
	return c.JSON(http.StatusOK, p)
}

// DeleteBIAProcess handles DELETE /api/v1/vaktcomply/bia/processes/:id.
func (h *Handler) DeleteBIAProcess(c echo.Context) error {
	if err := h.service.BCM.DeleteBIAProcess(c.Request().Context(), orgID(c), c.Param("id")); err != nil {
		log.Error().Err(err).Str("id", c.Param("id")).Msg("delete bia process")
		// S121-D4 (P3): not-found → 404, not 500
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "BIA process not found", "CK_BIA_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to delete BIA process", "CK_DELETE_BIA_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// GetBIASummary handles GET /api/v1/vaktcomply/bia/summary.
func (h *Handler) GetBIASummary(c echo.Context) error {
	summary, err := h.service.BCM.GetBIASummary(c.Request().Context(), orgID(c))
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
	var plans []bcm.RecoveryPlan
	var err error
	if biaID != "" {
		plans, err = h.service.BCM.ListRecoveryPlansByBIAProcess(c.Request().Context(), orgID(c), biaID)
	} else {
		plans, err = h.service.BCM.ListRecoveryPlans(c.Request().Context(), orgID(c))
	}
	if err != nil {
		log.Error().Err(err).Msg("list recovery plans")
		return errResp(c, http.StatusInternalServerError, "failed to list recovery plans", "CK_LIST_RECOVERY_PLANS_FAILED")
	}
	if plans == nil {
		plans = []bcm.RecoveryPlan{}
	}
	return c.JSON(http.StatusOK, plans)
}

// CreateRecoveryPlan handles POST /api/v1/vaktcomply/bcm/recovery-plans.
func (h *Handler) CreateRecoveryPlan(c echo.Context) error {
	var in bcm.CreateRecoveryPlanInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	plan, err := h.service.BCM.CreateRecoveryPlan(c.Request().Context(), orgID(c), in)
	if err != nil {
		if err == bcm.ErrRTORequired || err == bcm.ErrStepsOrderInvalid {
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": err.Error(), "code": "VALIDATION_ERROR"})
		}
		log.Error().Err(err).Msg("create recovery plan")
		return errResp(c, http.StatusInternalServerError, "failed to create recovery plan", "CK_CREATE_RECOVERY_PLAN_FAILED")
	}
	return c.JSON(http.StatusCreated, plan)
}

// GetRecoveryPlan handles GET /api/v1/vaktcomply/bcm/recovery-plans/:id.
func (h *Handler) GetRecoveryPlan(c echo.Context) error {
	plan, err := h.service.BCM.GetRecoveryPlan(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		return errResp(c, http.StatusNotFound, "recovery plan not found", "CK_RECOVERY_PLAN_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, plan)
}

// UpdateRecoveryPlan handles PUT /api/v1/vaktcomply/bcm/recovery-plans/:id.
func (h *Handler) UpdateRecoveryPlan(c echo.Context) error {
	var in bcm.UpdateRecoveryPlanInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	plan, err := h.service.BCM.UpdateRecoveryPlan(c.Request().Context(), orgID(c), c.Param("id"), in)
	if err != nil {
		if err == bcm.ErrRTORequired || err == bcm.ErrStepsOrderInvalid {
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": err.Error(), "code": "VALIDATION_ERROR"})
		}
		log.Error().Err(err).Str("id", c.Param("id")).Msg("update recovery plan")
		return errResp(c, http.StatusInternalServerError, "failed to update recovery plan", "CK_UPDATE_RECOVERY_PLAN_FAILED")
	}
	return c.JSON(http.StatusOK, plan)
}

// DeleteRecoveryPlan handles DELETE /api/v1/vaktcomply/bcm/recovery-plans/:id.
func (h *Handler) DeleteRecoveryPlan(c echo.Context) error {
	if err := h.service.BCM.DeleteRecoveryPlan(c.Request().Context(), orgID(c), c.Param("id")); err != nil {
		log.Error().Err(err).Str("id", c.Param("id")).Msg("delete recovery plan")
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "recovery plan not found", "CK_RECOVERY_PLAN_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to delete recovery plan", "CK_DELETE_RECOVERY_PLAN_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ── Emergency Contacts ────────────────────────────────────────────────────────

// ListEmergencyContacts handles GET /api/v1/vaktcomply/bcm/emergency-contacts.
func (h *Handler) ListEmergencyContacts(c echo.Context) error {
	contacts, err := h.service.BCM.ListEmergencyContacts(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list emergency contacts")
		return errResp(c, http.StatusInternalServerError, "failed to list emergency contacts", "CK_LIST_EMERGENCY_CONTACTS_FAILED")
	}
	if contacts == nil {
		contacts = []bcm.EmergencyContact{}
	}
	return c.JSON(http.StatusOK, contacts)
}

// CreateEmergencyContact handles POST /api/v1/vaktcomply/bcm/emergency-contacts.
func (h *Handler) CreateEmergencyContact(c echo.Context) error {
	var in bcm.CreateEmergencyContactInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	contact, err := h.service.BCM.CreateEmergencyContact(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create emergency contact")
		return errResp(c, http.StatusInternalServerError, "failed to create emergency contact", "CK_CREATE_EMERGENCY_CONTACT_FAILED")
	}
	return c.JSON(http.StatusCreated, contact)
}

// UpdateEmergencyContact handles PUT /api/v1/vaktcomply/bcm/emergency-contacts/:id.
func (h *Handler) UpdateEmergencyContact(c echo.Context) error {
	var in bcm.UpdateEmergencyContactInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	contact, err := h.service.BCM.UpdateEmergencyContact(c.Request().Context(), orgID(c), c.Param("id"), in)
	if err != nil {
		log.Error().Err(err).Str("id", c.Param("id")).Msg("update emergency contact")
		return errResp(c, http.StatusInternalServerError, "failed to update emergency contact", "CK_UPDATE_EMERGENCY_CONTACT_FAILED")
	}
	return c.JSON(http.StatusOK, contact)
}

// DeleteEmergencyContact handles DELETE /api/v1/vaktcomply/bcm/emergency-contacts/:id.
func (h *Handler) DeleteEmergencyContact(c echo.Context) error {
	if err := h.service.BCM.DeleteEmergencyContact(c.Request().Context(), orgID(c), c.Param("id")); err != nil {
		log.Error().Err(err).Str("id", c.Param("id")).Msg("delete emergency contact")
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "emergency contact not found", "CK_EMERGENCY_CONTACT_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to delete emergency contact", "CK_DELETE_EMERGENCY_CONTACT_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// GetBCMReadinessScore handles GET /api/v1/vaktcomply/bcm/readiness-score.
func (h *Handler) GetBCMReadinessScore(c echo.Context) error {
	score, err := h.service.BCM.GetBCMReadinessScore(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get bcm readiness score")
		return errResp(c, http.StatusInternalServerError, "failed to get BCM readiness score", "CK_BCM_SCORE_FAILED")
	}
	return c.JSON(http.StatusOK, score)
}

func (h *Handler) ListAISystems(c echo.Context) error {
	filters := AISystemFilters{
		RiskClass: c.QueryParam("risk_class"),
		Status:    c.QueryParam("status"),
	}
	systems, err := h.service.ListAISystems(c.Request().Context(), orgID(c), filters)
	if err != nil {
		log.Error().Err(err).Msg("list ai systems")
		return errResp(c, http.StatusInternalServerError, "failed to list AI systems", "CK_LIST_AI_SYSTEMS_FAILED")
	}
	return c.JSON(http.StatusOK, systems)
}

// DeleteAISystem handles DELETE /api/v1/vaktcomply/ai-systems/:id.
func (h *Handler) DeleteAISystem(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid AI system ID", "CK_INVALID_ID")
	}
	if err := h.service.DeleteAISystem(c.Request().Context(), orgID(c), id); err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "AI system not found", "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to delete AI system", "CK_INTERNAL")
	}
	return c.NoContent(http.StatusNoContent)
}

// CreateAISystem handles POST /api/v1/vaktcomply/ai-systems.
func (h *Handler) CreateAISystem(c echo.Context) error {
	var in CreateAISystemInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	a, err := h.service.CreateAISystem(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create ai system")
		return errResp(c, http.StatusInternalServerError, "failed to create AI system", "CK_CREATE_AI_SYSTEM_FAILED")
	}
	return c.JSON(http.StatusCreated, a)
}

// GetAISystem handles GET /api/v1/vaktcomply/ai-systems/:id.
func (h *Handler) GetAISystem(c echo.Context) error {
	a, err := h.service.GetAISystem(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		return errResp(c, http.StatusNotFound, "AI system not found", "CK_AI_SYSTEM_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, a)
}

// UpdateAISystem handles PATCH /api/v1/vaktcomply/ai-systems/:id.
func (h *Handler) UpdateAISystem(c echo.Context) error {
	id := c.Param("id")
	var in UpdateAISystemInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	a, err := h.service.UpdateAISystem(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		log.Error().Err(err).Msg("update ai system")
		return errResp(c, http.StatusInternalServerError, "failed to update AI system", "CK_UPDATE_AI_SYSTEM_FAILED")
	}
	return c.JSON(http.StatusOK, a)
}

// ClassifyAISystem handles POST /api/v1/vaktcomply/ai-systems/:id/classify.
func (h *Handler) ClassifyAISystem(c echo.Context) error {
	id := c.Param("id")
	var in ClassifyAISystemInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.service.ClassifyAISystem(c.Request().Context(), orgID(c), id, in); err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "AI system not found", "CK_NOT_FOUND")
		}
		log.Error().Err(err).Msg("classify ai system")
		return errResp(c, http.StatusInternalServerError, "failed to classify AI system", "CK_CLASSIFY_AI_SYSTEM_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ListAIClassifications handles GET /api/v1/vaktcomply/ai-systems/:id/classifications.
func (h *Handler) ListAIClassifications(c echo.Context) error {
	id := c.Param("id")
	list, err := h.service.ListAIClassifications(c.Request().Context(), orgID(c), id)
	if err != nil {
		log.Error().Err(err).Msg("list ai classifications")
		return errResp(c, http.StatusInternalServerError, "failed to list classifications", "CK_LIST_CLASSIFICATIONS_FAILED")
	}
	if list == nil {
		list = []AIClassification{}
	}
	return c.JSON(http.StatusOK, list)
}

// SaveAIDocumentation handles POST /api/v1/vaktcomply/ai-systems/:id/documentation.
func (h *Handler) SaveAIDocumentation(c echo.Context) error {
	id := c.Param("id")
	var in UpsertAIDocumentationInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	doc, err := h.service.SaveAIDocumentation(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		log.Error().Err(err).Msg("save ai documentation")
		return errResp(c, http.StatusInternalServerError, "failed to save documentation", "CK_SAVE_AI_DOC_FAILED")
	}
	return c.JSON(http.StatusOK, doc)
}

// GetLatestAIDocumentation handles GET /api/v1/vaktcomply/ai-systems/:id/documentation.
func (h *Handler) GetLatestAIDocumentation(c echo.Context) error {
	id := c.Param("id")
	doc, err := h.service.GetLatestAIDocumentation(c.Request().Context(), orgID(c), id)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "documentation not found", "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to get documentation", "CK_GET_AI_DOC_FAILED")
	}
	return c.JSON(http.StatusOK, doc)
}

// ListAIDocumentationVersions handles GET /api/v1/vaktcomply/ai-systems/:id/documentation/versions.
func (h *Handler) ListAIDocumentationVersions(c echo.Context) error {
	id := c.Param("id")
	versions, err := h.service.ListAIDocumentationVersions(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to list documentation versions", "CK_LIST_AI_DOC_FAILED")
	}
	if versions == nil {
		versions = []AIDocumentation{}
	}
	return c.JSON(http.StatusOK, versions)
}

// ExportAIDocumentationPDF handles GET /api/v1/vaktcomply/ai-systems/:id/documentation/export-pdf.
func (h *Handler) ExportAIDocumentationPDF(c echo.Context) error {
	id := c.Param("id")
	pdfBytes, filename, err := h.service.ExportAIDocumentationPDF(c.Request().Context(), orgID(c), id)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "AI system not found", "CK_NOT_FOUND")
		}
		log.Error().Err(err).Msg("export ai documentation pdf")
		return errResp(c, http.StatusInternalServerError, "failed to export PDF", "CK_EXPORT_AI_DOC_PDF_FAILED")
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Response().Header().Set("Content-Type", "application/pdf")
	_, err = c.Response().Write(pdfBytes)
	return err
}

// GetOrgSector handles GET /api/v1/vaktcomply/org-sector.
func (h *Handler) GetOrgSector(c echo.Context) error {
	settings, err := h.service.GetOrgSector(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get org sector")
		return errResp(c, http.StatusInternalServerError, "failed to get org sector", "CK_GET_SECTOR_FAILED")
	}
	return c.JSON(http.StatusOK, settings)
}

// UpdateOrgSector handles PATCH /api/v1/vaktcomply/org-sector.
func (h *Handler) UpdateOrgSector(c echo.Context) error {
	var in UpdateOrgSectorInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	settings, err := h.service.UpdateOrgSector(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("update org sector")
		return errResp(c, http.StatusInternalServerError, "failed to update org sector", "CK_UPDATE_SECTOR_FAILED")
	}
	return c.JSON(http.StatusOK, settings)
}

// ListAuthorities handles GET /api/v1/vaktcomply/authorities.
// S39-4: reads from db/seeds/authorities.yaml (falls back to in-memory directory).
func (h *Handler) ListAuthorities(c echo.Context) error {
	all := LoadAuthoritiesFromYAML()
	return c.JSON(http.StatusOK, all)
}

// GetOrgAuthorities handles GET /api/v1/vaktcomply/org-authorities — sector-specific.
func (h *Handler) GetOrgAuthorities(c echo.Context) error {
	authorities, err := h.service.GetAuthoritiesForOrg(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get org authorities")
		return errResp(c, http.StatusInternalServerError, "failed to get authorities", "CK_GET_AUTHORITIES_FAILED")
	}
	return c.JSON(http.StatusOK, authorities)
}

// GetEUAIActDashboard handles GET /api/v1/vaktcomply/eu-ai-act/dashboard.
func (h *Handler) GetEUAIActDashboard(c echo.Context) error {
	dashboard, err := h.service.GetEUAIActDashboard(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get eu ai act dashboard")
		return errResp(c, http.StatusInternalServerError, "failed to get EU AI Act dashboard", "CK_EU_AI_ACT_DASHBOARD_FAILED")
	}
	return c.JSON(http.StatusOK, dashboard)
}

// GetEUAIActReportPDF handles GET /api/v1/vaktcomply/eu-ai-act/report-pdf.
func (h *Handler) GetEUAIActReportPDF(c echo.Context) error {
	pdfBytes, err := h.service.ExportEUAIActReportPDF(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get eu ai act report pdf")
		return errResp(c, http.StatusInternalServerError, "failed to generate EU AI Act report PDF", "CK_EU_AI_ACT_PDF_FAILED")
	}
	c.Response().Header().Set("Content-Disposition", `attachment; filename="eu-ai-act-report.pdf"`)
	c.Response().Header().Set("Content-Type", "application/pdf")
	_, err = c.Response().Write(pdfBytes)
	return err
}

// --- Resilience Tests (DORA Art. 24-27) ---

// ListResilienceTests handles GET /api/v1/vaktcomply/resilience-tests.
func (h *Handler) ListResilienceTests(c echo.Context) error {
	tests, tlptOverdue, err := h.service.ListResilienceTests(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list resilience tests")
		return errResp(c, http.StatusInternalServerError, "failed to list resilience tests", "CK_LIST_RESILIENCE_TESTS_FAILED")
	}
	return c.JSON(http.StatusOK, map[string]any{
		"tests":                tests,
		"tlpt_overdue_warning": tlptOverdue,
	})
}

// CreateResilienceTest handles POST /api/v1/vaktcomply/resilience-tests.
func (h *Handler) CreateResilienceTest(c echo.Context) error {
	var in CreateResilienceTestInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	t, err := h.service.CreateResilienceTest(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create resilience test")
		return errResp(c, http.StatusInternalServerError, "failed to create resilience test", "CK_CREATE_RESILIENCE_TEST_FAILED")
	}
	return c.JSON(http.StatusCreated, t)
}

// GetResilienceTest handles GET /api/v1/vaktcomply/resilience-tests/:id.
func (h *Handler) GetResilienceTest(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid resilience test id", "CK_BAD_REQUEST")
	}
	t, err := h.service.GetResilienceTest(c.Request().Context(), orgID(c), id)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "resilience test not found", "CK_RESILIENCE_TEST_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("get resilience test")
		return errResp(c, http.StatusInternalServerError, "failed to get resilience test", "CK_GET_RESILIENCE_TEST_FAILED")
	}
	return c.JSON(http.StatusOK, t)
}

// UpdateResilienceTest handles PATCH /api/v1/vaktcomply/resilience-tests/:id.
func (h *Handler) UpdateResilienceTest(c echo.Context) error {
	id := c.Param("id")
	var in UpdateResilienceTestInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	t, err := h.service.UpdateResilienceTest(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "resilience test not found", "CK_RESILIENCE_TEST_NOT_FOUND")
		}
		log.Error().Err(err).Msg("update resilience test")
		return errResp(c, http.StatusInternalServerError, "failed to update resilience test", "CK_UPDATE_RESILIENCE_TEST_FAILED")
	}
	return c.JSON(http.StatusOK, t)
}

// DeleteResilienceTest handles DELETE /api/v1/vaktcomply/resilience-tests/:id.
func (h *Handler) DeleteResilienceTest(c echo.Context) error {
	id := c.Param("id")
	if err := h.service.DeleteResilienceTest(c.Request().Context(), orgID(c), id); err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "resilience test not found", "CK_RESILIENCE_TEST_NOT_FOUND")
		}
		log.Error().Err(err).Msg("delete resilience test")
		return errResp(c, http.StatusInternalServerError, "failed to delete resilience test", "CK_DELETE_RESILIENCE_TEST_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// LinkResilienceTestAsEvidence handles POST /api/v1/vaktcomply/resilience-tests/:id/link-evidence (S40-1).
// Creates a compliance evidence record on the given DORA control from the test result.
func (h *Handler) LinkResilienceTestAsEvidence(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid resilience test id", "CK_BAD_REQUEST")
	}
	var body struct {
		ControlID string `json:"control_id" validate:"required"`
	}
	if err := c.Bind(&body); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(body); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "control_id is required", "VALIDATION_ERROR")
	}
	ev, err := h.service.LinkResilienceTestAsEvidence(c.Request().Context(), orgID(c), id, body.ControlID, userID(c))
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "resilience test not found", "CK_NOT_FOUND")
		}
		log.Error().Err(err).Msg("link resilience test as evidence")
		return errResp(c, http.StatusInternalServerError, "failed to link as evidence", "CK_LINK_EVIDENCE_FAILED")
	}
	return c.JSON(http.StatusCreated, ev)
}

// UploadResilienceTestAttachment handles POST /api/v1/vaktcomply/resilience-tests/:id/attachment.
// Accepts multipart/form-data with a "file" field. Max size: 20 MB.
func (h *Handler) UploadResilienceTestAttachment(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid resilience test id", "CK_BAD_REQUEST")
	}

	const maxSize = 20 << 20 // 20 MB
	if err := c.Request().ParseMultipartForm(maxSize); err != nil {
		return errResp(c, http.StatusBadRequest, "failed to parse multipart form", "CK_BAD_REQUEST")
	}

	fh, err := c.FormFile("file")
	if err != nil {
		return errResp(c, http.StatusBadRequest, "file is required", "CK_BAD_REQUEST")
	}
	if fh.Size > maxSize {
		return errResp(c, http.StatusRequestEntityTooLarge, "file too large (max 20 MB)", "CK_FILE_TOO_LARGE")
	}

	src, err := fh.Open()
	if err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to open uploaded file", "CK_UPLOAD_FAILED")
	}
	defer src.Close()

	fileBytes, err := io.ReadAll(src)
	if err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to read uploaded file", "CK_UPLOAD_FAILED")
	}

	uploadDir := h.uploadDir
	if uploadDir == "" {
		uploadDir = "./data/uploads"
	}

	t, err := h.service.AttachResilienceTestFile(c.Request().Context(), orgID(c), id, uploadDir, fileBytes, filepath.Base(fh.Filename))
	if err != nil {
		log.Error().Err(err).Str("resilience_test_id", id).Msg("upload resilience test attachment")
		return errResp(c, http.StatusInternalServerError, "failed to save attachment", "CK_UPLOAD_FAILED")
	}
	return c.JSON(http.StatusOK, t)
}

// --- DORA Dashboard (Story 27.5) ---

// GetDORADashboard handles GET /api/v1/vaktcomply/dora/dashboard.
func (h *Handler) GetDORADashboard(c echo.Context) error {
	dashboard, err := h.service.GetDORADashboard(c.Request().Context(), orgID(c))
	if err != nil {
		if errors.Is(err, ErrDORANotEnabled) {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "DORA framework not enabled",
				"code":  "CK_DORA_NOT_ENABLED",
			})
		}
		log.Error().Err(err).Msg("get dora dashboard")
		return errResp(c, http.StatusInternalServerError, "failed to get DORA dashboard", "CK_DORA_DASHBOARD_FAILED")
	}
	return c.JSON(http.StatusOK, dashboard)
}

// GetDORAPDF handles GET /api/v1/vaktcomply/dora/report-pdf.
func (h *Handler) GetDORAPDF(c echo.Context) error {
	pdfBytes, err := h.service.ExportDORAPDF(c.Request().Context(), orgID(c))
	if err != nil {
		if errors.Is(err, ErrDORANotEnabled) {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "DORA framework not enabled",
				"code":  "CK_DORA_NOT_ENABLED",
			})
		}
		log.Error().Err(err).Msg("generate dora pdf")
		return errResp(c, http.StatusInternalServerError, "failed to generate PDF", "CK_DORA_PDF_FAILED")
	}
	c.Response().Header().Set("Content-Disposition", `attachment; filename="dora-bericht.pdf"`)
	return c.Blob(http.StatusOK, "application/pdf", pdfBytes)
}

// --- DORA IKT-Drittanbieter-Register (Art. 28-44) ---

// ListDORAThirdParties handles GET /api/v1/vaktcomply/dora/third-parties.
func (h *Handler) ListDORAThirdParties(c echo.Context) error {
	criticality := c.QueryParam("criticality")
	list, err := h.service.Risk.ListDORAThirdParties(c.Request().Context(), orgID(c), criticality)
	if err != nil {
		log.Error().Err(err).Msg("list dora third parties")
		return errResp(c, http.StatusInternalServerError, "failed to list third parties", "CK_LIST_DORA_TP_FAILED")
	}
	return c.JSON(http.StatusOK, list)
}

// GetDORAThirdParty handles GET /api/v1/vaktcomply/dora/third-parties/:id.
func (h *Handler) GetDORAThirdParty(c echo.Context) error {
	id := c.Param("id")
	tp, err := h.service.Risk.GetDORAThirdParty(c.Request().Context(), orgID(c), id)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "third party not found", "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to get third party", "CK_GET_DORA_TP_FAILED")
	}
	return c.JSON(http.StatusOK, tp)
}

// CreateDORAThirdParty handles POST /api/v1/vaktcomply/dora/third-parties.
func (h *Handler) CreateDORAThirdParty(c echo.Context) error {
	var in CreateDORAThirdPartyInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	tp, err := h.service.Risk.CreateDORAThirdParty(c.Request().Context(), orgID(c), userID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create dora third party")
		return errResp(c, http.StatusInternalServerError, "failed to create third party", "CK_CREATE_DORA_TP_FAILED")
	}
	return c.JSON(http.StatusCreated, tp)
}

// UpdateDORAThirdParty handles PATCH /api/v1/vaktcomply/dora/third-parties/:id.
func (h *Handler) UpdateDORAThirdParty(c echo.Context) error {
	id := c.Param("id")
	var in UpdateDORAThirdPartyInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	tp, err := h.service.Risk.UpdateDORAThirdParty(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "third party not found", "CK_NOT_FOUND")
		}
		log.Error().Err(err).Msg("update dora third party")
		return errResp(c, http.StatusInternalServerError, "failed to update third party", "CK_UPDATE_DORA_TP_FAILED")
	}
	return c.JSON(http.StatusOK, tp)
}

// DeleteDORAThirdParty handles DELETE /api/v1/vaktcomply/dora/third-parties/:id.
func (h *Handler) DeleteDORAThirdParty(c echo.Context) error {
	id := c.Param("id")
	if err := h.service.Risk.DeleteDORAThirdParty(c.Request().Context(), orgID(c), id); err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "third party not found", "CK_NOT_FOUND")
		}
		log.Error().Err(err).Msg("delete dora third party")
		return errResp(c, http.StatusInternalServerError, "failed to delete third party", "CK_DELETE_DORA_TP_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// LinkDORAThirdPartyControl handles POST /api/v1/vaktcomply/dora/third-parties/:id/controls.
func (h *Handler) LinkDORAThirdPartyControl(c echo.Context) error {
	id := c.Param("id")
	var body struct {
		ControlID string `json:"control_id" validate:"required"`
	}
	if err := c.Bind(&body); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(body); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "control_id is required", "VALIDATION_ERROR")
	}
	if err := h.service.Risk.LinkDORAThirdPartyControl(c.Request().Context(), orgID(c), id, body.ControlID); err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "third party not found", "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to link control", "CK_LINK_DORA_TP_CTRL_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// UnlinkDORAThirdPartyControl handles DELETE /api/v1/vaktcomply/dora/third-parties/:id/controls/:controlId.
func (h *Handler) UnlinkDORAThirdPartyControl(c echo.Context) error {
	id := c.Param("id")
	controlID := c.Param("controlId")
	if err := h.service.Risk.UnlinkDORAThirdPartyControl(c.Request().Context(), orgID(c), id, controlID); err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "third party not found", "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to unlink control", "CK_UNLINK_DORA_TP_CTRL_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// GetExecutiveSummaryPDF handles GET /api/v1/vaktcomply/reports/executive-summary.
func (h *Handler) GetExecutiveSummaryPDF(c echo.Context) error {
	pdfBytes, filename, err := h.service.ExportExecutiveSummaryPDF(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("generate executive summary pdf")
		return errResp(c, http.StatusInternalServerError, "failed to generate executive summary PDF", "CK_EXECUTIVE_SUMMARY_FAILED")
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	return c.Blob(http.StatusOK, "application/pdf", pdfBytes)
}
