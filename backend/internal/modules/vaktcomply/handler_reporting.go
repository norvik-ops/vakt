package vaktcomply

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
	"github.com/matharnica/vakt/internal/shared/audit"
	"github.com/matharnica/vakt/internal/shared/pagination"
	"github.com/matharnica/vakt/internal/shared/platform/features"
	"github.com/matharnica/vakt/internal/shared/xlsxexport"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

func (h *Handler) GetBoardReport(c echo.Context) error {
	ctx := c.Request().Context()
	oid := orgID(c)

	data, err := h.service.GetBoardReportData(ctx, oid)
	if err != nil {
		log.Error().Err(err).Msg("board report: gather data")
		return errResp(c, http.StatusInternalServerError, "failed to gather board report data", "CK_BOARD_REPORT_FAILED")
	}

	pdfBytes, err := GenerateBoardReportPDF(*data)
	if err != nil {
		log.Error().Err(err).Msg("board report: generate pdf")
		return errResp(c, http.StatusInternalServerError, "failed to generate board report PDF", "CK_BOARD_REPORT_FAILED")
	}

	filename := fmt.Sprintf("vakt-board-report-%s.pdf", data.GeneratedAt.Format("2006-01-02"))
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	return c.Blob(http.StatusOK, "application/pdf", pdfBytes)
}

// GetBoardReportPDF generates a board report PDF for the given org and returns
// the raw bytes. It satisfies the scheduledreports.BoardReportProvider interface.
func (s *Service) GetBoardReportPDF(ctx context.Context, orgID string) ([]byte, error) {
	data, err := s.GetBoardReportData(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("board report data: %w", err)
	}
	return GenerateBoardReportPDF(*data)
}

