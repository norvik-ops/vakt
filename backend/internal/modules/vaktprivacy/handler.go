// Package vaktprivacy provides HTTP handlers for the DSGVO documentation module.
package vaktprivacy

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/audit"
	"github.com/matharnica/vakt/internal/shared/pagination"
	"github.com/matharnica/vakt/internal/shared/safego"
)

// AlertFunc is a callback used to fire external alert events without importing
// the alerting package directly into this module (module isolation).
type AlertFunc func(ctx context.Context, orgID, event string, payload map[string]any)

// Handler handles HTTP requests for PrivacyOps.
type Handler struct {
	service   *Service
	validate  *validator.Validate
	alertFunc AlertFunc
	db        *pgxpool.Pool
	tia       *TIAService
}

// NewHandler creates a new PrivacyOps handler.
func NewHandler(service *Service) *Handler {
	return &Handler{
		service:  service,
		validate: validator.New(),
	}
}

// WithDB attaches a DB pool used for audit logging.
func (h *Handler) WithDB(db *pgxpool.Pool) *Handler {
	h.db = db
	return h
}

// WithAlerting sets an optional alerting callback invoked on key events.
func (h *Handler) WithAlerting(fn AlertFunc) *Handler {
	h.alertFunc = fn
	return h
}

// WithTIA injects the TIA service (S69-6).
func (h *Handler) WithTIA(tia *TIAService) *Handler {
	h.tia = tia
	return h
}

func orgID(c echo.Context) string {
	v, _ := c.Get("org_id").(string)
	return v
}

func errResp(c echo.Context, code int, msg, errCode string) error {
	return c.JSON(code, map[string]string{"error": msg, "code": errCode})
}

// --- VVT ---

// ListVVT handles GET /api/v1/vaktprivacy/vvt.
func (h *Handler) ListVVT(c echo.Context) error {
	offset, limit, meta := pagination.FromRequest(c)
	entries, total, err := h.service.ListVVTPaged(c.Request().Context(), orgID(c), offset, limit)
	if err != nil {
		log.Error().Err(err).Msg("list vvt")
		return errResp(c, http.StatusInternalServerError, "failed to list VVT entries", "PO_LIST_VVT_FAILED")
	}
	pagination.Complete(&meta, total)
	return c.JSON(http.StatusOK, pagination.Wrap(entries, meta))
}

// GetVVT handles GET /api/v1/vaktprivacy/vvt/:id.
func (h *Handler) GetVVT(c echo.Context) error {
	entry, err := h.service.GetVVT(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		return errResp(c, http.StatusNotFound, "VVT entry not found", "PO_VVT_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, entry)
}

// CreateVVT handles POST /api/v1/vaktprivacy/vvt.
func (h *Handler) CreateVVT(c echo.Context) error {
	var in CreateVVTInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "PO_INVALID_BODY")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	entry, err := h.service.CreateVVT(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create vvt")
		return errResp(c, http.StatusInternalServerError, "failed to create VVT entry", "PO_CREATE_VVT_FAILED")
	}
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID:        orgID(c),
		UserID:       func() string { v, _ := c.Get("user_id").(string); return v }(),
		Action:       "create",
		ResourceType: "vakt-privacy/vvt",
		ResourceID:   entry.ID,
		ResourceName: entry.Name,
		IPAddress:    c.RealIP(),
	})
	return c.JSON(http.StatusCreated, entry)
}

// UpdateVVT handles PUT /api/v1/vaktprivacy/vvt/:id.
func (h *Handler) UpdateVVT(c echo.Context) error {
	var in UpdateVVTInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "PO_INVALID_BODY")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	entry, err := h.service.UpdateVVT(c.Request().Context(), orgID(c), c.Param("id"), in)
	if err != nil {
		log.Error().Err(err).Msg("update vvt")
		return errResp(c, http.StatusInternalServerError, "failed to update VVT entry", "PO_UPDATE_VVT_FAILED")
	}
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID:        orgID(c),
		UserID:       func() string { v, _ := c.Get("user_id").(string); return v }(),
		Action:       "update",
		ResourceType: "vakt-privacy/vvt",
		ResourceID:   entry.ID,
		ResourceName: entry.Name,
		IPAddress:    c.RealIP(),
	})
	return c.JSON(http.StatusOK, entry)
}

