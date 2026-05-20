package secvitals

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// --- AI System Inventory ---

// ListAISystems handles GET /api/v1/secvitals/ai-systems.
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

// DeleteAISystem handles DELETE /api/v1/secvitals/ai-systems/:id.
func (h *Handler) DeleteAISystem(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid AI system ID", "CK_INVALID_ID")
	}
	if err := h.service.DeleteAISystem(c.Request().Context(), orgID(c), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResp(c, http.StatusNotFound, "AI system not found", "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to delete AI system", "CK_INTERNAL")
	}
	return c.NoContent(http.StatusNoContent)
}

// CreateAISystem handles POST /api/v1/secvitals/ai-systems.
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

// GetAISystem handles GET /api/v1/secvitals/ai-systems/:id.
func (h *Handler) GetAISystem(c echo.Context) error {
	a, err := h.service.GetAISystem(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		return errResp(c, http.StatusNotFound, "AI system not found", "CK_AI_SYSTEM_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, a)
}

// UpdateAISystem handles PATCH /api/v1/secvitals/ai-systems/:id.
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

// ClassifyAISystem handles POST /api/v1/secvitals/ai-systems/:id/classify.
func (h *Handler) ClassifyAISystem(c echo.Context) error {
	id := c.Param("id")
	var in ClassifyAISystemInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.service.ClassifyAISystem(c.Request().Context(), orgID(c), id, in); err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResp(c, http.StatusNotFound, "AI system not found", "CK_NOT_FOUND")
		}
		log.Error().Err(err).Msg("classify ai system")
		return errResp(c, http.StatusInternalServerError, "failed to classify AI system", "CK_CLASSIFY_AI_SYSTEM_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ListAIClassifications handles GET /api/v1/secvitals/ai-systems/:id/classifications.
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

// SaveAIDocumentation handles POST /api/v1/secvitals/ai-systems/:id/documentation.
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

// GetLatestAIDocumentation handles GET /api/v1/secvitals/ai-systems/:id/documentation.
func (h *Handler) GetLatestAIDocumentation(c echo.Context) error {
	id := c.Param("id")
	doc, err := h.service.GetLatestAIDocumentation(c.Request().Context(), orgID(c), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResp(c, http.StatusNotFound, "documentation not found", "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to get documentation", "CK_GET_AI_DOC_FAILED")
	}
	return c.JSON(http.StatusOK, doc)
}

// ListAIDocumentationVersions handles GET /api/v1/secvitals/ai-systems/:id/documentation/versions.
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

// ExportAIDocumentationPDF handles GET /api/v1/secvitals/ai-systems/:id/documentation/export-pdf.
func (h *Handler) ExportAIDocumentationPDF(c echo.Context) error {
	id := c.Param("id")
	pdfBytes, filename, err := h.service.ExportAIDocumentationPDF(c.Request().Context(), orgID(c), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
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

// GetOrgSector handles GET /api/v1/secvitals/org-sector.
func (h *Handler) GetOrgSector(c echo.Context) error {
	settings, err := h.service.GetOrgSector(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get org sector")
		return errResp(c, http.StatusInternalServerError, "failed to get org sector", "CK_GET_SECTOR_FAILED")
	}
	return c.JSON(http.StatusOK, settings)
}

// UpdateOrgSector handles PATCH /api/v1/secvitals/org-sector.
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

// ListAuthorities handles GET /api/v1/secvitals/authorities.
func (h *Handler) ListAuthorities(c echo.Context) error {
	all := ListAllAuthorities()
	return c.JSON(http.StatusOK, all)
}

// GetOrgAuthorities handles GET /api/v1/secvitals/org-authorities — sector-specific.
func (h *Handler) GetOrgAuthorities(c echo.Context) error {
	authorities, err := h.service.GetAuthoritiesForOrg(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get org authorities")
		return errResp(c, http.StatusInternalServerError, "failed to get authorities", "CK_GET_AUTHORITIES_FAILED")
	}
	return c.JSON(http.StatusOK, authorities)
}

// GetEUAIActDashboard handles GET /api/v1/secvitals/eu-ai-act/dashboard.
func (h *Handler) GetEUAIActDashboard(c echo.Context) error {
	dashboard, err := h.service.GetEUAIActDashboard(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get eu ai act dashboard")
		return errResp(c, http.StatusInternalServerError, "failed to get EU AI Act dashboard", "CK_EU_AI_ACT_DASHBOARD_FAILED")
	}
	return c.JSON(http.StatusOK, dashboard)
}

// GetEUAIActReportPDF handles GET /api/v1/secvitals/eu-ai-act/report-pdf.
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

// ListResilienceTests handles GET /api/v1/secvitals/resilience-tests.
func (h *Handler) ListResilienceTests(c echo.Context) error {
	tests, tlptOverdue, err := h.service.ListResilienceTests(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list resilience tests")
		return errResp(c, http.StatusInternalServerError, "failed to list resilience tests", "CK_LIST_RESILIENCE_TESTS_FAILED")
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"tests":                tests,
		"tlpt_overdue_warning": tlptOverdue,
	})
}

// CreateResilienceTest handles POST /api/v1/secvitals/resilience-tests.
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

// GetResilienceTest handles GET /api/v1/secvitals/resilience-tests/:id.
func (h *Handler) GetResilienceTest(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid resilience test id", "CK_BAD_REQUEST")
	}
	t, err := h.service.GetResilienceTest(c.Request().Context(), orgID(c), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "no rows") {
			return errResp(c, http.StatusNotFound, "resilience test not found", "CK_RESILIENCE_TEST_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("get resilience test")
		return errResp(c, http.StatusInternalServerError, "failed to get resilience test", "CK_GET_RESILIENCE_TEST_FAILED")
	}
	return c.JSON(http.StatusOK, t)
}

// UpdateResilienceTest handles PATCH /api/v1/secvitals/resilience-tests/:id.
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
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "no rows") {
			return errResp(c, http.StatusNotFound, "resilience test not found", "CK_RESILIENCE_TEST_NOT_FOUND")
		}
		log.Error().Err(err).Msg("update resilience test")
		return errResp(c, http.StatusInternalServerError, "failed to update resilience test", "CK_UPDATE_RESILIENCE_TEST_FAILED")
	}
	return c.JSON(http.StatusOK, t)
}

// DeleteResilienceTest handles DELETE /api/v1/secvitals/resilience-tests/:id.
func (h *Handler) DeleteResilienceTest(c echo.Context) error {
	id := c.Param("id")
	if err := h.service.DeleteResilienceTest(c.Request().Context(), orgID(c), id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errResp(c, http.StatusNotFound, "resilience test not found", "CK_RESILIENCE_TEST_NOT_FOUND")
		}
		log.Error().Err(err).Msg("delete resilience test")
		return errResp(c, http.StatusInternalServerError, "failed to delete resilience test", "CK_DELETE_RESILIENCE_TEST_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// UploadResilienceTestAttachment handles POST /api/v1/secvitals/resilience-tests/:id/attachment.
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

// GetDORADashboard handles GET /api/v1/secvitals/dora/dashboard.
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

// GetDORAPDF handles GET /api/v1/secvitals/dora/report-pdf.
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

// GetExecutiveSummaryPDF handles GET /api/v1/secvitals/reports/executive-summary.
func (h *Handler) GetExecutiveSummaryPDF(c echo.Context) error {
	pdfBytes, filename, err := h.service.ExportExecutiveSummaryPDF(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("generate executive summary pdf")
		return errResp(c, http.StatusInternalServerError, "failed to generate executive summary PDF", "CK_EXECUTIVE_SUMMARY_FAILED")
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	return c.Blob(http.StatusOK, "application/pdf", pdfBytes)
}
