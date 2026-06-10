// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// InterestedParty is a stakeholder entry for ISO 27001 Clause 4.2.
type InterestedParty struct {
	ID               string  `json:"id"`
	OrgID            string  `json:"org_id"`
	Name             string  `json:"name"`
	Category         string  `json:"category"`
	Requirements     string  `json:"requirements,omitempty"`
	Concerns         string  `json:"concerns,omitempty"`
	ReviewDate       *string `json:"review_date,omitempty"`
	ReviewOverdue    bool    `json:"review_overdue"`
	IsSystemDefault  bool    `json:"is_system_default"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

// CreateInterestedPartyInput holds validated input for a new interested party.
type CreateInterestedPartyInput struct {
	Name         string  `json:"name"     validate:"required,max=200"`
	Category     string  `json:"category" validate:"required,oneof=customer regulator employee shareholder supplier insurer it_provider other"`
	Requirements string  `json:"requirements,omitempty" validate:"max=5000"`
	Concerns     string  `json:"concerns,omitempty"     validate:"max=5000"`
	ReviewDate   *string `json:"review_date,omitempty"`
}

// ListInterestedParties handles GET /api/v1/vaktcomply/interested-parties
func (h *Handler) ListInterestedParties(c echo.Context) error {
	parties, err := h.service.ListInterestedParties(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list interested parties")
		return errResp(c, http.StatusInternalServerError, "failed to list interested parties", "CK_INTERESTED_PARTIES_FAILED")
	}
	return c.JSON(http.StatusOK, parties)
}

// CreateInterestedParty handles POST /api/v1/vaktcomply/interested-parties
func (h *Handler) CreateInterestedParty(c echo.Context) error {
	var in CreateInterestedPartyInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	party, err := h.service.CreateInterestedParty(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create interested party")
		return errResp(c, http.StatusInternalServerError, "failed to create interested party", "CK_INTERESTED_PARTIES_FAILED")
	}
	return c.JSON(http.StatusCreated, party)
}

// UpdateInterestedParty handles PUT /api/v1/vaktcomply/interested-parties/:id
func (h *Handler) UpdateInterestedParty(c echo.Context) error {
	var in CreateInterestedPartyInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	party, err := h.service.UpdateInterestedParty(c.Request().Context(), orgID(c), c.Param("id"), in)
	if err != nil {
		log.Error().Err(err).Msg("update interested party")
		return errResp(c, http.StatusInternalServerError, "failed to update interested party", "CK_INTERESTED_PARTIES_FAILED")
	}
	return c.JSON(http.StatusOK, party)
}

// DeleteInterestedParty handles DELETE /api/v1/vaktcomply/interested-parties/:id
func (h *Handler) DeleteInterestedParty(c echo.Context) error {
	if err := h.service.DeleteInterestedParty(c.Request().Context(), orgID(c), c.Param("id")); err != nil {
		log.Error().Err(err).Msg("delete interested party")
		return errResp(c, http.StatusInternalServerError, "failed to delete interested party", "CK_INTERESTED_PARTIES_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// SeedDefaultInterestedParties handles POST /api/v1/vaktcomply/interested-parties/seed-defaults
// Inserts the 6 standard ISMS stakeholders if the org has none.
func (h *Handler) SeedDefaultInterestedParties(c echo.Context) error {
	if err := h.service.SeedDefaultInterestedParties(c.Request().Context(), orgID(c)); err != nil {
		log.Error().Err(err).Msg("seed default interested parties")
		return errResp(c, http.StatusInternalServerError, "failed to seed interested parties", "CK_INTERESTED_PARTIES_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ExportInterestedPartiesPDF handles GET /api/v1/vaktcomply/interested-parties/export
func (h *Handler) ExportInterestedPartiesPDF(c echo.Context) error {
	data, err := h.service.ExportInterestedPartiesPDF(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("export interested parties pdf")
		return errResp(c, http.StatusInternalServerError, "export failed", "CK_INTERESTED_PARTIES_EXPORT_FAILED")
	}
	filename := fmt.Sprintf("vakt-interested-parties-%s.pdf", time.Now().UTC().Format("2006-01-02"))
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	return c.Blob(http.StatusOK, "application/pdf", data)
}
