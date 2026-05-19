// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvitals

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// SoAEntry is one row in the Statement of Applicability.
type SoAEntry struct {
	ControlID                  string `json:"control_id"`
	FrameworkName              string `json:"framework_name"`
	Domain                     string `json:"domain"`
	Title                      string `json:"title"`
	Applicable                 bool   `json:"applicable"`
	Status                     string `json:"status"`
	JustificationApplicable    string `json:"justification_applicable,omitempty"`
	JustificationNotApplicable string `json:"justification_not_applicable,omitempty"`
}

// GetSoA handles GET /api/v1/secvitals/soa
func (h *Handler) GetSoA(c echo.Context) error {
	entries, err := h.service.GetSoAEntries(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get soa")
		return errResp(c, http.StatusInternalServerError, "failed to get SoA", "CK_SOA_FAILED")
	}
	return c.JSON(http.StatusOK, entries)
}

// GetSoACSV handles GET /api/v1/secvitals/soa.csv
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

// UpdateSoAApplicability handles PATCH /api/v1/secvitals/soa/:control_id
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
