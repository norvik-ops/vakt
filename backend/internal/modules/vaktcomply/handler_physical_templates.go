// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S88-5: Physische-Maßnahmen-Templates HTTP handlers.

package vaktcomply

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ListPhysicalControlTemplates handles GET /api/v1/vaktcomply/physical-templates
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
