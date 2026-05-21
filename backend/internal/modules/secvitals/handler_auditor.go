package secvitals

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// CreateAuditorLink handles POST /api/v1/secvitals/frameworks/:id/auditor-link.
func (h *Handler) CreateAuditorLink(c echo.Context) error {
	frameworkID := c.Param("id")
	var body struct {
		ExpiresInHours int  `json:"expires_in_hours" validate:"required,min=1,max=8760"`
		MaxUses        *int `json:"max_uses"         validate:"omitempty,min=1"`
	}
	if err := c.Bind(&body); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(body); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": "Ungültige Eingabe",
			"code":  "VALIDATION_ERROR",
		})
	}

	expiresIn := time.Duration(body.ExpiresInHours) * time.Hour
	rawToken, err := h.service.CreateAuditorLink(c.Request().Context(), orgID(c), frameworkID, userID(c), expiresIn, body.MaxUses)
	if err != nil {
		log.Error().Err(err).Str("framework_id", frameworkID).Msg("create auditor link")
		return errResp(c, http.StatusInternalServerError, "failed to create auditor link", "CK_AUDITOR_LINK_FAILED")
	}

	return c.JSON(http.StatusCreated, map[string]string{
		"auditor_url": "/api/v1/secvitals/auditor/" + rawToken,
	})
}

// AuditorView handles GET /api/v1/secvitals/auditor/:token — no auth required.
func (h *Handler) AuditorView(c echo.Context) error {
	rawToken := c.Param("token")
	fw, err := h.service.ValidateAuditorLink(c.Request().Context(), rawToken)
	if err != nil {
		log.Debug().Err(err).Msg("auditor link validation failed")
		return errResp(c, http.StatusNotFound, "invalid or expired auditor link", "CK_AUDITOR_LINK_INVALID")
	}

	// Return a read-only framework view — report without org-internal details.
	report, err := h.service.GetReadinessReport(c.Request().Context(), fw.OrgID, fw.ID)
	if err != nil {
		log.Error().Err(err).Str("framework_id", fw.ID).Msg("auditor view: readiness report")
		return errResp(c, http.StatusInternalServerError, "failed to generate report", "CK_AUDITOR_REPORT_FAILED")
	}

	return c.JSON(http.StatusOK, map[string]any{
		"framework": fw,
		"report":    report,
	})
}

// ListAuditorLinks handles GET /api/v1/secvitals/auditor-links.
func (h *Handler) ListAuditorLinks(c echo.Context) error {
	links, err := h.service.ListAuditorLinks(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list auditor links")
		return errResp(c, http.StatusInternalServerError, "failed to list auditor links", "CK_LIST_AUDITOR_LINKS_FAILED")
	}
	return c.JSON(http.StatusOK, links)
}

// RevokeAuditorLink handles DELETE /api/v1/secvitals/auditor-links/:id.
// Returns HTTP 410 Gone if the link is already revoked.
func (h *Handler) RevokeAuditorLink(c echo.Context) error {
	linkID := c.Param("id")
	if linkID == "" {
		return errResp(c, http.StatusBadRequest, "link id is required", "CK_BAD_REQUEST")
	}

	if err := h.service.RevokeAuditorLink(c.Request().Context(), orgID(c), linkID); err != nil {
		log.Error().Err(err).Str("link_id", linkID).Msg("revoke auditor link")
		return errResp(c, http.StatusGone, "auditor link not found or already revoked", "CK_AUDITOR_LINK_GONE")
	}
	return c.NoContent(http.StatusNoContent)
}

