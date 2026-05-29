package audit

import (
	"fmt"
	"net/http"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// Handler handles the audit report endpoint.
type ReportHandler struct {
	db *pgxpool.Pool
}

// NewHandler creates a new Handler.
func NewReportHandler(db *pgxpool.Pool) *ReportHandler {
	return &ReportHandler{db: db}
}

// GenerateAuditReport handles GET /api/v1/vaktcomply/audit-report.
// It collects all compliance data for the organisation and renders a PDF.
func (h *ReportHandler) GenerateAuditReport(c echo.Context) error {
	orgID, ok := c.Get("org_id").(string)
	if !ok || orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "unauthorized",
			"code":  "AR_UNAUTHORIZED",
		})
	}

	ctx := c.Request().Context()

	data, err := Collect(ctx, h.db, orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("audit report: gather failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Bericht konnte nicht erstellt werden.",
			"code":  "AR_GATHER_ERROR",
		})
	}

	pdfBytes, err := Render(data)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("audit report: render failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "PDF-Generierung fehlgeschlagen.",
			"code":  "AR_RENDER_ERROR",
		})
	}

	safeName := sanitiseFilename(data.OrgName)
	dateStr := data.GeneratedAt.Format("2006-01-02")
	filename := fmt.Sprintf("vakt-audit-%s-%s.pdf", safeName, dateStr)

	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	return c.Blob(http.StatusOK, "application/pdf", pdfBytes)
}

// sanitiseFilename keeps only alphanumeric, dash, and underscore characters.
func sanitiseFilename(s string) string {
	safe := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, s)
	// Collapse repeated dashes
	for strings.Contains(safe, "--") {
		safe = strings.ReplaceAll(safe, "--", "-")
	}
	safe = strings.Trim(safe, "-")
	if safe == "" {
		safe = "org"
	}
	return safe
}