// DeleteVVT handles DELETE /api/v1/vaktprivacy/vvt/:id.
func (h *Handler) DeleteVVT(c echo.Context) error {
	id := c.Param("id")
	if err := h.service.DeleteVVT(c.Request().Context(), orgID(c), id); err != nil {
		log.Error().Err(err).Msg("delete vvt")
		return errResp(c, http.StatusInternalServerError, "failed to delete VVT entry", "PO_DELETE_VVT_FAILED")
	}
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID:        orgID(c),
		UserID:       func() string { v, _ := c.Get("user_id").(string); return v }(),
		Action:       "delete",
		ResourceType: "vakt-privacy/vvt",
		ResourceID:   id,
		IPAddress:    c.RealIP(),
	})
	return c.NoContent(http.StatusNoContent)
}

// --- DPIA ---

// ListDPIAs handles GET /api/v1/vaktprivacy/dpias.
func (h *Handler) ListDPIAs(c echo.Context) error {
	dpias, err := h.service.ListDPIAs(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list dpias")
		return errResp(c, http.StatusInternalServerError, "failed to list DPIAs", "PO_LIST_DPIAS_FAILED")
	}
	return c.JSON(http.StatusOK, dpias)
}

// GetDPIA handles GET /api/v1/vaktprivacy/dpias/:id.
func (h *Handler) GetDPIA(c echo.Context) error {
	dpia, err := h.service.GetDPIA(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		return errResp(c, http.StatusNotFound, "DPIA not found", "PO_DPIA_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, dpia)
}

// CreateDPIA handles POST /api/v1/vaktprivacy/dpias.
func (h *Handler) CreateDPIA(c echo.Context) error {
	var in CreateDPIAInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "PO_INVALID_BODY")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	dpia, err := h.service.CreateDPIA(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create dpia")
		return errResp(c, http.StatusInternalServerError, "failed to create DPIA", "PO_CREATE_DPIA_FAILED")
	}
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID:        orgID(c),
		UserID:       func() string { v, _ := c.Get("user_id").(string); return v }(),
		Action:       "create",
		ResourceType: "vakt-privacy/dpia",
		ResourceID:   dpia.ID,
		ResourceName: dpia.Title,
		IPAddress:    c.RealIP(),
	})
	return c.JSON(http.StatusCreated, dpia)
}

// UpdateDPIA handles PUT /api/v1/vaktprivacy/dpias/:id.
func (h *Handler) UpdateDPIA(c echo.Context) error {
	var in UpdateDPIAInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "PO_INVALID_BODY")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	dpia, err := h.service.UpdateDPIA(c.Request().Context(), orgID(c), c.Param("id"), in)
	if err != nil {
		log.Error().Err(err).Msg("update dpia")
		return errResp(c, http.StatusInternalServerError, "failed to update DPIA", "PO_UPDATE_DPIA_FAILED")
	}
	return c.JSON(http.StatusOK, dpia)
}

// ApproveDPIA handles POST /api/v1/vaktprivacy/dpias/:id/approve.
func (h *Handler) ApproveDPIA(c echo.Context) error {
	uid, _ := c.Get("user_id").(string)
	dpia, err := h.service.ApproveDPIA(c.Request().Context(), orgID(c), c.Param("id"), uid)
	if err != nil {
		log.Error().Err(err).Msg("approve dpia")
		return errResp(c, http.StatusInternalServerError, "failed to approve DPIA", "PO_APPROVE_DPIA_FAILED")
	}
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID:        orgID(c),
		UserID:       uid,
		Action:       "approve",
		ResourceType: "vakt-privacy/dpia",
		ResourceID:   dpia.ID,
		ResourceName: dpia.Title,
		IPAddress:    c.RealIP(),
	})
	return c.JSON(http.StatusOK, dpia)
}

