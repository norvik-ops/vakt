package vaktcomply

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/matharnica/vakt/internal/db"
	"github.com/matharnica/vakt/internal/shared/audit"
	"github.com/matharnica/vakt/internal/shared/docxexport"
	"github.com/matharnica/vakt/internal/shared/platform/features"
	"github.com/matharnica/vakt/internal/shared/veriniceimport"
	"github.com/matharnica/vakt/internal/shared/xlsxexport"
	"github.com/rs/zerolog/log"
)

func (h *Handler) ListDBPolicyTemplates(c echo.Context) error {
	ctx := c.Request().Context()
	category := c.QueryParam("category")

	if category != "" && category != "policy" && category != "dpia" && category != "avv" {
		return errResp(c, http.StatusBadRequest, "invalid category; must be policy, dpia, or avv", "INVALID_CATEGORY")
	}

	templates, queryErr := h.service.ListPolicyTemplates(ctx, category)
	if queryErr != nil {
		log.Error().Err(queryErr).Msg("ListDBPolicyTemplates: query failed")
		return errResp(c, http.StatusInternalServerError, "failed to list templates", "DB_ERROR")
	}
	return c.JSON(http.StatusOK, templates)
}

// GetDBPolicyTemplate handles GET /api/v1/vaktcomply/templates/:id
//
// Returns a single DB-backed compliance template by UUID.
func (h *Handler) GetDBPolicyTemplate(c echo.Context) error {
	ctx := c.Request().Context()
	id := c.Param("id")
	if id == "" {
		return errResp(c, http.StatusBadRequest, "missing template id", "MISSING_ID")
	}

	tmpl, err := h.service.GetPolicyTemplate(ctx, id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("GetDBPolicyTemplate: not found")
		return errResp(c, http.StatusNotFound, "template not found", "TEMPLATE_NOT_FOUND")
	}

	return c.JSON(http.StatusOK, tmpl)
}

// templateListRowToDTO converts a ListCKPolicyTemplatesRow to DBPolicyTemplate.
// COALESCE(framework, ”) means empty string signals DB NULL — convert back to nil.
func templateListRowToDTO(r db.ListCKPolicyTemplatesRow) DBPolicyTemplate {
	var fw *string
	if r.Framework != "" {
		fw = &r.Framework
	}
	return DBPolicyTemplate{
		ID:          r.ID,
		Category:    r.Category,
		Name:        r.Name,
		Description: r.Description,
		Content:     r.Content,
		Tags:        r.Tags,
		Framework:   fw,
		CreatedAt:   r.CreatedAt,
	}
}

// templateGetRowToDTO converts a GetCKPolicyTemplateByIDRow to DBPolicyTemplate.
func templateGetRowToDTO(r db.GetCKPolicyTemplateByIDRow) DBPolicyTemplate {
	var fw *string
	if r.Framework != "" {
		fw = &r.Framework
	}
	return DBPolicyTemplate{
		ID:          r.ID,
		Category:    r.Category,
		Name:        r.Name,
		Description: r.Description,
		Content:     r.Content,
		Tags:        r.Tags,
		Framework:   fw,
		CreatedAt:   r.CreatedAt,
	}
}

const xlsxContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

// ExportRisksXLSX handles GET /api/v1/vaktcomply/risks/export/xlsx.
// Returns all org risks as a proper XLSX workbook (two sheets: Risiken + Matrix).
// Requires FeatureAuditPDF.
func (h *Handler) ExportRisksXLSX(c echo.Context) error {
	if !features.IsEnabled(c, features.FeatureAuditPDF) {
		return errResp(c, http.StatusPaymentRequired, "XLSX export requires Pro", "CK_FEATURE_REQUIRED")
	}
	ctx := c.Request().Context()
	org := orgID(c)

	risks, _, err := h.service.Risk.ListRisksPaged(ctx, org, 0, 10_000)
	if err != nil {
		log.Error().Err(err).Str("org_id", org).Msg("export risks xlsx")
		return errResp(c, http.StatusInternalServerError, "export failed", "CK_EXPORT_ERROR")
	}

	rows := make([]xlsxexport.RiskRow, len(risks))
	for i, r := range risks {
		rows[i] = xlsxexport.RiskRow{
			ID:            r.ID,
			Title:         r.Title,
			Category:      r.Category,
			Likelihood:    r.Likelihood,
			Impact:        r.Impact,
			RiskScore:     r.RiskScore,
			Treatment:     r.Treatment,
			Status:        r.Status,
			Owner:         r.Owner,
			DueDate:       r.TreatmentDueDate,
			ResidualScore: r.ResidualScore,
		}
	}

	data, err := xlsxexport.RenderRisiken(rows)
	if err != nil {
		log.Error().Err(err).Msg("export risks xlsx: render")
		return errResp(c, http.StatusInternalServerError, "export failed", "CK_EXPORT_ERROR")
	}

	filename := fmt.Sprintf("risikoregister-%s.xlsx", time.Now().UTC().Format("2006-01-02"))
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	return c.Blob(http.StatusOK, xlsxContentType, data)
}