// GetBoardReportData collects all data required for the Board Report PDF.
// The six independent data sources are fetched in parallel using errgroup.
func (s *Service) GetBoardReportData(ctx context.Context, orgID string) (*BoardReportData, error) {
	d := &BoardReportData{GeneratedAt: time.Now().UTC()}

	g, gctx := errgroup.WithContext(ctx)

	// 1. Org name (soft-fail — never blocks the report).
	g.Go(func() error {
		d.OrgName = fetchOrgName(gctx, s.db, orgID)
		if d.OrgName == "" {
			d.OrgName = orgID
		}
		return nil
	})

	// 2. Compliance score: weighted average of implemented/total controls across all frameworks.
	var (
		scoreMu     sync.Mutex
		totalWeight float64
		weightedSum float64
	)
	g.Go(func() error {
		scoreRows, err := s.repo.GetBoardReportComplianceScoreRows(gctx, orgID)
		if err != nil {
			// Non-fatal: leave score at 0.
			return nil //nolint:nilerr
		}
		for _, row := range scoreRows {
			if row.Total > 0 {
				score := float64(row.Implemented) / float64(row.Total) * 100
				scoreMu.Lock()
				weightedSum += score * float64(row.Total)
				totalWeight += float64(row.Total)
				scoreMu.Unlock()
			}
		}
		return nil
	})

	// 3. Previous score from score_history (most recent snapshot before today).
	g.Go(func() error {
		prevScore, err := s.repo.GetPreviousScore(gctx, orgID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			// S13-18: ErrNoRows ist erwartet (frischer Org ohne History) — alles
			// andere muss sichtbar sein.
			log.Warn().Err(err).Str("org_id", orgID).Msg("board-report: previous score lookup failed")
		}
		d.ScorePrevious = prevScore
		return nil
	})

	// 4. Open risks.
	g.Go(func() error {
		risks, _, err := s.Risk.ListRisksPaged(gctx, orgID, 0, 10_000)
		if err != nil {
			return nil //nolint:nilerr
		}
		var openRisks, criticalRisks int
		for _, r := range risks {
			if r.Status == "open" {
				openRisks++
				if r.RiskScore >= 15 {
					criticalRisks++
				}
			}
		}
		d.OpenRisks = openRisks
		d.CriticalRisks = criticalRisks
		return nil
	})

	// 5. Open & overdue CAPAs.
	g.Go(func() error {
		capas, err := s.ListCAPAs(gctx, orgID, "")
		if err != nil {
			return nil //nolint:nilerr
		}
		now := time.Now()
		var openCAPAs, overdueCAPAs int
		for _, ca := range capas {
			if ca.Status == "open" || ca.Status == "in_progress" {
				openCAPAs++
				if ca.DueDate != nil && ca.DueDate.Before(now) {
					overdueCAPAs++
				}
			}
		}
		d.OpenCAPAs = openCAPAs
		d.OverdueCAPAs = overdueCAPAs
		return nil
	})

	// 6. Incidents in the last 30 days.
	g.Go(func() error {
		since := time.Now().UTC().Add(-30 * 24 * time.Hour)
		count, err := s.repo.CountRecentIncidents(gctx, orgID, since)
		if err != nil {
			// S13-18: kein hard-fail — Report soll auch ohne diesen Counter
			// ausliefern. Aber Sichtbarkeit im Log.
			log.Warn().Err(err).Str("org_id", orgID).Msg("board-report: incidents-30d counter failed")
		} else {
			d.RecentIncidents = count
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Apply weighted score now that goroutine 2 has finished.
	if totalWeight > 0 {
		d.Score = int(weightedSum / totalWeight)
	}

	return d, nil
}

func (h *Handler) GetKPIDashboard(c echo.Context) error {
	dashboard, err := h.service.GetKPIDashboard(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get kpi dashboard")
		return errResp(c, http.StatusInternalServerError, "failed to load KPI dashboard", "CK_KPI_DASHBOARD_FAILED")
	}
	return c.JSON(http.StatusOK, dashboard)
}

// ExportKPIReportPDF handles GET /api/v1/vaktcomply/kpi-dashboard/export-pdf.
func (h *Handler) ExportKPIReportPDF(c echo.Context) error {
	ctx := c.Request().Context()
	oid := orgID(c)
	dashboard, err := h.service.GetKPIDashboard(ctx, oid)
	if err != nil {
		log.Error().Err(err).Msg("kpi report: load dashboard")
		return errResp(c, http.StatusInternalServerError, "failed to load KPI dashboard", "CK_KPI_DASHBOARD_FAILED")
	}
	orgName := fetchOrgName(ctx, h.service.db, oid)
	if orgName == "" {
		orgName = oid
	}
	now := time.Now().UTC()
	pdfBytes, err := GenerateKPIReportPDF(dashboard, orgName, now)
	if err != nil {
		log.Error().Err(err).Msg("kpi report: generate pdf")
		return errResp(c, http.StatusInternalServerError, "failed to generate KPI report PDF", "CK_KPI_PDF_FAILED")
	}
	filename := fmt.Sprintf("vakt-kpi-report-%s.pdf", now.Format("2006-01-02"))
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	return c.Blob(http.StatusOK, "application/pdf", pdfBytes)
}

func (h *Handler) GetSoA(c echo.Context) error {
	entries, err := h.service.GetSoAEntries(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get soa")
		return errResp(c, http.StatusInternalServerError, "failed to get SoA", "CK_SOA_FAILED")
	}
	return c.JSON(http.StatusOK, entries)
}

// GetSoACSV handles GET /api/v1/vaktcomply/soa.csv
func (h *Handler) GetSoACSV(c echo.Context) error {
	entries, err := h.service.GetSoAEntries(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get soa csv")
		return errResp(c, http.StatusInternalServerError, "failed to generate SoA CSV", "CK_SOA_FAILED")
	}

	filename := fmt.Sprintf("vakt-soa-%s.csv", time.Now().UTC().Format("2006-01-02"))
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	c.Response().Header().Set("Content-Type", "text/csv; charset=utf-8")

	w := csv.NewWriter(c.Response().Writer)
	_ = w.Write([]string{"Framework", "Domain", "Kontrolle", "Anwendbar", "Status", "Begründung (Anwendbar)", "Begründung (Nicht anwendbar)"})
	for _, e := range entries {
		applicable := "Nein"
		if e.Applicable {
			applicable = "Ja"
		}
		_ = w.Write([]string{
			e.FrameworkName, e.Domain, e.Title, applicable, e.Status,
			e.JustificationApplicable, e.JustificationNotApplicable,
		})
	}
	w.Flush()
	return nil
}

// UpdateSoAApplicability handles PATCH /api/v1/vaktcomply/soa/:control_id
func (h *Handler) UpdateSoAApplicability(c echo.Context) error {
	var in struct {
		Applicable                 bool   `json:"applicable"`
		JustificationApplicable    string `json:"justification_applicable"`
		JustificationNotApplicable string `json:"justification_not_applicable"`
	}
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.service.UpdateSoAApplicability(c.Request().Context(), orgID(c), c.Param("control_id"), in.Applicable, in.JustificationApplicable, in.JustificationNotApplicable); err != nil {
		log.Error().Err(err).Msg("update soa applicability")
		return errResp(c, http.StatusInternalServerError, "failed to update SoA", "CK_SOA_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ── Dedicated SoA (S68-1) ────────────────────────────────────────────────────

// InitDedicatedSoA handles POST /api/v1/vaktcomply/soa/init
// Creates version 1 with all 93 ISO 27001:2022 Annex A controls for the org.
func (h *Handler) InitDedicatedSoA(c echo.Context) error {
	if err := h.service.InitDedicatedSoA(c.Request().Context(), orgID(c)); err != nil {
		log.Error().Err(err).Msg("init dedicated soa")
		return errResp(c, http.StatusInternalServerError, "failed to initialize SoA", "CK_SOA_INIT_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// GetDedicatedSoAEntries handles GET /api/v1/vaktcomply/soa/entries
func (h *Handler) GetDedicatedSoAEntries(c echo.Context) error {
	entries, err := h.service.ListDedicatedSoAEntries(c.Request().Context(), orgID(c))
	if err != nil {
		if errors.Is(err, ErrSoANotInitialized) {
			return errResp(c, http.StatusNotFound, err.Error(), "CK_SOA_NOT_INITIALIZED")
		}
		log.Error().Err(err).Msg("get dedicated soa entries")
		return errResp(c, http.StatusInternalServerError, "failed to get SoA entries", "CK_SOA_FAILED")
	}
	return c.JSON(http.StatusOK, entries)
}

// GetDedicatedSoAEntry handles GET /api/v1/vaktcomply/soa/entries/:control_ref
func (h *Handler) GetDedicatedSoAEntry(c echo.Context) error {
	entry, err := h.service.GetDedicatedSoAEntry(c.Request().Context(), orgID(c), c.Param("control_ref"))
	if err != nil {
		if errors.Is(err, ErrSoANotInitialized) {
			return errResp(c, http.StatusNotFound, err.Error(), "CK_SOA_NOT_INITIALIZED")
		}
		return errResp(c, http.StatusNotFound, "SoA entry not found", "CK_SOA_ENTRY_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, entry)
}

// UpdateDedicatedSoAEntry handles PUT /api/v1/vaktcomply/soa/entries/:control_ref
func (h *Handler) UpdateDedicatedSoAEntry(c echo.Context) error {
	var in UpdateSoAEntryInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	if err := h.service.UpdateDedicatedSoAEntry(c.Request().Context(), orgID(c), c.Param("control_ref"), in); err != nil {
		if errors.Is(err, ErrSoANotInitialized) {
			return errResp(c, http.StatusNotFound, err.Error(), "CK_SOA_NOT_INITIALIZED")
		}
		log.Error().Err(err).Msg("update dedicated soa entry")
		return errResp(c, http.StatusBadRequest, err.Error(), "CK_SOA_UPDATE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ApproveDedicatedSoA handles POST /api/v1/vaktcomply/soa/approve
func (h *Handler) ApproveDedicatedSoA(c echo.Context) error {
	var in struct {
		Notes string `json:"notes"`
	}
	_ = c.Bind(&in)
	if err := h.service.ApproveDedicatedSoA(c.Request().Context(), orgID(c), userID(c)); err != nil {
		if errors.Is(err, ErrExclusionReasonRequired) {
			return errResp(c, http.StatusUnprocessableEntity, err.Error(), "CK_SOA_EXCLUSION_REASON_MISSING")
		}
		log.Error().Err(err).Msg("approve dedicated soa")
		return errResp(c, http.StatusInternalServerError, "failed to approve SoA", "CK_SOA_APPROVE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// GetDedicatedSoAVersions handles GET /api/v1/vaktcomply/soa/versions
func (h *Handler) GetDedicatedSoAVersions(c echo.Context) error {
	versions, err := h.service.GetDedicatedSoAVersions(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get dedicated soa versions")
		return errResp(c, http.StatusInternalServerError, "failed to get SoA versions", "CK_SOA_FAILED")
	}
	return c.JSON(http.StatusOK, versions)
}

// GetDedicatedSoASummary handles GET /api/v1/vaktcomply/soa/summary
func (h *Handler) GetDedicatedSoASummary(c echo.Context) error {
	summary, err := h.service.GetDedicatedSoASummary(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get dedicated soa summary")
		return errResp(c, http.StatusInternalServerError, "failed to get SoA summary", "CK_SOA_FAILED")
	}
	return c.JSON(http.StatusOK, summary)
}

// ExportDedicatedSoAXLSX handles GET /api/v1/vaktcomply/soa/export.xlsx
// Requires FeatureAuditPDF (same gate as PDF export).
func (h *Handler) ExportDedicatedSoAXLSX(c echo.Context) error {
	if !features.IsEnabled(c, features.FeatureAuditPDF) {
		return errResp(c, http.StatusPaymentRequired, "XLSX export requires Pro", "CK_FEATURE_REQUIRED")
	}
	ctx := c.Request().Context()
	org := orgID(c)

	entries, err := h.service.ListDedicatedSoAEntries(ctx, org)
	if err != nil {
		if errors.Is(err, ErrSoANotInitialized) {
			return errResp(c, http.StatusNotFound, err.Error(), "CK_SOA_NOT_INITIALIZED")
		}
		log.Error().Err(err).Msg("export soa xlsx: list entries")
		return errResp(c, http.StatusInternalServerError, "export failed", "CK_SOA_EXPORT_FAILED")
	}

	summary, err := h.service.GetDedicatedSoASummary(ctx, org)
	if err != nil {
		log.Error().Err(err).Msg("export soa xlsx: get summary")
		return errResp(c, http.StatusInternalServerError, "export failed", "CK_SOA_EXPORT_FAILED")
	}

	rows := make([]xlsxexport.SoARow, len(entries))
	for i, e := range entries {
		justification := e.Justification
		if !e.Applicable && e.ExclusionReason != "" {
			justification = e.ExclusionReason
		}
		owner := ""
		if e.ApprovedBy != nil {
			owner = *e.ApprovedBy
		}
		rows[i] = xlsxexport.SoARow{
			ControlRef:           e.ControlRef,
			ControlName:          e.ControlName,
			ControlGroup:         e.ControlGroup,
			Applicable:           e.Applicable,
			Justification:        justification,
			ImplementationStatus: e.ImplementationStatus,
			Owner:                owner,
			UpdatedAt:            e.UpdatedAt,
		}
	}

	var xlsSummary xlsxexport.SoASummary
	if summary != nil {
		xlsSummary = xlsxexport.SoASummary{
			ApplicableCount:   summary.ApplicableCount,
			ExcludedCount:     summary.ExcludedCount,
			ImplementedCount:  summary.ImplementedCount,
			PartialCount:      summary.PartialCount,
			PlannedCount:      summary.PlannedCount,
			NotStartedCount:   summary.NotStartedCount,
			ImplementationPct: summary.ImplementationPct,
		}
	}

	data, err := xlsxexport.RenderSoA(rows, xlsSummary)
	if err != nil {
		log.Error().Err(err).Msg("export soa xlsx: render")
		return errResp(c, http.StatusInternalServerError, "export failed", "CK_SOA_EXPORT_FAILED")
	}

	filename := fmt.Sprintf("soa-%s.xlsx", time.Now().UTC().Format("2006-01-02"))
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	return c.Blob(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
}

// ExportDedicatedSoA handles GET /api/v1/vaktcomply/soa/export
func (h *Handler) ExportDedicatedSoA(c echo.Context) error {
	format := c.QueryParam("format")
	if format == "" {
		format = "pdf"
	}
	ctx := c.Request().Context()
	org := orgID(c)

	switch format {
	case "pdf":
		data, err := h.service.ExportDedicatedSoAPDF(ctx, org)
		if err != nil {
			if errors.Is(err, ErrSoANotInitialized) {
				return errResp(c, http.StatusNotFound, err.Error(), "CK_SOA_NOT_INITIALIZED")
			}
			log.Error().Err(err).Msg("export dedicated soa pdf")
			return errResp(c, http.StatusInternalServerError, "export failed", "CK_SOA_EXPORT_FAILED")
		}
		filename := fmt.Sprintf("vakt-soa-v%s.pdf", time.Now().UTC().Format("2006-01-02"))
		c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
		return c.Blob(http.StatusOK, "application/pdf", data)

	default: // csv / xlsx
		rows, err := h.service.ExportDedicatedSoACSV(ctx, org)
		if err != nil {
			if errors.Is(err, ErrSoANotInitialized) {
				return errResp(c, http.StatusNotFound, err.Error(), "CK_SOA_NOT_INITIALIZED")
			}
			return errResp(c, http.StatusInternalServerError, "export failed", "CK_SOA_EXPORT_FAILED")
		}
		filename := fmt.Sprintf("vakt-soa-%s.csv", time.Now().UTC().Format("2006-01-02"))
		c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
		c.Response().Header().Set("Content-Type", "text/csv; charset=utf-8")
		w := csv.NewWriter(c.Response().Writer)
		for _, row := range rows {
			_ = w.Write(row)
		}
		w.Flush()
		return nil
	}
}

func (h *Handler) GetPolicy(c echo.Context) error {
	id := c.Param("id")
	policy, err := h.service.GetPolicy(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusNotFound, "policy not found", "CK_POLICY_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, policy)
}

// UpdatePolicy handles PATCH /api/v1/vaktcomply/policies/:id.
func (h *Handler) UpdatePolicy(c echo.Context) error {
	id := c.Param("id")
	var in UpdatePolicyInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	policy, err := h.service.UpdatePolicy(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		log.Error().Err(err).Msg("update policy")
		return errResp(c, http.StatusInternalServerError, "failed to update policy", "CK_UPDATE_POLICY_FAILED")
	}
	return c.JSON(http.StatusOK, policy)
}

// ListPolicies handles GET /api/v1/vaktcomply/policies.
func (h *Handler) ListPolicies(c echo.Context) error {
	offset, limit, meta := pagination.FromRequest(c)
	policies, total, err := h.service.ListPoliciesPaged(c.Request().Context(), orgID(c), offset, limit)
	if err != nil {
		log.Error().Err(err).Msg("list policies")
		return errResp(c, http.StatusInternalServerError, "failed to list policies", "CK_LIST_POLICIES_FAILED")
	}
	pagination.Complete(&meta, total)
	return c.JSON(http.StatusOK, pagination.Wrap(policies, meta))
}

// CreatePolicy handles POST /api/v1/vaktcomply/policies.
func (h *Handler) CreatePolicy(c echo.Context) error {
	var in CreatePolicyInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	policy, err := h.service.CreatePolicy(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create policy")
		return errResp(c, http.StatusInternalServerError, "failed to create policy", "CK_CREATE_POLICY_FAILED")
	}
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID:        orgID(c),
		UserID:       userID(c),
		Action:       "create",
		ResourceType: "vakt-comply/policy",
		ResourceID:   policy.ID,
		ResourceName: policy.Title,
		IPAddress:    c.RealIP(),
	})
	return c.JSON(http.StatusCreated, policy)
}

// ListPolicyVersions handles GET /api/v1/vaktcomply/policies/:id/versions.
// Returns all historical version snapshots for a policy, newest first.
func (h *Handler) ListPolicyVersions(c echo.Context) error {
	policyID := c.Param("id")
	versions, err := h.service.ListPolicyVersions(c.Request().Context(), orgID(c), policyID)
	if err != nil {
		log.Error().Err(err).Str("policy_id", policyID).Msg("list policy versions")
		return errResp(c, http.StatusInternalServerError, "failed to list policy versions", "CK_LIST_POLICY_VERSIONS_FAILED")
	}
	return c.JSON(http.StatusOK, versions)
}

// GetPolicyVersion handles GET /api/v1/vaktcomply/policies/:id/versions/:v.
// Returns a single historical version snapshot by version number.
func (h *Handler) GetPolicyVersion(c echo.Context) error {
	policyID := c.Param("id")
	vStr := c.Param("v")
	vNum, err := strconv.Atoi(vStr)
	if err != nil || vNum < 1 {
		return errResp(c, http.StatusBadRequest, "invalid version number", "CK_BAD_REQUEST")
	}
	pv, err := h.service.GetPolicyVersion(c.Request().Context(), orgID(c), policyID, vNum)
	if err != nil {
		return errResp(c, http.StatusNotFound, "policy version not found", "CK_POLICY_VERSION_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, pv)
}

// ListPolicyTemplates handles GET /api/v1/vaktcomply/policy-templates.
func (h *Handler) ListPolicyTemplates(c echo.Context) error {
	return c.JSON(http.StatusOK, BuiltinPolicyTemplates())
}

// CreatePolicyFromTemplate handles POST /api/v1/vaktcomply/policy-templates/:id/apply.
// Creates a new policy using the template content as description.
func (h *Handler) CreatePolicyFromTemplate(c echo.Context) error {
	orgID := orgID(c)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	templateID := c.Param("id")
	templates := BuiltinPolicyTemplates()
	var found *PolicyTemplate
	for i := range templates {
		if templates[i].ID == templateID {
			found = &templates[i]
			break
		}
	}
	if found == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "template not found"})
	}

	in := CreatePolicyInput{
		Title:       found.Title,
		Category:    found.Category,
		Description: found.Content,
		Version:     "1.0",
	}
	policy, err := h.service.CreatePolicy(c.Request().Context(), orgID, in)
	if err != nil {
		log.Error().Err(err).Msg("CreatePolicyFromTemplate: create policy failed")
		return errResp(c, http.StatusInternalServerError, "failed to create policy from template", "CK_CREATE_POLICY_FAILED")
	}
	return c.JSON(http.StatusCreated, policy)
}

// GeneratePolicyDraft handles POST /api/v1/vaktcomply/policies/generate-draft.
// Generates an AI-written policy draft in German using the configured AI provider.
func (h *Handler) GeneratePolicyDraft(c echo.Context) error {
	var in GeneratePolicyDraftInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_INVALID_BODY")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	draft, err := h.service.GeneratePolicyDraft(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("generate policy draft")
		if errors.Is(err, ErrNotConfigured) {
			return errResp(c, http.StatusServiceUnavailable, err.Error(), "CK_AI_NOT_CONFIGURED")
		}
		return errResp(c, http.StatusInternalServerError, "AI generation failed", "CK_AI_FAILED")
	}
	// EU AI Act Art. 52: users must be informed when content is AI-generated.
	return c.JSON(http.StatusOK, map[string]any{
		"draft":         draft,
		"ai_generated":  true,
		"ai_disclaimer": "KI-generierter Entwurf gemäß EU-KI-Verordnung Art. 52 — bitte von einem qualifizierten Experten prüfen lassen.",
	})
}

type PolicyAcceptanceHandlerConfig struct {
	SMTPHost    string
	SMTPPort    string
	SMTPUser    string
	SMTPPass    string
	SMTPFrom    string
	FrontendURL string
}

// paCfg holds handler-level config for policy acceptance.
// It is set via WithPolicyAcceptanceConfig after construction.
var _ = (*Handler)(nil)

// WithPolicyAcceptanceConfig attaches SMTP and frontend URL config to the handler.
func (h *Handler) WithPolicyAcceptanceConfig(cfg PolicyAcceptanceHandlerConfig) {
	h.paCfg = cfg
}

// CreateAcceptanceCampaign handles POST /vaktcomply/policies/:id/acceptance-campaigns.
// Creates an acceptance campaign and fires off invitation emails.
func (h *Handler) CreateAcceptanceCampaign(c echo.Context) error {
	policyID := c.Param("id")
	if policyID == "" {
		return errResp(c, http.StatusBadRequest, "policy ID required", "CK_BAD_REQUEST")
	}

	var in CreateCampaignInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	in.PolicyID = policyID

	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusBadRequest, err.Error(), "CK_VALIDATION_ERROR")
	}

	smtpCfg := PolicyAcceptanceSMTPConfig{
		Host: h.paCfg.SMTPHost,
		Port: h.paCfg.SMTPPort,
		User: h.paCfg.SMTPUser,
		Pass: h.paCfg.SMTPPass,
		From: h.paCfg.SMTPFrom,
	}

	campaign, err := h.service.Policy.CreateAcceptanceCampaign(
		c.Request().Context(),
		orgID(c), userID(c),
		in,
		smtpCfg,
		h.paCfg.FrontendURL,
	)
	if err != nil {
		log.Error().Err(err).Msg("create acceptance campaign")
		return errResp(c, http.StatusInternalServerError, "failed to create campaign", "CK_CAMPAIGN_CREATE_FAILED")
	}
	return c.JSON(http.StatusCreated, campaign)
}

// ListAcceptanceCampaigns handles GET /vaktcomply/policies/:id/acceptance-campaigns.
func (h *Handler) ListAcceptanceCampaigns(c echo.Context) error {
	policyID := c.Param("id")
	if policyID == "" {
		return errResp(c, http.StatusBadRequest, "policy ID required", "CK_BAD_REQUEST")
	}

	campaigns, err := h.service.Policy.ListCampaigns(c.Request().Context(), orgID(c), policyID)
	if err != nil {
		log.Error().Err(err).Msg("list acceptance campaigns")
		return errResp(c, http.StatusInternalServerError, "failed to list campaigns", "CK_CAMPAIGN_LIST_FAILED")
	}
	if campaigns == nil {
		campaigns = []PolicyAcceptanceCampaign{}
	}
	return c.JSON(http.StatusOK, campaigns)
}

// GetCampaignStats handles GET /vaktcomply/policies/acceptance-campaigns/:cid/stats.
func (h *Handler) GetCampaignStats(c echo.Context) error {
	cid := c.Param("cid")
	if cid == "" {
		return errResp(c, http.StatusBadRequest, "campaign ID required", "CK_BAD_REQUEST")
	}

	stats, err := h.service.Policy.GetCampaignStats(c.Request().Context(), cid)
	if err != nil {
		log.Error().Err(err).Msg("get campaign stats")
		return errResp(c, http.StatusInternalServerError, "failed to get stats", "CK_CAMPAIGN_STATS_FAILED")
	}
	return c.JSON(http.StatusOK, stats)
}

// ListCampaignRequests handles GET /vaktcomply/policies/acceptance-campaigns/:cid/requests.
func (h *Handler) ListCampaignRequests(c echo.Context) error {
	cid := c.Param("cid")
	if cid == "" {
		return errResp(c, http.StatusBadRequest, "campaign ID required", "CK_BAD_REQUEST")
	}

	requests, err := h.service.Policy.ListCampaignRequests(c.Request().Context(), cid)
	if err != nil {
		log.Error().Err(err).Msg("list campaign requests")
		return errResp(c, http.StatusInternalServerError, "failed to list requests", "CK_CAMPAIGN_REQUESTS_FAILED")
	}
	if requests == nil {
		requests = []PolicyAcceptanceRequest{}
	}
	return c.JSON(http.StatusOK, requests)
}

// GetAcceptanceInfo handles GET /policy-accept/:token — public, no auth.
// Returns policy/org/message info for the accept page.
func (h *Handler) GetAcceptanceInfo(c echo.Context) error {
	token := c.Param("token")
	if token == "" {
		return errResp(c, http.StatusBadRequest, "token required", "CK_BAD_REQUEST")
	}

	info, err := h.service.Policy.GetAcceptanceRequestInfo(c.Request().Context(), token)
	if err != nil {
		return errResp(c, http.StatusNotFound, "token not found or expired", "CK_TOKEN_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, info)
}

// AcceptPolicy handles POST /policy-accept/:token — public, no auth.
// Records the acceptance timestamp and IP; creates compliance evidence.
func (h *Handler) AcceptPolicy(c echo.Context) error {
	token := c.Param("token")
	if token == "" {
		return errResp(c, http.StatusBadRequest, "token required", "CK_BAD_REQUEST")
	}

	ip := c.RealIP()
	if err := h.service.Policy.AcceptPolicy(c.Request().Context(), token, ip); err != nil {
		log.Warn().Err(err).Str("ip", ip).Msg("accept policy")
		return errResp(c, http.StatusNotFound, "token not found or expired", "CK_TOKEN_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "accepted"})
}
