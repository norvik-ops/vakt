// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S69-6: TIA HTTP handlers.

package vaktprivacy

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ── Adequacy Decisions ────────────────────────────────────────────────────────

// ListAdequacyDecisions handles GET /api/v1/privacy/adequacy-decisions.
func (h *Handler) ListAdequacyDecisions(c echo.Context) error {
	decisions, err := h.tia.ListAdequacyDecisions(c.Request().Context())
	if err != nil {
		log.Error().Err(err).Msg("list adequacy decisions")
		return c.JSON(http.StatusInternalServerError, errBody("failed to list adequacy decisions", "PO_INTERNAL"))
	}
	return c.JSON(http.StatusOK, decisions)
}

// ── Data Transfers ────────────────────────────────────────────────────────────

// ListDataTransfers handles GET /api/v1/privacy/transfers.
func (h *Handler) ListDataTransfers(c echo.Context) error {
	transfers, err := h.tia.ListTransfers(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list data transfers")
		return c.JSON(http.StatusInternalServerError, errBody("failed to list transfers", "PO_INTERNAL"))
	}
	return c.JSON(http.StatusOK, transfers)
}

// CreateDataTransfer handles POST /api/v1/privacy/transfers.
func (h *Handler) CreateDataTransfer(c echo.Context) error {
	var input CreateTransferInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, errBody("invalid request", "PO_BAD_REQUEST"))
	}
	if err := h.validate.Struct(input); err != nil {
		return c.JSON(http.StatusBadRequest, errBody(err.Error(), "PO_VALIDATION"))
	}

	t, err := h.tia.CreateTransfer(c.Request().Context(), orgID(c), input)
	if err != nil {
		log.Error().Err(err).Msg("create data transfer")
		return c.JSON(http.StatusInternalServerError, errBody("failed to create transfer", "PO_INTERNAL"))
	}
	return c.JSON(http.StatusCreated, t)
}

// GetTransferComplianceStatus handles GET /api/v1/privacy/transfers/compliance.
func (h *Handler) GetTransferComplianceStatus(c echo.Context) error {
	status, err := h.tia.GetTransferComplianceStatus(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get transfer compliance status")
		return c.JSON(http.StatusInternalServerError, errBody("failed to get compliance status", "PO_INTERNAL"))
	}
	return c.JSON(http.StatusOK, status)
}

// ── TIA Documents ─────────────────────────────────────────────────────────────

// ListTIAs handles GET /api/v1/privacy/transfers/:id/tia.
func (h *Handler) ListTIAs(c echo.Context) error {
	tias, err := h.tia.ListTIAsForTransfer(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		log.Error().Err(err).Msg("list tias")
		return c.JSON(http.StatusInternalServerError, errBody("failed to list TIAs", "PO_INTERNAL"))
	}
	return c.JSON(http.StatusOK, tias)
}

// CreateTIA handles POST /api/v1/privacy/transfers/:id/tia.
func (h *Handler) CreateTIA(c echo.Context) error {
	userID, _ := c.Get("user_id").(string)

	var input CreateTIAInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, errBody("invalid request", "PO_BAD_REQUEST"))
	}
	if err := h.validate.Struct(input); err != nil {
		return c.JSON(http.StatusBadRequest, errBody(err.Error(), "PO_VALIDATION"))
	}

	tia, err := h.tia.CreateTIA(c.Request().Context(), orgID(c), c.Param("id"), userID, input)
	if err != nil {
		log.Error().Err(err).Msg("create tia")
		return c.JSON(http.StatusInternalServerError, errBody("failed to create TIA", "PO_INTERNAL"))
	}
	return c.JSON(http.StatusCreated, tia)
}

func errBody(msg, code string) map[string]string {
	return map[string]string{"error": msg, "code": code}
}