// ExportControlsXLSX handles GET /api/v1/vaktcomply/controls/export/xlsx.
// Optional query param: framework_id to filter controls by framework.
// Returns columns: Title, Framework, Status, Owner, Due Date.
func (h *Handler) ExportControlsXLSX(c echo.Context) error {
	ctx := c.Request().Context()
	org := orgID(c)
	frameworkID := c.QueryParam("framework_id")
	if frameworkID == "" {
		return errResp(c, http.StatusBadRequest, "framework_id is required", "CK_MISSING_PARAM")
	}

	controls, err := h.service.ListControls(ctx, org, frameworkID)
	if err != nil {
		log.Error().Err(err).Str("org_id", org).Str("framework_id", frameworkID).Msg("export controls xlsx")
		return errResp(c, http.StatusInternalServerError, "export failed", "CK_EXPORT_ERROR")
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"Title", "Framework", "Status", "Owner", "Due Date"})
	for _, ctrl := range controls {
		dueDate := ""
		if ctrl.NextReviewDue != nil {
			dueDate = ctrl.NextReviewDue.Format(time.DateOnly)
		}
		_ = w.Write([]string{
			ctrl.Title,
			ctrl.FrameworkID,
			ctrl.Status,
			ctrl.LastReviewedBy,
			dueDate,
		})
	}
	w.Flush()

	c.Response().Header().Set("Content-Disposition", `attachment; filename="controls.xlsx"`)
	return c.Blob(http.StatusOK, xlsxContentType, buf.Bytes())
}

func (h *Handler) CalendarDeadlines(c echo.Context) error {
	ctx := c.Request().Context()
	oid := orgID(c)
	now := time.Now().UTC()

	type icalEvent struct {
		uid         string
		dtstart     string
		summary     string
		description string
	}

	var events []icalEvent

	// S121-F6 (A3): deadline data now comes from the service layer, not h.q.*.
	deadlines, err := h.service.ListICalDeadlines(ctx, oid)
	if err != nil {
		log.Error().Err(err).Str("org_id", oid).Msg("ical: load deadlines")
		return errResp(c, http.StatusInternalServerError, "failed to load deadlines", "CK_ICAL_ERROR")
	}

	// --- Source 1: Audit milestones ---
	for _, m := range deadlines.Milestones {
		events = append(events, icalEvent{
			uid:         m.ID + "@vakt",
			dtstart:     m.MilestoneDate.Time.Format("20060102"),
			summary:     m.Title,
			description: m.Description,
		})
	}

	// --- Source 2: Open/in-progress CAPAs with due dates ---
	for _, ca := range deadlines.CAPAs {
		events = append(events, icalEvent{
			uid:         ca.ID + "@vakt",
			dtstart:     ca.DueDate.Time.Format("20060102"),
			summary:     "CAPA fällig: " + ca.Title,
			description: "Corrective and Preventive Action",
		})
	}

	// --- Source 3: Evidence expiring within the future ---
	for _, ev := range deadlines.Evidence {
		events = append(events, icalEvent{
			uid:         ev.ID + "@vakt",
			dtstart:     ev.ExpiresAt.Time.UTC().Format("20060102"),
			summary:     "Nachweis läuft ab: " + ev.Title,
			description: "Compliance-Nachweis läuft ab",
		})
	}

	// Build iCalendar output.
	dtstamp := now.Format("20060102T150405Z")
	var sb strings.Builder
	sb.WriteString("BEGIN:VCALENDAR\r\n")
	sb.WriteString("VERSION:2.0\r\n")
	sb.WriteString("PRODID:-//Vakt//Compliance Calendar//DE\r\n")
	sb.WriteString("CALSCALE:GREGORIAN\r\n")
	sb.WriteString("X-WR-CALNAME:Vakt Compliance\r\n")

	for _, ev := range events {
		sb.WriteString("BEGIN:VEVENT\r\n")
		fmt.Fprintf(&sb, "UID:%s\r\n", icalEscape(ev.uid))
		fmt.Fprintf(&sb, "DTSTAMP:%s\r\n", dtstamp)
		fmt.Fprintf(&sb, "DTSTART;VALUE=DATE:%s\r\n", ev.dtstart)
		fmt.Fprintf(&sb, "SUMMARY:%s\r\n", icalEscape(ev.summary))
		if ev.description != "" {
			fmt.Fprintf(&sb, "DESCRIPTION:%s\r\n", icalEscape(ev.description))
		}
		sb.WriteString("END:VEVENT\r\n")
	}

	sb.WriteString("END:VCALENDAR\r\n")

	c.Response().Header().Set("Content-Type", "text/calendar; charset=utf-8")
	c.Response().Header().Set("Content-Disposition", `attachment; filename="vakt-compliance.ics"`)
	return c.String(http.StatusOK, sb.String())
}

