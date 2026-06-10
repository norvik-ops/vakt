package vaktcomply

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/audit"
	"github.com/matharnica/vakt/internal/shared/pagination"
)

// GetIncident handles GET /api/v1/vaktcomply/incidents/:id.
func (h *Handler) GetIncident(c echo.Context) error {
	id := c.Param("id")
	inc, err := h.service.GetIncident(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusNotFound, "incident not found", "CK_INCIDENT_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, inc)
}

// UpdateIncident handles PATCH /api/v1/vaktcomply/incidents/:id.
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

// ListIncidents handles GET /api/v1/vaktcomply/incidents.
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

// CreateIncident handles POST /api/v1/vaktcomply/incidents.
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
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
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

// AssessReportability handles POST /api/v1/vaktcomply/incidents/:id/assess-reportability.
func (h *Handler) AssessReportability(c echo.Context) error {
	id := c.Param("id")
	var in AssessReportabilityInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	result, err := h.service.AssessReportability(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "incident not found", "CK_INCIDENT_NOT_FOUND")
		}
		log.Error().Err(err).Str("incident_id", id).Msg("assess reportability")
		return errResp(c, http.StatusInternalServerError, "failed to assess reportability", "CK_ASSESS_FAILED")
	}
	return c.JSON(http.StatusOK, result)
}

// GenerateIncidentReportForm handles POST /api/v1/vaktcomply/incidents/:id/reports.
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
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "incident not found", "CK_INCIDENT_NOT_FOUND")
		}
		log.Error().Err(err).Str("incident_id", id).Msg("generate incident report form")
		return errResp(c, http.StatusInternalServerError, "failed to generate report", "CK_REPORT_FAILED")
	}
	return c.JSON(http.StatusCreated, report)
}

// ListIncidentReports handles GET /api/v1/vaktcomply/incidents/:id/reports.
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

// DownloadIncidentReportPDF handles GET /api/v1/vaktcomply/incident-reports/:reportId/pdf.
func (h *Handler) DownloadIncidentReportPDF(c echo.Context) error {
	reportID := c.Param("reportId")
	pdfBytes, err := h.service.GetIncidentReportPDF(c.Request().Context(), orgID(c), reportID)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "report not found", "CK_REPORT_NOT_FOUND")
		}
		log.Error().Err(err).Str("report_id", reportID).Msg("download incident report pdf")
		return errResp(c, http.StatusInternalServerError, "failed to retrieve PDF", "CK_PDF_FAILED")
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", "nis2-meldung-"+reportID+".pdf"))
	return c.Blob(http.StatusOK, "application/pdf", pdfBytes)
}

// MarkDeadlineReported handles POST /api/v1/vaktcomply/incidents/:id/mark-reported.
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

// IncidentReportPDF handles GET /api/v1/vaktcomply/incidents/:id/report-pdf.
// It streams a BaFin-style DORA incident report as a PDF download.
func (h *Handler) IncidentReportPDF(c echo.Context) error {
	id := c.Param("id")
	inc, err := h.service.GetIncident(c.Request().Context(), orgID(c), id)
	if err != nil {
		if isNotFound(err) {
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

// ClassifyReportingObligation handles POST /api/v1/vaktcomply/incidents/:id/classify-reporting.
// S39-1: 3-question BSI meldepflicht wizard — returns obligation + authority + reason.
func (h *Handler) ClassifyReportingObligation(c echo.Context) error {
	id := c.Param("id")
	var in ClassifyReportingInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	result, err := h.service.ClassifyReportingObligation(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "incident not found", "CK_INCIDENT_NOT_FOUND")
		}
		log.Error().Err(err).Str("incident_id", id).Msg("classify reporting obligation")
		return errResp(c, http.StatusInternalServerError, "failed to classify reporting obligation", "CK_CLASSIFY_FAILED")
	}
	return c.JSON(http.StatusOK, result)
}

// NIS2ReportingEnabled handles GET /api/v1/vaktcomply/nis2/enabled.
// License probe for the NIS2 reporting feature — the route itself is gated.
func (h *Handler) NIS2ReportingEnabled(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]bool{"enabled": true})
}