// DeleteDPIA handles DELETE /api/v1/vaktprivacy/dpias/:id.
func (h *Handler) DeleteDPIA(c echo.Context) error {
	if err := h.service.DeleteDPIA(c.Request().Context(), orgID(c), c.Param("id")); err != nil {
		log.Error().Err(err).Msg("delete dpia")
		return errResp(c, http.StatusInternalServerError, "failed to delete DPIA", "PO_DELETE_DPIA_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// --- AVV ---

// ListAVVs handles GET /api/v1/vaktprivacy/avvs.
func (h *Handler) ListAVVs(c echo.Context) error {
	avvs, err := h.service.ListAVVs(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list avvs")
		return errResp(c, http.StatusInternalServerError, "failed to list AVVs", "PO_LIST_AVVS_FAILED")
	}
	return c.JSON(http.StatusOK, avvs)
}

// GetAVV handles GET /api/v1/vaktprivacy/avvs/:id.
func (h *Handler) GetAVV(c echo.Context) error {
	avv, err := h.service.GetAVV(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		return errResp(c, http.StatusNotFound, "AVV not found", "PO_AVV_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, avv)
}

// CreateAVV handles POST /api/v1/vaktprivacy/avvs.
func (h *Handler) CreateAVV(c echo.Context) error {
	var in CreateAVVInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "PO_INVALID_BODY")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	avv, err := h.service.CreateAVV(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create avv")
		return errResp(c, http.StatusInternalServerError, "failed to create AVV", "PO_CREATE_AVV_FAILED")
	}
	return c.JSON(http.StatusCreated, avv)
}

// UpdateAVV handles PUT /api/v1/vaktprivacy/avvs/:id.
func (h *Handler) UpdateAVV(c echo.Context) error {
	var in UpdateAVVInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "PO_INVALID_BODY")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	avv, err := h.service.UpdateAVV(c.Request().Context(), orgID(c), c.Param("id"), in)
	if err != nil {
		log.Error().Err(err).Msg("update avv")
		return errResp(c, http.StatusInternalServerError, "failed to update AVV", "PO_UPDATE_AVV_FAILED")
	}
	return c.JSON(http.StatusOK, avv)
}

// DeleteAVV handles DELETE /api/v1/vaktprivacy/avvs/:id.
func (h *Handler) DeleteAVV(c echo.Context) error {
	if err := h.service.DeleteAVV(c.Request().Context(), orgID(c), c.Param("id")); err != nil {
		log.Error().Err(err).Msg("delete avv")
		return errResp(c, http.StatusInternalServerError, "failed to delete AVV", "PO_DELETE_AVV_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// --- Breach ---

// ListBreaches handles GET /api/v1/vaktprivacy/breaches.
func (h *Handler) ListBreaches(c echo.Context) error {
	offset, limit, meta := pagination.FromRequest(c)
	breaches, total, err := h.service.ListBreachesPaged(c.Request().Context(), orgID(c), offset, limit)
	if err != nil {
		log.Error().Err(err).Msg("list breaches")
		return errResp(c, http.StatusInternalServerError, "failed to list breaches", "PO_LIST_BREACHES_FAILED")
	}
	pagination.Complete(&meta, total)
	return c.JSON(http.StatusOK, pagination.Wrap(breaches, meta))
}

// GetBreach handles GET /api/v1/vaktprivacy/breaches/:id.
func (h *Handler) GetBreach(c echo.Context) error {
	breach, err := h.service.GetBreach(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		return errResp(c, http.StatusNotFound, "breach not found", "PO_BREACH_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, breach)
}

// CreateBreach handles POST /api/v1/vaktprivacy/breaches.
func (h *Handler) CreateBreach(c echo.Context) error {
	var in CreateBreachInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "PO_INVALID_BODY")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	breach, err := h.service.CreateBreach(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create breach")
		return errResp(c, http.StatusInternalServerError, "failed to create breach record", "PO_CREATE_BREACH_FAILED")
	}
	// Fire external alert — non-blocking, best-effort.
	// Capture orgID before spawning the goroutine to avoid reading from a
	// potentially recycled echo.Context after the handler returns.
	if h.alertFunc != nil {
		capturedOrgID := orgID(c)
		capturedBreachID := breach.ID
		capturedTitle := breach.Title
		// ADR-0018: safego.Run + WithoutCancel — Alert darf nicht vom Client-
		// Disconnect abgebrochen werden, aber respektiert lifecycle-Shutdown.
		safego.Run(c.Request().Context(), "vaktprivacy.breach.alert", func(parent context.Context) error {
			alertCtx, alertCancel := context.WithTimeout(context.WithoutCancel(parent), 30*time.Second)
			defer alertCancel()
			h.alertFunc(alertCtx, capturedOrgID, "breach.created", map[string]any{
				"breach_id": capturedBreachID,
				"title":     capturedTitle,
			})
			return nil
		})
	}
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID:        orgID(c),
		UserID:       func() string { v, _ := c.Get("user_id").(string); return v }(),
		Action:       "create",
		ResourceType: "vakt-privacy/breach",
		ResourceID:   breach.ID,
		ResourceName: breach.Title,
		IPAddress:    c.RealIP(),
	})
	return c.JSON(http.StatusCreated, breach)
}

// UpdateBreach handles PUT /api/v1/vaktprivacy/breaches/:id.
func (h *Handler) UpdateBreach(c echo.Context) error {
	var in UpdateBreachInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "PO_INVALID_BODY")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	breach, err := h.service.UpdateBreach(c.Request().Context(), orgID(c), c.Param("id"), in)
	if err != nil {
		log.Error().Err(err).Msg("update breach")
		return errResp(c, http.StatusInternalServerError, "failed to update breach", "PO_UPDATE_BREACH_FAILED")
	}
	return c.JSON(http.StatusOK, breach)
}

// DeleteBreach handles DELETE /api/v1/vaktprivacy/breaches/:id.
func (h *Handler) DeleteBreach(c echo.Context) error {
	if err := h.service.DeleteBreach(c.Request().Context(), orgID(c), c.Param("id")); err != nil {
		log.Error().Err(err).Msg("delete breach")
		return errResp(c, http.StatusInternalServerError, "failed to delete breach", "PO_DELETE_BREACH_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// MarkAuthorityNotified handles POST /api/v1/vaktprivacy/breaches/:id/notify-authority.
// Stamps authority_notified_at to the current time, fulfilling the documentation
// requirement that the supervisory-authority notification under Art. 33 DSGVO was made.
func (h *Handler) MarkAuthorityNotified(c echo.Context) error {
	id := c.Param("id")
	if err := h.service.MarkAuthorityNotified(c.Request().Context(), id, orgID(c)); err != nil {
		log.Error().Err(err).Msg("mark authority notified")
		return errResp(c, http.StatusInternalServerError, "failed to update breach", "PO_UPDATE_BREACH_FAILED")
	}
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID:        orgID(c),
		UserID:       func() string { v, _ := c.Get("user_id").(string); return v }(),
		Action:       "update",
		ResourceType: "vakt-privacy/breach",
		ResourceID:   id,
		Details:      map[string]string{"event": "authority_notified"},
		IPAddress:    c.RealIP(),
	})
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// ExportVVT handles GET /api/v1/vaktprivacy/vvt/export.
// Streams a DSGVO Art. 30-compliant PDF containing all active VVT entries.
func (h *Handler) ExportVVT(c echo.Context) error {
	ctx := c.Request().Context()
	filename := fmt.Sprintf("vvt-export-%s.pdf", time.Now().Format("2006-01-02"))
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Response().Header().Set("Content-Type", "application/pdf")
	c.Response().WriteHeader(http.StatusOK)

	if err := h.service.GenerateVVTPDF(ctx, orgID(c), c.Response().Writer); err != nil {
		log.Error().Err(err).Msg("vvt pdf export failed")
		return nil
	}
	return nil
}

// ExportDPIA handles GET /api/v1/vaktprivacy/dpias/export.
// Streams a DSGVO Art. 35-compliant PDF containing all DPIA entries.
func (h *Handler) ExportDPIA(c echo.Context) error {
	ctx := c.Request().Context()
	filename := fmt.Sprintf("dpia-export-%s.pdf", time.Now().Format("2006-01-02"))
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Response().Header().Set("Content-Type", "application/pdf")
	c.Response().WriteHeader(http.StatusOK)

	if err := h.service.GenerateDPIAPDF(ctx, orgID(c), c.Response().Writer); err != nil {
		log.Error().Err(err).Msg("dpia pdf export failed")
		return nil
	}
	return nil
}

// --- DSR ---

// ListDSRs handles GET /api/v1/vaktprivacy/dsr.
// Cursor mode (preferred): ?cursor=<opaque>&limit=25
// Offset mode (deprecated): ?page=1&limit=25 — sends Deprecation header
func (h *Handler) ListDSRs(c echo.Context) error {
	if c.QueryParam("page") == "" {
		cp := pagination.CursorFromRequest(c)
		cursorID, cursorTS := pagination.DecodeCursor(cp.Cursor)
		rows, err := h.service.ListDSRsCursor(c.Request().Context(), orgID(c), cursorID, cursorTS, cp.Limit)
		if err != nil {
			log.Error().Err(err).Msg("list dsrs cursor")
			return errResp(c, http.StatusInternalServerError, "failed to list DSRs", "PO_LIST_DSRS_FAILED")
		}
		resp := pagination.WrapCursor(rows, cp, func(d DSR) string {
			return pagination.EncodeCursor(d.ID, d.CreatedAt)
		})
		return c.JSON(http.StatusOK, resp)
	}
	c.Response().Header().Set("Deprecation", "true")
	c.Response().Header().Set("Sunset", "2027-01-01")
	dsrs, err := h.service.ListDSRs(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list dsrs")
		return errResp(c, http.StatusInternalServerError, "failed to list DSRs", "PO_LIST_DSRS_FAILED")
	}
	return c.JSON(http.StatusOK, dsrs)
}

// CreateDSR handles POST /api/v1/vaktprivacy/dsr.
// Validates input, persists the request with an auto-computed 30-day due_date
// (Art. 12 DSGVO), and triggers a warning-level notification to the DPO.
func (h *Handler) CreateDSR(c echo.Context) error {
	var in CreateDSRInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "PO_INVALID_BODY")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	dsr, err := h.service.CreateDSR(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create dsr")
		return errResp(c, http.StatusInternalServerError, "failed to create DSR", "PO_CREATE_DSR_FAILED")
	}
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID:        orgID(c),
		UserID:       func() string { v, _ := c.Get("user_id").(string); return v }(),
		Action:       "create",
		ResourceType: "vakt-privacy/dsr",
		ResourceID:   dsr.ID,
		ResourceName: dsr.RequesterName,
		IPAddress:    c.RealIP(),
	})
	return c.JSON(http.StatusCreated, dsr)
}

// UpdateDSR handles PUT /api/v1/vaktprivacy/dsr/:id.
// Accepts a new status and optional notes; stamps completed_at when status
// is "completed" or "rejected" to record the response timeline against due_date.
func (h *Handler) UpdateDSR(c echo.Context) error {
	var in UpdateDSRInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "PO_INVALID_BODY")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	dsr, err := h.service.UpdateDSR(c.Request().Context(), orgID(c), c.Param("id"), in)
	if err != nil {
		log.Error().Err(err).Msg("update dsr")
		return errResp(c, http.StatusInternalServerError, "failed to update DSR", "PO_UPDATE_DSR_FAILED")
	}
	return c.JSON(http.StatusOK, dsr)
}

// DeleteDSR handles DELETE /api/v1/vaktprivacy/dsr/:id.
// Permanently removes the DSR; should only be used for erroneous duplicates.
func (h *Handler) DeleteDSR(c echo.Context) error {
	if err := h.service.DeleteDSR(c.Request().Context(), orgID(c), c.Param("id")); err != nil {
		log.Error().Err(err).Msg("delete dsr")
		return errResp(c, http.StatusInternalServerError, "failed to delete DSR", "PO_DELETE_DSR_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ExportDSRsCSV handles GET /api/v1/vaktprivacy/dsrs/export.csv.
// Streams all DSRs for the authenticated organisation as a CSV file.
func (h *Handler) ExportDSRsCSV(c echo.Context) error {
	ctx := c.Request().Context()
	dsrs, err := h.service.ListDSRs(ctx, orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("export dsrs csv")
		return errResp(c, http.StatusInternalServerError, "failed to export DSRs", "PO_EXPORT_DSR_FAILED")
	}

	c.Response().Header().Set("Content-Type", "text/csv")
	c.Response().Header().Set("Content-Disposition", `attachment; filename="dsr-export.csv"`)
	c.Response().WriteHeader(http.StatusOK)

	w := csv.NewWriter(c.Response().Writer)
	_ = w.Write([]string{"id", "type", "requester_name", "requester_email", "status", "received_at", "due_date", "completed_at"})
	for _, d := range dsrs {
		dueDate := ""
		if d.DueDate != nil {
			dueDate = *d.DueDate
		}
		completedAt := ""
		if d.CompletedAt != nil {
			completedAt = d.CompletedAt.Format(time.RFC3339)
		}
		_ = w.Write([]string{
			d.ID,
			d.Type,
			d.RequesterName,
			d.RequesterEmail,
			d.Status,
			d.ReceivedAt.Format(time.RFC3339),
			dueDate,
			completedAt,
		})
	}
	w.Flush()
	return nil
}

// ── DSR enhanced endpoints (S68-2) ─────────────────────────────────────────

// ResolveDSR handles POST /api/v1/vaktprivacy/dsr/:id/resolve
func (h *Handler) ResolveDSR(c echo.Context) error {
	var in ResolveDSRInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "PO_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	uid, _ := c.Get("user_id").(string)
	dsr, err := h.service.ResolveDSR(c.Request().Context(), orgID(c), c.Param("id"), uid, in)
	if err != nil {
		log.Error().Err(err).Msg("resolve dsr")
		return errResp(c, http.StatusBadRequest, err.Error(), "PO_RESOLVE_DSR_FAILED")
	}
	return c.JSON(http.StatusOK, dsr)
}

// AssignDSR handles PATCH /api/v1/vaktprivacy/dsr/:id/assign
func (h *Handler) AssignDSR(c echo.Context) error {
	var in struct {
		AssignedTo string `json:"assigned_to"`
	}
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "PO_BAD_REQUEST")
	}
	_, err := h.service.db.Exec(c.Request().Context(),
		`UPDATE po_dsr SET assigned_to = NULLIF($1,''), updated_at = NOW() WHERE org_id = $2 AND id = $3`,
		in.AssignedTo, orgID(c), c.Param("id"),
	)
	if err != nil {
		log.Error().Err(err).Msg("assign dsr")
		return errResp(c, http.StatusInternalServerError, "failed to assign DSR", "PO_ASSIGN_DSR_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// GetDSRSummary handles GET /api/v1/vaktprivacy/dsr/summary
func (h *Handler) GetDSRSummary(c echo.Context) error {
	summary, err := h.service.GetDSRSummary(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get dsr summary")
		return errResp(c, http.StatusInternalServerError, "failed to get DSR summary", "PO_DSR_SUMMARY_FAILED")
	}
	return c.JSON(http.StatusOK, summary)
}

// ExportDSRLog handles GET /api/v1/vaktprivacy/dsr/export
func (h *Handler) ExportDSRLog(c echo.Context) error {
	data, err := h.service.ExportDSRLogPDF(c.Request().Context(), orgID(c), 365)
	if err != nil {
		log.Error().Err(err).Msg("export dsr log pdf")
		return errResp(c, http.StatusInternalServerError, "export failed", "PO_DSR_EXPORT_FAILED")
	}
	filename := fmt.Sprintf("vakt-dsr-log-%s.pdf", time.Now().UTC().Format("2006-01-02"))
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	return c.Blob(http.StatusOK, "application/pdf", data)
}

// ── Retention handlers (S68-5) ─────────────────────────────────────────────

// GetRetentionInfo handles GET /api/v1/vaktprivacy/processing-activities/:id/retention
func (h *Handler) GetRetentionInfo(c echo.Context) error {
	info, err := h.service.GetRetentionInfo(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		log.Error().Err(err).Msg("get retention info")
		return errResp(c, http.StatusNotFound, "processing activity not found", "PO_RETENTION_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, info)
}

// UpdateRetentionInfo handles PUT /api/v1/vaktprivacy/processing-activities/:id/retention
func (h *Handler) UpdateRetentionInfo(c echo.Context) error {
	var in UpdateRetentionInfoInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "PO_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	if err := h.service.UpdateRetentionInfo(c.Request().Context(), orgID(c), c.Param("id"), in); err != nil {
		log.Error().Err(err).Msg("update retention info")
		return errResp(c, http.StatusInternalServerError, "failed to update retention info", "PO_RETENTION_UPDATE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// GetRetentionSummary handles GET /api/v1/vaktprivacy/retention/summary
func (h *Handler) GetRetentionSummary(c echo.Context) error {
	summary, err := h.service.GetRetentionSummary(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get retention summary")
		return errResp(c, http.StatusInternalServerError, "failed to get retention summary", "PO_RETENTION_SUMMARY_FAILED")
	}
	return c.JSON(http.StatusOK, summary)
}

// ListDeletionReminders handles GET /api/v1/vaktprivacy/deletion-reminders
func (h *Handler) ListDeletionReminders(c echo.Context) error {
	reminders, err := h.service.ListDeletionReminders(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list deletion reminders")
		return errResp(c, http.StatusInternalServerError, "failed to list deletion reminders", "PO_DELETION_REMINDERS_FAILED")
	}
	return c.JSON(http.StatusOK, reminders)
}

// CreateDeletionReminder handles POST /api/v1/vaktprivacy/deletion-reminders
func (h *Handler) CreateDeletionReminder(c echo.Context) error {
	var in CreateDeletionReminderInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "PO_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	reminder, err := h.service.CreateDeletionReminder(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create deletion reminder")
		return errResp(c, http.StatusInternalServerError, "failed to create deletion reminder", "PO_DELETION_REMINDER_CREATE_FAILED")
	}
	return c.JSON(http.StatusCreated, reminder)
}

// CompleteDeletionReminder handles PATCH /api/v1/vaktprivacy/deletion-reminders/:id/complete
func (h *Handler) CompleteDeletionReminder(c echo.Context) error {
	var in CompleteDeletionReminderInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "PO_BAD_REQUEST")
	}
	userUID, _ := c.Get("user_id").(string)
	if err := h.service.CompleteDeletionReminder(c.Request().Context(), orgID(c), c.Param("id"), userUID, in); err != nil {
		log.Error().Err(err).Msg("complete deletion reminder")
		return errResp(c, http.StatusInternalServerError, "failed to complete reminder", "PO_DELETION_REMINDER_COMPLETE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ListRetentionTemplates handles GET /api/v1/vaktprivacy/retention-templates
func (h *Handler) ListRetentionTemplates(c echo.Context) error {
	templates, err := h.service.ListRetentionTemplates(c.Request().Context())
	if err != nil {
		log.Error().Err(err).Msg("list retention templates")
		return errResp(c, http.StatusInternalServerError, "failed to list retention templates", "PO_RETENTION_TEMPLATES_FAILED")
	}
	return c.JSON(http.StatusOK, templates)
}

// ExportBreachNotification handles GET /api/v1/vaktprivacy/breaches/:id/notification-pdf.
// Streams the Art. 33 DSGVO authority notification letter as a PDF blob directly to
// the response writer. The Content-Disposition header triggers a browser download.
// If PDF generation fails after headers are sent, the error is logged but not surfaced
// to the client (the connection is already committed at that point).
func (h *Handler) ExportBreachNotification(c echo.Context) error {
	ctx := c.Request().Context()
	id := c.Param("id")
	filename := fmt.Sprintf("breach-notification-%s.pdf", time.Now().Format("2006-01-02"))
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Response().Header().Set("Content-Type", "application/pdf")
	c.Response().WriteHeader(http.StatusOK)

	if err := h.service.GenerateBreachNotificationPDF(ctx, orgID(c), id, c.Response().Writer); err != nil {
		log.Error().Err(err).Msg("breach notification pdf export failed")
		return nil
	}
	return nil
}

// --- DSR Portal (public, no auth) ---

// PortalGetInfo handles GET /api/v1/dsr-portal/:slug/info.
// Returns public information about the DSR portal for the given organisation slug.
func (h *Handler) PortalGetInfo(c echo.Context) error {
	info, err := h.service.GetDSRPortalInfo(c.Request().Context(), c.Param("slug"))
	if err != nil {
		return errResp(c, http.StatusNotFound, "DSR portal not found", "PO_DSR_PORTAL_NOT_FOUND")
	}
	if !info.Enabled {
		return errResp(c, http.StatusNotFound, "DSR portal is not enabled", "PO_DSR_PORTAL_DISABLED")
	}
	return c.JSON(http.StatusOK, info)
}

// PortalSubmitDSR handles POST /api/v1/dsr-portal/:slug/submit.
// Accepts an unauthenticated DSR submission from the public self-service portal.
// Returns the raw status token so the requester can track processing later.
func (h *Handler) PortalSubmitDSR(c echo.Context) error {
	var in PortalDSRInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "PO_INVALID_BODY")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}

	rawToken, err := h.service.SubmitPortalDSR(c.Request().Context(), c.Param("slug"), in, c.RealIP())
	if err != nil {
		log.Error().Err(err).Msg("portal submit dsr")
		return errResp(c, http.StatusBadRequest, "could not submit DSR", "PO_DSR_PORTAL_SUBMIT_FAILED")
	}
	return c.JSON(http.StatusCreated, map[string]string{"token": rawToken})
}

// PortalGetDSRStatus handles GET /api/v1/dsr-portal/status/:token.
// Looks up a DSR by the raw status token returned at submission time.
// Only public fields are returned — internal fields (org_id, notes, requester
// details) are stripped to prevent information disclosure to unauthenticated
// callers (H6/H7 security fix).
func (h *Handler) PortalGetDSRStatus(c echo.Context) error {
	dsr, err := h.service.GetPortalDSR(c.Request().Context(), c.Param("token"))
	if err != nil {
		return errResp(c, http.StatusNotFound, "DSR not found", "PO_DSR_NOT_FOUND")
	}
	public := DSRPublicStatus{
		ID:          dsr.ID,
		Status:      dsr.Status,
		Type:        dsr.Type,
		CreatedAt:   dsr.CreatedAt,
		UpdatedAt:   dsr.UpdatedAt,
		CompletedAt: dsr.CompletedAt,
	}
	return c.JSON(http.StatusOK, public)
}

// --- DSR Portal Settings (authenticated) ---

// GetDSRPortalSettings handles GET /api/v1/vaktprivacy/dsr-portal-settings.
func (h *Handler) GetDSRPortalSettings(c echo.Context) error {
	settings, err := h.service.GetDSRPortalSettings(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get dsr portal settings")
		return errResp(c, http.StatusInternalServerError, "failed to get DSR portal settings", "PO_DSR_SETTINGS_FAILED")
	}
	return c.JSON(http.StatusOK, settings)
}

// UpdateDSRPortalSettings handles PATCH /api/v1/vaktprivacy/dsr-portal-settings.
func (h *Handler) UpdateDSRPortalSettings(c echo.Context) error {
	var in UpdateDSRPortalSettingsInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "PO_INVALID_BODY")
	}
	if err := h.service.UpdateDSRPortalSettings(c.Request().Context(), orgID(c), in); err != nil {
		log.Error().Err(err).Msg("update dsr portal settings")
		return errResp(c, http.StatusInternalServerError, "failed to update DSR portal settings", "PO_DSR_SETTINGS_UPDATE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// --- AVV Templates & SCC ---

// ListAVVTemplates handles GET /api/v1/vaktprivacy/avv-templates.
func (h *Handler) ListAVVTemplates(c echo.Context) error {
	return c.JSON(http.StatusOK, h.service.ListAVVTemplates())
}

// ListSCCModules handles GET /api/v1/vaktprivacy/scc-modules.
func (h *Handler) ListSCCModules(c echo.Context) error {
	return c.JSON(http.StatusOK, h.service.ListSCCModules())
}

// CreateAVVFromTemplate handles POST /api/v1/vaktprivacy/avvs/from-template.
func (h *Handler) CreateAVVFromTemplate(c echo.Context) error {
	var in CreateAVVFromTemplateInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "PO_INVALID_BODY")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	avv, err := h.service.CreateAVVFromTemplate(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create avv from template")
		return errResp(c, http.StatusInternalServerError, "failed to create AVV from template", "PO_CREATE_AVV_TEMPLATE_FAILED")
	}
	return c.JSON(http.StatusCreated, avv)
}

// ExportAVVPDF handles GET /api/v1/vaktprivacy/avvs/:id/pdf.
func (h *Handler) ExportAVVPDF(c echo.Context) error {
	data, filename, err := h.service.ExportAVVPDF(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		log.Error().Err(err).Msg("export avv pdf")
		return errResp(c, http.StatusInternalServerError, "failed to generate AVV PDF", "PO_AVV_PDF_FAILED")
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	return c.Blob(http.StatusOK, "application/pdf", data)
}

// UpdateAVVSCC handles PATCH /api/v1/vaktprivacy/avvs/:id/scc.
func (h *Handler) UpdateAVVSCC(c echo.Context) error {
	var in UpdateAVVSCCInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "PO_INVALID_BODY")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	if err := h.service.UpdateAVVSCC(c.Request().Context(), orgID(c), c.Param("id"), in); err != nil {
		log.Error().Err(err).Msg("update avv scc")
		return errResp(c, http.StatusInternalServerError, "failed to update SCC fields", "PO_UPDATE_AVV_SCC_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ExportSCCPDF handles GET /api/v1/vaktprivacy/avvs/:id/scc.pdf.
func (h *Handler) ExportSCCPDF(c echo.Context) error {
	data, filename, err := h.service.ExportSCCPDF(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		log.Error().Err(err).Msg("export scc pdf")
		return errResp(c, http.StatusInternalServerError, "failed to generate SCC PDF", "PO_SCC_PDF_FAILED")
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	return c.Blob(http.StatusOK, "application/pdf", data)
}