// icalEscape escapes special characters in iCalendar text values per RFC 5545.
func icalEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, ";", `\;`)
	s = strings.ReplaceAll(s, ",", `\,`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

func (h *Handler) logDocxExport(c echo.Context, resourceType, resourceName string, data []byte) {
	sum := sha256.Sum256(data)
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID: orgID(c), UserID: userID(c), Action: "export",
		ResourceType: resourceType, ResourceName: resourceName,
		IPAddress: c.RealIP(),
		Details:   map[string]string{"format": "docx", "sha256": hex.EncodeToString(sum[:]), "bytes": fmt.Sprintf("%d", len(data))},
	})
}

// ExportRisksDOCX handles GET /api/v1/vaktcomply/risks/export/docx (Pro).
func (h *Handler) ExportRisksDOCX(c echo.Context) error {
	if !features.IsEnabled(c, features.FeatureAuditPDF) {
		return errResp(c, http.StatusPaymentRequired, "DOCX export requires Pro", "CK_FEATURE_REQUIRED")
	}
	ctx := c.Request().Context()
	org := orgID(c)

	risks, _, err := h.service.Risk.ListRisksPaged(ctx, org, 0, 10_000)
	if err != nil {
		log.Error().Err(err).Str("org_id", org).Msg("export risks docx")
		return errResp(c, http.StatusInternalServerError, "export failed", "CK_EXPORT_ERROR")
	}

	rows := make([]docxexport.RiskRow, len(risks))
	for i, r := range risks {
		rows[i] = docxexport.RiskRow{
			ID: r.ID, Title: r.Title, Category: r.Category,
			Likelihood: r.Likelihood, Impact: r.Impact, RiskScore: r.RiskScore,
			Treatment: r.Treatment, Status: r.Status, Owner: r.Owner,
			DueDate: r.TreatmentDueDate, ResidualScore: r.ResidualScore,
		}
	}

	data, err := docxexport.RenderRisiken(rows)
	if err != nil {
		log.Error().Err(err).Msg("export risks docx: render")
		return errResp(c, http.StatusInternalServerError, "export failed", "CK_EXPORT_ERROR")
	}
	h.logDocxExport(c, "vakt-comply/risk-register", "Risikoregister", data)

	filename := fmt.Sprintf("risikoregister-%s.docx", time.Now().UTC().Format("2006-01-02"))
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	return c.Blob(http.StatusOK, docxexport.ContentType, data)
}

// ExportSoADOCX handles GET /api/v1/vaktcomply/soa/export.docx (Pro).
func (h *Handler) ExportSoADOCX(c echo.Context) error {
	if !features.IsEnabled(c, features.FeatureAuditPDF) {
		return errResp(c, http.StatusPaymentRequired, "DOCX export requires Pro", "CK_FEATURE_REQUIRED")
	}
	ctx := c.Request().Context()
	org := orgID(c)

	entries, err := h.service.ListDedicatedSoAEntries(ctx, org)
	if err != nil {
		if errors.Is(err, ErrSoANotInitialized) {
			return errResp(c, http.StatusNotFound, err.Error(), "CK_SOA_NOT_INITIALIZED")
		}
		log.Error().Err(err).Msg("export soa docx: list entries")
		return errResp(c, http.StatusInternalServerError, "export failed", "CK_SOA_EXPORT_FAILED")
	}
	summary, err := h.service.GetDedicatedSoASummary(ctx, org)
	if err != nil {
		log.Error().Err(err).Msg("export soa docx: get summary")
		return errResp(c, http.StatusInternalServerError, "export failed", "CK_SOA_EXPORT_FAILED")
	}

	rows := make([]docxexport.SoARow, len(entries))
	for i, e := range entries {
		justification := e.Justification
		if !e.Applicable && e.ExclusionReason != "" {
			justification = e.ExclusionReason
		}
		owner := ""
		if e.ApprovedBy != nil {
			owner = *e.ApprovedBy
		}
		rows[i] = docxexport.SoARow{
			ControlRef: e.ControlRef, ControlName: e.ControlName, ControlGroup: e.ControlGroup,
			Applicable: e.Applicable, Justification: justification,
			ImplementationStatus: e.ImplementationStatus, Owner: owner, UpdatedAt: e.UpdatedAt,
		}
	}
	var sum docxexport.SoASummary
	if summary != nil {
		sum = docxexport.SoASummary{
			ApplicableCount:   summary.ApplicableCount,
			ExcludedCount:     summary.ExcludedCount,
			ImplementedCount:  summary.ImplementedCount,
			ImplementationPct: summary.ImplementationPct,
		}
	}

	data, err := docxexport.RenderSoA(rows, sum)
	if err != nil {
		log.Error().Err(err).Msg("export soa docx: render")
		return errResp(c, http.StatusInternalServerError, "export failed", "CK_SOA_EXPORT_FAILED")
	}
	h.logDocxExport(c, "vakt-comply/soa", "Statement of Applicability", data)

	filename := fmt.Sprintf("soa-%s.docx", time.Now().UTC().Format("2006-01-02"))
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	return c.Blob(http.StatusOK, docxexport.ContentType, data)
}