// NIS2AssessReportability handles POST /api/v1/vaktcomply/incidents/:id/nis2/assess.
// Stores the NIS2 meldepflicht assessment and sets deadline timers.
func (h *Handler) NIS2AssessReportability(c echo.Context) error {
	id := c.Param("id")
	var in struct {
		NIS2ReportabilityCheck
		DetectedAt *string `json:"detected_at"`
	}
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}

	detectedAt := time.Now().UTC()
	if in.DetectedAt != nil {
		if t, err := time.Parse(time.RFC3339, *in.DetectedAt); err == nil {
			detectedAt = t
		}
	}

	incidentID, err := uuid.Parse(id)
	if err != nil {
		return errResp(c, http.StatusBadRequest, "invalid incident id", "CK_BAD_REQUEST")
	}

	if err := h.service.MarkIncidentReportable(c.Request().Context(), orgID(c), incidentID, detectedAt, in.NIS2ReportabilityCheck); err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "incident not found", "CK_INCIDENT_NOT_FOUND")
		}
		log.Error().Err(err).Str("incident_id", id).Msg("nis2 assess reportability")
		return errResp(c, http.StatusInternalServerError, "failed to assess reportability", "CK_ASSESS_FAILED")
	}
	return c.JSON(http.StatusOK, map[string]any{
		"is_reportable": in.IsReportable(),
	})
}

// NIS2Status handles GET /api/v1/vaktcomply/incidents/:id/nis2-status.
func (h *Handler) NIS2Status(c echo.Context) error {
	id := c.Param("id")
	status, err := h.service.GetNIS2Status(c.Request().Context(), orgID(c), id)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "incident not found", "CK_INCIDENT_NOT_FOUND")
		}
		log.Error().Err(err).Str("incident_id", id).Msg("get nis2 status")
		return errResp(c, http.StatusInternalServerError, "failed to get nis2 status", "CK_NIS2_STATUS_FAILED")
	}
	return c.JSON(http.StatusOK, status)
}

// NIS2SubmitStage handles POST /api/v1/vaktcomply/incidents/:id/nis2/submit/:stage.
func (h *Handler) NIS2SubmitStage(c echo.Context) error {
	id := c.Param("id")
	stage := c.Param("stage")
	var in NIS2ReportInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}

	report, err := h.service.SubmitNIS2Stage(c.Request().Context(), orgID(c), id, userID(c), stage, in)
	if err != nil {
		log.Error().Err(err).Str("incident_id", id).Str("stage", stage).Msg("submit nis2 stage")
		return errResp(c, http.StatusInternalServerError, "failed to submit nis2 stage", "CK_NIS2_SUBMIT_FAILED")
	}
	return c.JSON(http.StatusOK, report)
}

// ListAuthorityContacts handles GET /api/v1/vaktcomply/authority-contacts.
func (h *Handler) ListAuthorityContacts(c echo.Context) error {
	contacts, err := h.service.ListAuthorityContacts(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list authority contacts")
		return errResp(c, http.StatusInternalServerError, "failed to list authority contacts", "CK_LIST_AUTH_CONTACTS_FAILED")
	}
	return c.JSON(http.StatusOK, contacts)
}

// CreateAuthorityContact handles POST /api/v1/vaktcomply/authority-contacts.
func (h *Handler) CreateAuthorityContact(c echo.Context) error {
	var in AuthorityContact
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	contact, err := h.service.CreateAuthorityContact(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create authority contact")
		return errResp(c, http.StatusInternalServerError, "failed to create authority contact", "CK_CREATE_AUTH_CONTACT_FAILED")
	}
	return c.JSON(http.StatusCreated, contact)
}
