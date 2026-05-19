package secvitals

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/auditlog"
	"github.com/matharnica/vakt/internal/shared/pagination"
)

// GetIncident handles GET /api/v1/secvitals/incidents/:id.
func (h *Handler) GetIncident(c echo.Context) error {
	id := c.Param("id")
	inc, err := h.service.GetIncident(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusNotFound, "incident not found", "CK_INCIDENT_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, inc)
}

// UpdateIncident handles PATCH /api/v1/secvitals/incidents/:id.
func (h *Handler) UpdateIncident(c echo.Context) error {
	id := c.Param("id")
	var in UpdateIncidentInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	inc, err := h.service.UpdateIncident(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		log.Error().Err(err).Msg("update incident")
		return errResp(c, http.StatusInternalServerError, "failed to update incident", "CK_UPDATE_INCIDENT_FAILED")
	}
	return c.JSON(http.StatusOK, inc)
}

// ListIncidents handles GET /api/v1/secvitals/incidents.
func (h *Handler) ListIncidents(c echo.Context) error {
	offset, limit, meta := pagination.FromRequest(c)
	incidents, total, err := h.service.ListIncidentsPaged(c.Request().Context(), orgID(c), offset, limit)
	if err != nil {
		log.Error().Err(err).Msg("list incidents")
		return errResp(c, http.StatusInternalServerError, "failed to list incidents", "CK_LIST_INCIDENTS_FAILED")
	}
	pagination.Complete(&meta, total)
	return c.JSON(http.StatusOK, pagination.Wrap(incidents, meta))
}

// CreateIncident handles POST /api/v1/secvitals/incidents.
func (h *Handler) CreateIncident(c echo.Context) error {
	var in CreateIncidentInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	incident, err := h.service.CreateIncident(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create incident")
		return errResp(c, http.StatusInternalServerError, "failed to create incident", "CK_CREATE_INCIDENT_FAILED")
	}
	auditlog.Log(c.Request().Context(), h.db, auditlog.Entry{
		OrgID:        orgID(c),
		UserID:       userID(c),
		Action:       "create",
		ResourceType: "vakt-comply/incident",
		ResourceID:   incident.ID,
		ResourceName: incident.Title,
		IPAddress:    c.RealIP(),
	})
	return c.JSON(http.StatusCreated, incident)
}

// AssessReportability handles POST /api/v1/secvitals/incidents/:id/assess-reportability.
func (h *Handler) AssessReportability(c echo.Context) error {
	id := c.Param("id")
	var in AssessReportabilityInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	result, err := h.service.AssessReportability(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "no rows") {
			return errResp(c, http.StatusNotFound, "incident not found", "CK_INCIDENT_NOT_FOUND")
		}
		log.Error().Err(err).Str("incident_id", id).Msg("assess reportability")
		return errResp(c, http.StatusInternalServerError, "failed to assess reportability", "CK_ASSESS_FAILED")
	}
	return c.JSON(http.StatusOK, result)
}

// GenerateIncidentReportForm handles POST /api/v1/secvitals/incidents/:id/reports.
func (h *Handler) GenerateIncidentReportForm(c echo.Context) error {
	id := c.Param("id")
	var body struct {
		ReportType string `json:"report_type" validate:"required,oneof=24h 72h 30d"`
	}
	if err := c.Bind(&body); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(body); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	report, _, err := h.service.GenerateIncidentReportForm(c.Request().Context(), orgID(c), id, body.ReportType, orgID(c))
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "no rows") {
			return errResp(c, http.StatusNotFound, "incident not found", "CK_INCIDENT_NOT_FOUND")
		}
		log.Error().Err(err).Str("incident_id", id).Msg("generate incident report form")
		return errResp(c, http.StatusInternalServerError, "failed to generate report", "CK_REPORT_FAILED")
	}
	return c.JSON(http.StatusCreated, report)
}

// ListIncidentReports handles GET /api/v1/secvitals/incidents/:id/reports.
func (h *Handler) ListIncidentReports(c echo.Context) error {
	id := c.Param("id")
	reports, err := h.service.ListIncidentReports(c.Request().Context(), orgID(c), id)
	if err != nil {
		log.Error().Err(err).Str("incident_id", id).Msg("list incident reports")
		return errResp(c, http.StatusInternalServerError, "failed to list reports", "CK_LIST_FAILED")
	}
	if reports == nil {
		reports = []IncidentReport{}
	}
	return c.JSON(http.StatusOK, reports)
}

// DownloadIncidentReportPDF handles GET /api/v1/secvitals/incident-reports/:reportId/pdf.
func (h *Handler) DownloadIncidentReportPDF(c echo.Context) error {
	reportID := c.Param("reportId")
	pdfBytes, err := h.service.GetIncidentReportPDF(c.Request().Context(), orgID(c), reportID)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return errResp(c, http.StatusNotFound, "report not found", "CK_REPORT_NOT_FOUND")
		}
		log.Error().Err(err).Str("report_id", reportID).Msg("download incident report pdf")
		return errResp(c, http.StatusInternalServerError, "failed to retrieve PDF", "CK_PDF_FAILED")
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", "nis2-meldung-"+reportID+".pdf"))
	return c.Blob(http.StatusOK, "application/pdf", pdfBytes)
}

// MarkDeadlineReported handles POST /api/v1/secvitals/incidents/:id/mark-reported.
func (h *Handler) MarkDeadlineReported(c echo.Context) error {
	id := c.Param("id")
	var in MarkDeadlineReportedInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	inc, err := h.service.MarkDeadlineReported(c.Request().Context(), orgID(c), id, in.Deadline)
	if err != nil {
		log.Error().Err(err).Msg("mark deadline reported")
		return errResp(c, http.StatusInternalServerError, "failed to mark deadline", "CK_MARK_DEADLINE_FAILED")
	}
	return c.JSON(http.StatusOK, inc)
}

// IncidentReportPDF handles GET /api/v1/secvitals/incidents/:id/report-pdf.
// It streams a BaFin-style DORA incident report as a PDF download.
func (h *Handler) IncidentReportPDF(c echo.Context) error {
	id := c.Param("id")
	inc, err := h.service.GetIncident(c.Request().Context(), orgID(c), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "no rows") {
			return errResp(c, http.StatusNotFound, "incident not found", "CK_INCIDENT_NOT_FOUND")
		}
		log.Error().Err(err).Str("incident_id", id).Msg("get incident for pdf")
		return errResp(c, http.StatusInternalServerError, "failed to retrieve incident", "CK_GET_INCIDENT_FAILED")
	}

	// Use org_id as a stand-in for org name when no name is available via context.
	// In production the org name can be resolved from the claims or a lookup.
	org := orgID(c)

	pdfBytes, err := GenerateIncidentReportPDF(inc, org)
	if err != nil {
		log.Error().Err(err).Str("incident_id", id).Msg("generate incident report pdf")
		return errResp(c, http.StatusInternalServerError, "failed to generate PDF", "CK_PDF_FAILED")
	}

	filename := fmt.Sprintf("incident-%s-bafin.pdf", inc.ID)
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	return c.Blob(http.StatusOK, "application/pdf", pdfBytes)
}