func (h *Handler) ListPhysicalControlTemplates(c echo.Context) error {
	return c.JSON(http.StatusOK, h.service.ListPhysicalControlTemplates())
}

// ApplyPhysicalControlTemplate handles POST /api/v1/vaktcomply/physical-templates/:code/apply
func (h *Handler) ApplyPhysicalControlTemplate(c echo.Context) error {
	code := c.Param("code")
	ev, err := h.service.ApplyPhysicalControlTemplate(c.Request().Context(), orgID(c), code, userID(c))
	if err != nil {
		log.Warn().Err(err).Str("control_code", code).Msg("apply physical template")
		return errResp(c, http.StatusBadRequest, err.Error(), "CK_PHYS_TEMPLATE_FAILED")
	}
	return c.JSON(http.StatusCreated, ev)
}

func readVNAUpload(c echo.Context) ([]byte, error) {
	fh, err := c.FormFile("file")
	if err != nil {
		return nil, errResp(c, http.StatusBadRequest, "file is required", "CK_BAD_REQUEST")
	}
	if fh.Size > veriniceimport.MaxArchiveSize {
		return nil, errResp(c, http.StatusRequestEntityTooLarge, "file too large", "CK_FILE_TOO_LARGE")
	}
	src, err := fh.Open()
	if err != nil {
		return nil, errResp(c, http.StatusInternalServerError, "failed to open upload", "CK_UPLOAD_FAILED")
	}
	defer src.Close()
	data, err := io.ReadAll(io.LimitReader(src, veriniceimport.MaxArchiveSize+1))
	if err != nil {
		return nil, errResp(c, http.StatusInternalServerError, "failed to read upload", "CK_UPLOAD_FAILED")
	}
	return data, nil
}

// PreviewVeriniceImport handles POST /api/v1/vaktcomply/verinice-import/preview
func (h *Handler) PreviewVeriniceImport(c echo.Context) error {
	data, errResponse := readVNAUpload(c)
	if errResponse != nil {
		return errResponse
	}
	preview, err := h.service.PreviewVeriniceImport(data)
	if err != nil {
		log.Warn().Err(err).Msg("verinice import preview")
		return errResp(c, http.StatusBadRequest, "failed to parse .vna file", "CK_VNA_PARSE_FAILED")
	}
	return c.JSON(http.StatusOK, preview)
}

// CommitVeriniceImport handles POST /api/v1/vaktcomply/verinice-import/commit
func (h *Handler) CommitVeriniceImport(c echo.Context) error {
	data, errResponse := readVNAUpload(c)
	if errResponse != nil {
		return errResponse
	}
	res, err := h.service.CommitVeriniceImport(c.Request().Context(), orgID(c), userID(c), data)
	if err != nil {
		log.Warn().Err(err).Msg("verinice import commit")
		return errResp(c, http.StatusBadRequest, "failed to import .vna file", "CK_VNA_IMPORT_FAILED")
	}
	// Structured audit-log entry per import (who, what counts).
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID: orgID(c), UserID: userID(c), Action: "import",
		ResourceType: "vakt-comply/verinice-import",
		ResourceName: "verinice .vna",
		IPAddress:    c.RealIP(),
		Details: map[string]string{
			"assets_created":   strconv.Itoa(res.AssetsCreated),
			"controls_created": strconv.Itoa(res.ControlsCreated),
			"risks_created":    strconv.Itoa(res.RisksCreated),
			"skipped":          strconv.Itoa(res.Skipped),
		},
	})
	return c.JSON(http.StatusOK, res)
}