// AuditorExportBundle handles GET /api/v1/secvitals/auditor/:token/export — no auth required.
// Returns a ZIP archive of all framework controls with their evidence metadata.
func (h *Handler) AuditorExportBundle(c echo.Context) error {
	rawToken := c.Param("token")
	ctx := c.Request().Context()

	// ExportAuditorBundle validates the token, writes the ZIP to the writer,
	// and returns the framework name for the Content-Disposition header.
	// We must set headers before writing, so we buffer via ExportAuditorBundle
	// which streams directly; we pre-set headers and let it write.
	//
	// To keep headers accurate we do a lightweight token check first.
	fwName, err := h.service.PreflightAuditorExport(ctx, rawToken)
	if err != nil {
		log.Debug().Err(err).Msg("auditor export bundle: preflight failed")
		return errResp(c, http.StatusNotFound, "invalid or expired auditor link", "CK_AUDITOR_LINK_INVALID")
	}

	filename := fmt.Sprintf("%s-evidence-bundle.zip", fwName)
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Response().Header().Set("Content-Type", "application/zip")
	c.Response().WriteHeader(http.StatusOK)

	if _, err := h.service.ExportAuditorBundle(ctx, rawToken, c.Response().Writer); err != nil {
		log.Error().Err(err).Msg("auditor export bundle: write failed")
		// Headers already sent; cannot return JSON — log only.
		return nil
	}
	return nil
}

// AuditorExportZIP handles GET /auditor/secvitals/export.zip.
// Produces a ZIP containing JSON snapshots of controls, risks, incidents, policies, and audit records.
// The route is behind AuditorAuth middleware — org_id is set in context.
func (h *Handler) AuditorExportZIP(c echo.Context) error {
	ctx := c.Request().Context()
	oid := orgID(c)

	risks, err := h.service.ListRisks(ctx, oid)
	if err != nil {
		log.Error().Err(err).Str("org_id", oid).Msg("auditor export zip: list risks")
		return errResp(c, http.StatusInternalServerError, "failed to build export", "CK_EXPORT_ERROR")
	}

	incidents, err := h.service.ListIncidents(ctx, oid)
	if err != nil {
		log.Error().Err(err).Str("org_id", oid).Msg("auditor export zip: list incidents")
		return errResp(c, http.StatusInternalServerError, "failed to build export", "CK_EXPORT_ERROR")
	}

	policies, err := h.service.ListPolicies(ctx, oid)
	if err != nil {
		log.Error().Err(err).Str("org_id", oid).Msg("auditor export zip: list policies")
		return errResp(c, http.StatusInternalServerError, "failed to build export", "CK_EXPORT_ERROR")
	}

	auditRecords, err := h.service.ListAuditRecords(ctx, oid)
	if err != nil {
		log.Error().Err(err).Str("org_id", oid).Msg("auditor export zip: list audit records")
		return errResp(c, http.StatusInternalServerError, "failed to build export", "CK_EXPORT_ERROR")
	}

	// Collect controls across all frameworks.
	frameworks, err := h.service.ListFrameworks(ctx, oid)
	if err != nil {
		log.Error().Err(err).Str("org_id", oid).Msg("auditor export zip: list frameworks")
		return errResp(c, http.StatusInternalServerError, "failed to build export", "CK_EXPORT_ERROR")
	}
	var allControls []Control
	for _, fw := range frameworks {
		controls, cErr := h.service.ListControls(ctx, oid, fw.ID)
		if cErr != nil {
			log.Warn().Err(cErr).Str("framework_id", fw.ID).Msg("auditor export zip: list controls for framework")
			continue
		}
		allControls = append(allControls, controls...)
	}

	c.Response().Header().Set("Content-Type", "application/zip")
	c.Response().Header().Set("Content-Disposition", `attachment; filename="vakt-audit-export.zip"`)
	c.Response().WriteHeader(http.StatusOK)

	zw := zip.NewWriter(c.Response().Writer)
	defer func() { _ = zw.Close() }()

	entries := []struct {
		name string
		data any
	}{
		{"controls.json", allControls},
		{"risks.json", risks},
		{"incidents.json", incidents},
		{"policies.json", policies},
		{"audit_records.json", auditRecords},
	}

	for _, entry := range entries {
		f, fErr := zw.Create(entry.name)
		if fErr != nil {
			log.Error().Err(fErr).Str("file", entry.name).Msg("auditor export zip: create zip entry")
			return nil
		}
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		if encErr := enc.Encode(entry.data); encErr != nil {
			log.Error().Err(encErr).Str("file", entry.name).Msg("auditor export zip: encode json")
			return nil
		}
	}

	return zw.Close()
}
