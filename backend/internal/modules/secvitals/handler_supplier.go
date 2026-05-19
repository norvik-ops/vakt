package secvitals

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ListSuppliers handles GET /api/v1/secvitals/suppliers.
// Supports optional query params: criticality, assessment_status.
func (h *Handler) ListSuppliers(c echo.Context) error {
	filter := &SupplierFilter{
		Criticality:      c.QueryParam("criticality"),
		AssessmentStatus: c.QueryParam("assessment_status"),
	}
	if filter.Criticality == "" && filter.AssessmentStatus == "" {
		filter = nil
	}
	suppliers, err := h.service.ListSuppliers(c.Request().Context(), orgID(c), filter)
	if err != nil {
		log.Error().Err(err).Msg("list suppliers")
		return errResp(c, http.StatusInternalServerError, "failed to list suppliers", "CK_LIST_SUPPLIERS_FAILED")
	}
	return c.JSON(http.StatusOK, suppliers)
}

// CreateSupplier handles POST /api/v1/secvitals/suppliers.
func (h *Handler) CreateSupplier(c echo.Context) error {
	var in CreateSupplierInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	s, err := h.service.CreateSupplier(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create supplier")
		return errResp(c, http.StatusInternalServerError, "failed to create supplier", "CK_CREATE_SUPPLIER_FAILED")
	}
	return c.JSON(http.StatusCreated, s)
}

// GetSupplier handles GET /api/v1/secvitals/suppliers/:id.
func (h *Handler) GetSupplier(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid supplier id", "CK_BAD_REQUEST")
	}
	s, err := h.service.GetSupplier(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusNotFound, "supplier not found", "CK_SUPPLIER_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, s)
}

// UpdateSupplier handles PATCH /api/v1/secvitals/suppliers/:id.
func (h *Handler) UpdateSupplier(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid supplier id", "CK_BAD_REQUEST")
	}
	var in UpdateSupplierInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	s, err := h.service.UpdateSupplier(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		log.Error().Err(err).Msg("update supplier")
		return errResp(c, http.StatusInternalServerError, "failed to update supplier", "CK_UPDATE_SUPPLIER_FAILED")
	}
	return c.JSON(http.StatusOK, s)
}

// DeleteSupplier handles DELETE /api/v1/secvitals/suppliers/:id.
func (h *Handler) DeleteSupplier(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid supplier id", "CK_BAD_REQUEST")
	}
	if err := h.service.DeleteSupplier(c.Request().Context(), orgID(c), id); err != nil {
		return errResp(c, http.StatusNotFound, "supplier not found", "CK_SUPPLIER_NOT_FOUND")
	}
	return c.NoContent(http.StatusNoContent)
}

// GetSupplierIncidents handles GET /api/v1/secvitals/suppliers/:id/incidents.
func (h *Handler) GetSupplierIncidents(c echo.Context) error {
	supplierID := c.Param("id")
	if _, err := uuid.Parse(supplierID); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid supplier id"})
	}
	incidents, err := h.service.ListIncidentsBySupplier(c.Request().Context(), orgID(c), supplierID)
	if err != nil {
		log.Error().Err(err).Str("supplier_id", supplierID).Msg("get supplier incidents")
		return errResp(c, http.StatusInternalServerError, "failed to list supplier incidents", "CK_LIST_SUPPLIER_INCIDENTS_FAILED")
	}
	return c.JSON(http.StatusOK, incidents)
}

