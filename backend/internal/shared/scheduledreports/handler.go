// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package scheduledreports

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/auth"
)

// Handler handles HTTP requests for scheduled reports.
type Handler struct {
	svc      *Service
	validate *validator.Validate
}

// NewHandler creates a new scheduled reports handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{
		svc:      svc,
		validate: validator.New(),
	}
}

// Register wires scheduled report routes under the provided group.
// Expected: group is already at /api/v1/reports.
//
// S121-B1 (R1): scheduled reports email compliance/findings data to an
// operator-chosen external address on a recurring basis — a data-exfiltration
// vector. Every mutating route is gated to Admin so read-only (Viewer) and
// read-write (SecurityAnalyst) roles cannot schedule an exfil. Reading the
// schedule list stays open to any authenticated user.
func Register(g *echo.Group, h *Handler) {
	admin := auth.RequireRole("Admin")
	g.GET("/scheduled", h.List)
	g.POST("/scheduled", h.Create, admin)
	g.PUT("/scheduled/:id", h.Update, admin)
	g.DELETE("/scheduled/:id", h.Delete, admin)
	g.POST("/scheduled/:id/run", h.RunNow, admin)
}

// orgID extracts org_id from the Echo context.
func orgID(c echo.Context) string {
	v, _ := c.Get("org_id").(string)
	return v
}

// errResp returns a standardised JSON error response.
func errResp(c echo.Context, code int, msg, errCode string) error {
	return c.JSON(code, map[string]string{
		"error": msg,
		"code":  errCode,
	})
}

// List handles GET /api/v1/reports/scheduled.
func (h *Handler) List(c echo.Context) error {
	reports, err := h.svc.List(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list scheduled reports")
		return errResp(c, http.StatusInternalServerError, "failed to list scheduled reports", "SR_LIST_ERROR")
	}
	return c.JSON(http.StatusOK, reports)
}

// Create handles POST /api/v1/reports/scheduled.
func (h *Handler) Create(c echo.Context) error {
	var in CreateScheduledReportInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "SR_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": "Ungültige Eingabe",
			"code":  "VALIDATION_ERROR",
		})
	}
	r, err := h.svc.Create(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create scheduled report")
		return errResp(c, http.StatusInternalServerError, "failed to create scheduled report", "SR_CREATE_ERROR")
	}
	return c.JSON(http.StatusCreated, r)
}

// Update handles PUT /api/v1/reports/scheduled/:id.
func (h *Handler) Update(c echo.Context) error {
	id := c.Param("id")
	var in UpdateScheduledReportInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "SR_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": "Ungültige Eingabe",
			"code":  "VALIDATION_ERROR",
		})
	}
	r, err := h.svc.Update(c.Request().Context(), id, orgID(c), in)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("update scheduled report")
		return errResp(c, http.StatusNotFound, "scheduled report not found or update failed", "SR_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, r)
}

// Delete handles DELETE /api/v1/reports/scheduled/:id.
func (h *Handler) Delete(c echo.Context) error {
	id := c.Param("id")
	if err := h.svc.Delete(c.Request().Context(), id, orgID(c)); err != nil {
		log.Error().Err(err).Str("id", id).Msg("delete scheduled report")
		return errResp(c, http.StatusNotFound, "scheduled report not found", "SR_NOT_FOUND")
	}
	return c.NoContent(http.StatusNoContent)
}

// RunNow handles POST /api/v1/reports/scheduled/:id/run.
func (h *Handler) RunNow(c echo.Context) error {
	id := c.Param("id")
	if err := h.svc.RunNow(c.Request().Context(), id, orgID(c)); err != nil {
		log.Error().Err(err).Str("id", id).Msg("run scheduled report")
		return errResp(c, http.StatusInternalServerError, "failed to run report", "SR_RUN_ERROR")
	}
	return c.JSON(http.StatusAccepted, map[string]string{"message": "report is being delivered"})
}