// ExportSuppliers handles GET /api/v1/secvitals/suppliers/export.
// Returns a CSV file with all suppliers for the organisation.
func (h *Handler) ExportSuppliers(c echo.Context) error {
	suppliers, err := h.service.ListSuppliers(c.Request().Context(), orgID(c), nil)
	if err != nil {
		log.Error().Err(err).Msg("export suppliers: list suppliers")
		return errResp(c, http.StatusInternalServerError, "failed to list suppliers", "CK_LIST_SUPPLIERS_FAILED")
	}
	data, err := GenerateSupplierCSV(suppliers)
	if err != nil {
		log.Error().Err(err).Msg("export suppliers: generate csv")
		return errResp(c, http.StatusInternalServerError, "failed to generate CSV", "CK_EXPORT_SUPPLIERS_FAILED")
	}
	c.Response().Header().Set("Content-Disposition", `attachment; filename=suppliers-export.csv`)
	return c.Blob(http.StatusOK, "text/csv", data)
}

// ImportSuppliersCSV handles POST /api/v1/secvitals/suppliers/import-csv.
// Accepts a multipart form with field "file" containing a CSV.
func (h *Handler) ImportSuppliersCSV(c echo.Context) error {
	if err := c.Request().ParseMultipartForm(10 << 20); err != nil { // 10 MB
		return errResp(c, http.StatusBadRequest, "failed to parse multipart form", "CK_BAD_REQUEST")
	}
	file, _, err := c.Request().FormFile("file")
	if err != nil {
		return errResp(c, http.StatusBadRequest, "missing file field in multipart form", "CK_BAD_REQUEST")
	}
	defer file.Close()

	result, err := h.service.ParseAndImportSupplierCSV(c.Request().Context(), orgID(c), file)
	if err != nil {
		log.Error().Err(err).Msg("import suppliers csv")
		return errResp(c, http.StatusInternalServerError, "failed to import CSV", "CK_IMPORT_SUPPLIERS_FAILED")
	}
	return c.JSON(http.StatusOK, result)
}

// LinkSupplierRisk handles POST /api/v1/secvitals/suppliers/:id/risks.
func (h *Handler) LinkSupplierRisk(c echo.Context) error {
	supplierID := c.Param("id")
	if _, err := uuid.Parse(supplierID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid supplier id", "CK_BAD_REQUEST")
	}
	var body struct {
		RiskID string `json:"risk_id"`
	}
	if err := c.Bind(&body); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if _, err := uuid.Parse(body.RiskID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid risk_id", "CK_BAD_REQUEST")
	}
	if err := h.service.LinkSupplierRisk(c.Request().Context(), orgID(c), supplierID, body.RiskID); err != nil {
		log.Error().Err(err).Str("supplier_id", supplierID).Str("risk_id", body.RiskID).Msg("link supplier risk")
		if strings.Contains(err.Error(), "not found") {
			return errResp(c, http.StatusNotFound, err.Error(), "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to link risk", "CK_LINK_SUPPLIER_RISK_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// UnlinkSupplierRisk handles DELETE /api/v1/secvitals/suppliers/:id/risks/:riskId.
func (h *Handler) UnlinkSupplierRisk(c echo.Context) error {
	supplierID := c.Param("id")
	riskID := c.Param("riskId")
	if _, err := uuid.Parse(supplierID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid supplier id", "CK_BAD_REQUEST")
	}
	if _, err := uuid.Parse(riskID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid risk id", "CK_BAD_REQUEST")
	}
	if err := h.service.UnlinkSupplierRisk(c.Request().Context(), orgID(c), supplierID, riskID); err != nil {
		log.Error().Err(err).Str("supplier_id", supplierID).Str("risk_id", riskID).Msg("unlink supplier risk")
		if strings.Contains(err.Error(), "not found") {
			return errResp(c, http.StatusNotFound, err.Error(), "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to unlink risk", "CK_UNLINK_SUPPLIER_RISK_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ListSupplierRisks handles GET /api/v1/secvitals/suppliers/:id/risks.
func (h *Handler) ListSupplierRisks(c echo.Context) error {
	supplierID := c.Param("id")
	if _, err := uuid.Parse(supplierID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid supplier id", "CK_BAD_REQUEST")
	}
	risks, err := h.service.ListSupplierRisks(c.Request().Context(), orgID(c), supplierID)
	if err != nil {
		log.Error().Err(err).Str("supplier_id", supplierID).Msg("list supplier risks")
		if strings.Contains(err.Error(), "not found") {
			return errResp(c, http.StatusNotFound, err.Error(), "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to list supplier risks", "CK_LIST_SUPPLIER_RISKS_FAILED")
	}
	return c.JSON(http.StatusOK, risks)
}

// ListTemplates handles GET /api/v1/secvitals/questionnaires/templates.
func (h *Handler) ListTemplates(c echo.Context) error {
	templates, err := h.service.ListTemplates(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list templates")
		return errResp(c, http.StatusInternalServerError, "failed to list templates", "CK_LIST_TEMPLATES_FAILED")
	}
	return c.JSON(http.StatusOK, templates)
}

// ListQuestionnaires handles GET /api/v1/secvitals/questionnaires.
func (h *Handler) ListQuestionnaires(c echo.Context) error {
	var isTemplate *bool
	if raw := c.QueryParam("is_template"); raw != "" {
		v := raw == "true"
		isTemplate = &v
	}
	questionnaires, err := h.service.ListQuestionnaires(c.Request().Context(), orgID(c), isTemplate)
	if err != nil {
		log.Error().Err(err).Msg("list questionnaires")
		return errResp(c, http.StatusInternalServerError, "failed to list questionnaires", "CK_LIST_QUESTIONNAIRES_FAILED")
	}
	return c.JSON(http.StatusOK, questionnaires)
}

// CreateQuestionnaire handles POST /api/v1/secvitals/questionnaires.
func (h *Handler) CreateQuestionnaire(c echo.Context) error {
	var in CreateQuestionnaireInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	if in.CloneFromID != "" {
		if _, err := uuid.Parse(in.CloneFromID); err != nil {
			return errResp(c, http.StatusBadRequest, "invalid clone_from_id", "CK_BAD_REQUEST")
		}
	}
	q, err := h.service.CreateQuestionnaire(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create questionnaire")
		return errResp(c, http.StatusInternalServerError, "failed to create questionnaire", "CK_CREATE_QUESTIONNAIRE_FAILED")
	}
	return c.JSON(http.StatusCreated, q)
}

// GetQuestionnaire handles GET /api/v1/secvitals/questionnaires/:id.
func (h *Handler) GetQuestionnaire(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid questionnaire id", "CK_BAD_REQUEST")
	}
	q, err := h.service.GetQuestionnaire(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusNotFound, "questionnaire not found", "CK_QUESTIONNAIRE_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, q)
}

// UpdateQuestionnaire handles PATCH /api/v1/secvitals/questionnaires/:id.
func (h *Handler) UpdateQuestionnaire(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid questionnaire id", "CK_BAD_REQUEST")
	}
	var in UpdateQuestionnaireInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	q, err := h.service.UpdateQuestionnaire(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errResp(c, http.StatusNotFound, "questionnaire not found", "CK_QUESTIONNAIRE_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("update questionnaire")
		return errResp(c, http.StatusInternalServerError, "failed to update questionnaire", "CK_UPDATE_QUESTIONNAIRE_FAILED")
	}
	return c.JSON(http.StatusOK, q)
}

// DeleteQuestionnaire handles DELETE /api/v1/secvitals/questionnaires/:id.
func (h *Handler) DeleteQuestionnaire(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid questionnaire id", "CK_BAD_REQUEST")
	}
	if err := h.service.DeleteQuestionnaire(c.Request().Context(), orgID(c), id); err != nil {
		log.Error().Err(err).Str("id", id).Msg("delete questionnaire")
		return errResp(c, http.StatusInternalServerError, "failed to delete questionnaire", "CK_DELETE_QUESTIONNAIRE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// AddQuestion handles POST /api/v1/secvitals/questionnaires/:id/questions.
func (h *Handler) AddQuestion(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid questionnaire id", "CK_BAD_REQUEST")
	}
	var in CreateQuestionInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	if in.ControlID != "" {
		if _, err := uuid.Parse(in.ControlID); err != nil {
			return errResp(c, http.StatusBadRequest, "invalid control_id", "CK_BAD_REQUEST")
		}
	}
	q, err := h.service.AddQuestion(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if strings.Contains(err.Error(), "multiple_choice question requires non-empty options") {
			return errResp(c, http.StatusUnprocessableEntity, err.Error(), "CK_VALIDATION_ERROR")
		}
		log.Error().Err(err).Str("questionnaire_id", id).Msg("add question")
		return errResp(c, http.StatusInternalServerError, "failed to add question", "CK_ADD_QUESTION_FAILED")
	}
	return c.JSON(http.StatusCreated, q)
}

// UpdateQuestion handles PATCH /api/v1/secvitals/questionnaires/:id/questions/:qid.
func (h *Handler) UpdateQuestion(c echo.Context) error {
	id := c.Param("id")
	qid := c.Param("qid")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid questionnaire id", "CK_BAD_REQUEST")
	}
	if _, err := uuid.Parse(qid); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid question id", "CK_BAD_REQUEST")
	}
	var in CreateQuestionInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	if in.ControlID != "" {
		if _, err := uuid.Parse(in.ControlID); err != nil {
			return errResp(c, http.StatusBadRequest, "invalid control_id", "CK_BAD_REQUEST")
		}
	}
	q, err := h.service.UpdateQuestion(c.Request().Context(), orgID(c), id, qid, in)
	if err != nil {
		if strings.Contains(err.Error(), "multiple_choice question requires non-empty options") {
			return errResp(c, http.StatusUnprocessableEntity, err.Error(), "CK_VALIDATION_ERROR")
		}
		log.Error().Err(err).Str("questionnaire_id", id).Str("question_id", qid).Msg("update question")
		return errResp(c, http.StatusInternalServerError, "failed to update question", "CK_UPDATE_QUESTION_FAILED")
	}
	return c.JSON(http.StatusOK, q)
}

// DeleteQuestion handles DELETE /api/v1/secvitals/questionnaires/:id/questions/:qid.
func (h *Handler) DeleteQuestion(c echo.Context) error {
	id := c.Param("id")
	qid := c.Param("qid")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid questionnaire id", "CK_BAD_REQUEST")
	}
	if _, err := uuid.Parse(qid); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid question id", "CK_BAD_REQUEST")
	}
	if err := h.service.DeleteQuestion(c.Request().Context(), orgID(c), id, qid); err != nil {
		log.Error().Err(err).Str("questionnaire_id", id).Str("question_id", qid).Msg("delete question")
		return errResp(c, http.StatusInternalServerError, "failed to delete question", "CK_DELETE_QUESTION_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ReorderQuestions handles POST /api/v1/secvitals/questionnaires/:id/questions/reorder.
func (h *Handler) ReorderQuestions(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid questionnaire id", "CK_BAD_REQUEST")
	}
	var in ReorderQuestionsInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	for _, qid := range in.Order {
		if _, err := uuid.Parse(qid); err != nil {
			return errResp(c, http.StatusBadRequest, fmt.Sprintf("invalid question id in order: %s", qid), "CK_BAD_REQUEST")
		}
	}
	if err := h.service.ReorderQuestions(c.Request().Context(), orgID(c), id, in.Order); err != nil {
		log.Error().Err(err).Str("questionnaire_id", id).Msg("reorder questions")
		return errResp(c, http.StatusInternalServerError, "failed to reorder questions", "CK_REORDER_QUESTIONS_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// CreateSupplierAssessment handles POST /api/v1/secvitals/suppliers/:id/assessments.
func (h *Handler) CreateSupplierAssessment(c echo.Context) error {
	supplierID := c.Param("id")
	if _, err := uuid.Parse(supplierID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid supplier id", "CK_BAD_REQUEST")
	}
	var in CreateAssessmentInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}

	req := c.Request()
	scheme := "https"
	if req.TLS == nil {
		scheme = "http"
	}
	baseURL := scheme + "://" + req.Host

	assessment, _, err := h.service.CreateAssessment(c.Request().Context(), orgID(c), supplierID, in, baseURL)
	if err != nil {
		log.Error().Err(err).Str("supplier_id", supplierID).Msg("create supplier assessment")
		return errResp(c, http.StatusInternalServerError, "failed to create assessment", "CK_CREATE_ASSESSMENT_FAILED")
	}
	return c.JSON(http.StatusCreated, map[string]string{
		"id":        assessment.ID,
		"share_url": assessment.ShareURL,
	})
}

// ListSupplierAssessments handles GET /api/v1/secvitals/suppliers/:id/assessments.
func (h *Handler) ListSupplierAssessments(c echo.Context) error {
	supplierID := c.Param("id")
	if _, err := uuid.Parse(supplierID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid supplier id", "CK_BAD_REQUEST")
	}
	assessments, err := h.service.ListAssessmentsForSupplier(c.Request().Context(), orgID(c), supplierID)
	if err != nil {
		log.Error().Err(err).Str("supplier_id", supplierID).Msg("list supplier assessments")
		return errResp(c, http.StatusInternalServerError, "failed to list assessments", "CK_LIST_ASSESSMENTS_FAILED")
	}
	if assessments == nil {
		assessments = []Assessment{}
	}
	return c.JSON(http.StatusOK, assessments)
}

// GetAssessment handles GET /api/v1/secvitals/assessments/:id.
func (h *Handler) GetAssessment(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid assessment id", "CK_BAD_REQUEST")
	}
	a, err := h.service.GetAssessment(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusNotFound, "assessment not found", "CK_ASSESSMENT_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, a)
}

// PortalGetAssessment handles GET /supplier/:token (public, no auth).
func (h *Handler) PortalGetAssessment(c echo.Context) error {
	token := c.Param("token")
	a, err := h.service.GetAssessmentForPortal(c.Request().Context(), token)
	if err != nil {
		if errors.Is(err, ErrAssessmentExpiredOrSubmitted) {
			return c.JSON(http.StatusGone, map[string]string{"error": "assessment_expired_or_submitted"})
		}
		log.Error().Err(err).Msg("portal get assessment")
		return errResp(c, http.StatusInternalServerError, "failed to load assessment", "CK_ASSESSMENT_LOAD_FAILED")
	}
	return c.JSON(http.StatusOK, a)
}

// PortalSaveAnswers handles POST /supplier/:token/save (public, no auth).
func (h *Handler) PortalSaveAnswers(c echo.Context) error {
	token := c.Param("token")
	var in SaveAnswersInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	if err := h.service.SaveAnswers(c.Request().Context(), token, in); err != nil {
		if errors.Is(err, ErrAssessmentExpiredOrSubmitted) {
			return c.JSON(http.StatusGone, map[string]string{"error": "assessment_expired_or_submitted"})
		}
		log.Error().Err(err).Msg("portal save answers")
		return errResp(c, http.StatusInternalServerError, "failed to save answers", "CK_SAVE_ANSWERS_FAILED")
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "saved"})
}

// PortalSubmitAssessment handles POST /supplier/:token/submit (public, no auth).
func (h *Handler) PortalSubmitAssessment(c echo.Context) error {
	token := c.Param("token")
	var in SaveAnswersInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	clientIP := c.RealIP()
	userAgent := c.Request().Header.Get("User-Agent")
	if len(userAgent) > 512 {
		userAgent = userAgent[:512]
	}
	if err := h.service.SubmitAssessment(c.Request().Context(), token, clientIP, userAgent, in); err != nil {
		if errors.Is(err, ErrAssessmentExpiredOrSubmitted) {
			return c.JSON(http.StatusGone, map[string]string{"error": "assessment_expired_or_submitted"})
		}
		log.Error().Err(err).Msg("portal submit assessment")
		return errResp(c, http.StatusInternalServerError, "failed to submit assessment", "CK_SUBMIT_ASSESSMENT_FAILED")
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "submitted"})
}

// PortalUploadFile handles POST /supplier/:token/upload (public, no auth).
// Accepts a file (max 20 MB, allowed MIMEs: PDF/PNG/JPEG/XLSX).
func (h *Handler) PortalUploadFile(c echo.Context) error {
	token := c.Param("token")

	// Validate token is for a live assessment.
	a, err := h.service.GetAssessmentForPortal(c.Request().Context(), token)
	if err != nil {
		if errors.Is(err, ErrAssessmentExpiredOrSubmitted) {
			return c.JSON(http.StatusGone, map[string]string{"error": "assessment_expired_or_submitted"})
		}
		log.Error().Err(err).Msg("portal upload: validate token")
		return errResp(c, http.StatusInternalServerError, "failed to validate assessment", "CK_ASSESSMENT_LOAD_FAILED")
	}

	const maxUploadSize = 20 << 20 // 20 MB
	if err := c.Request().ParseMultipartForm(maxUploadSize); err != nil {
		return errResp(c, http.StatusBadRequest, "failed to parse multipart form", "CK_BAD_REQUEST")
	}

	fh, err := c.FormFile("file")
	if err != nil {
		return errResp(c, http.StatusBadRequest, "file is required", "CK_BAD_REQUEST")
	}
	if fh.Size > maxUploadSize {
		return errResp(c, http.StatusRequestEntityTooLarge, "file exceeds 20 MB limit", "CK_FILE_TOO_LARGE")
	}

	src, err := fh.Open()
	if err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to open uploaded file", "CK_UPLOAD_FAILED")
	}
	defer src.Close()

	// Read first 512 bytes for MIME detection.
	buf := make([]byte, 512)
	n, _ := src.Read(buf)
	detectedMIME := http.DetectContentType(buf[:n])

	ext := strings.ToLower(filepath.Ext(fh.Filename))
	allowedMIMEs := map[string]bool{
		"application/pdf": true,
		"image/png":       true,
		"image/jpeg":      true,
	}
	// XLSX is a ZIP archive: http.DetectContentType returns "application/zip".
	// Accept only when extension AND detected type agree to prevent file-rename bypass.
	xlsxAllowed := ext == ".xlsx" && detectedMIME == "application/zip"
	if !allowedMIMEs[detectedMIME] && !xlsxAllowed {
		return errResp(c, http.StatusUnsupportedMediaType, "unsupported file type", "CK_UNSUPPORTED_MIME")
	}

	uploadDir := h.uploadDir
	if uploadDir == "" {
		uploadDir = "./data/uploads"
	}
	assessmentDir := filepath.Join(uploadDir, "supplier-assessments", a.ID)
	if err := os.MkdirAll(assessmentDir, 0o750); err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to create upload directory", "CK_UPLOAD_FAILED")
	}

	destName := uuid.New().String() + ext
	destPath := filepath.Join(assessmentDir, destName)

	dst, err := os.Create(destPath)
	if err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to save file", "CK_UPLOAD_FAILED")
	}
	defer dst.Close()

	// Write already-read bytes first.
	if _, err := dst.Write(buf[:n]); err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to write file", "CK_UPLOAD_FAILED")
	}
	if _, err := io.Copy(dst, src); err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to write file", "CK_UPLOAD_FAILED")
	}

	// Return a relative URL rather than the raw filesystem path.
	fileURL := "/uploads/supplier-assessments/" + a.ID + "/" + destName
	return c.JSON(http.StatusOK, map[string]string{"file_url": fileURL})
}

// ReviewAnswer handles PATCH /secvitals/assessments/:id/answers/:aid.
func (h *Handler) ReviewAnswer(c echo.Context) error {
	orgID := orgID(c)
	assessmentID := c.Param("id")
	answerID := c.Param("aid")
	if _, err := uuid.Parse(assessmentID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid assessment ID", "CK_INVALID_ID")
	}
	if _, err := uuid.Parse(answerID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid answer ID", "CK_INVALID_ID")
	}
	var in ReviewAnswerInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusBadRequest, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	evidenceID, err := h.service.ReviewAnswer(c.Request().Context(), orgID, assessmentID, answerID, in)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResp(c, http.StatusNotFound, "answer not found", "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusBadRequest, err.Error(), "CK_VALIDATION_ERROR")
	}
	resp := map[string]any{"ok": true}
	if evidenceID != nil {
		resp["evidence_id"] = *evidenceID
	}
	return c.JSON(http.StatusOK, resp)
}

// GetSupplierStatus handles GET /secvitals/suppliers/:id/status.
func (h *Handler) GetSupplierStatus(c echo.Context) error {
	orgID := orgID(c)
	supplierID := c.Param("id")
	if _, err := uuid.Parse(supplierID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid supplier ID", "CK_INVALID_ID")
	}
	status, err := h.service.ComputeSupplierStatus(c.Request().Context(), orgID, supplierID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResp(c, http.StatusNotFound, "supplier not found", "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to compute status", "CK_INTERNAL")
	}
	return c.JSON(http.StatusOK, status)
}

// UpdateAssessment handles PATCH /secvitals/assessments/:id (status=reviewed only).
func (h *Handler) UpdateAssessment(c echo.Context) error {
	orgID := orgID(c)
	assessmentID := c.Param("id")
	if _, err := uuid.Parse(assessmentID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid assessment ID", "CK_INVALID_ID")
	}
	var in UpdateAssessmentInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusBadRequest, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	if err := h.service.MarkAssessmentReviewed(c.Request().Context(), orgID, assessmentID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResp(c, http.StatusNotFound, "assessment not found or not in submitted state", "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to update assessment", "CK_INTERNAL")
	}
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}

// GetAssessmentAnswers handles GET /secvitals/assessments/:id/answers.
func (h *Handler) GetAssessmentAnswers(c echo.Context) error {
	orgID := orgID(c)
	assessmentID := c.Param("id")
	if _, err := uuid.Parse(assessmentID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid assessment ID", "CK_INVALID_ID")
	}
	answers, err := h.service.GetAnswersForAssessment(c.Request().Context(), orgID, assessmentID)
	if err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to load answers", "CK_INTERNAL")
	}
	return c.JSON(http.StatusOK, answers)
}

// ExportAuditPackage handles GET /frameworks/:id/audit-package.zip.
// Returns a ZIP archive with INDEX.pdf, summary.json, and per-control evidence files.
func (h *Handler) ExportAuditPackage(c echo.Context) error {
	data, filename, err := h.service.ExportAuditPackage(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		log.Error().Err(err).Str("framework_id", c.Param("id")).Msg("export audit package")
		return errResp(c, http.StatusInternalServerError, "failed to generate audit package", "CK_AUDIT_PACKAGE_FAILED")
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	return c.Blob(http.StatusOK, "application/zip", data)
}

// GetAssessmentReportPDF handles GET /secvitals/assessments/:id/report-pdf.
func (h *Handler) GetAssessmentReportPDF(c echo.Context) error {
	orgID := orgID(c)
	assessmentID := c.Param("id")
	if _, err := uuid.Parse(assessmentID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid assessment ID", "CK_INVALID_ID")
	}
	pdf, err := h.service.GenerateAssessmentReportPDF(c.Request().Context(), orgID, assessmentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResp(c, http.StatusNotFound, "assessment not found", "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to generate PDF", "CK_INTERNAL")
	}
	c.Response().Header().Set("Content-Type", "application/pdf")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", "assessment-"+assessmentID+".pdf"))
	return c.Blob(http.StatusOK, "application/pdf", pdf)
}
