// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S88-9: VVT→Control link HTTP handlers.

package vaktcomply

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ListControlVVTLinks handles GET /api/v1/vaktcomply/controls/:id/vvt-links
func (h *Handler) ListControlVVTLinks(c echo.Context) error {
	links, err := h.service.ListLinksForControl(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		log.Error().Err(err).Msg("list control vvt links")
		return errResp(c, http.StatusInternalServerError, "failed to list VVT links", "CK_VVT_LINKS_FAILED")
	}
	return c.JSON(http.StatusOK, links)
}

// ListVVTControlLinks handles GET /api/v1/vaktcomply/vvt-links?vvt_id=...
func (h *Handler) ListVVTControlLinks(c echo.Context) error {
	vvtID := c.QueryParam("vvt_id")
	if vvtID == "" {
		return errResp(c, http.StatusBadRequest, "vvt_id query param required", "CK_BAD_REQUEST")
	}
	links, err := h.service.ListLinksForVVT(c.Request().Context(), orgID(c), vvtID)
	if err != nil {
		log.Error().Err(err).Msg("list vvt control links")
		return errResp(c, http.StatusInternalServerError, "failed to list control links", "CK_VVT_LINKS_FAILED")
	}
	return c.JSON(http.StatusOK, links)
}

// CreateVVTControlLink handles POST /api/v1/vaktcomply/vvt-links
func (h *Handler) CreateVVTControlLink(c echo.Context) error {
	var in LinkVVTToControlInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, err.Error(), "CK_VALIDATION_ERROR")
	}
	link, err := h.service.LinkVVTToControl(c.Request().Context(), orgID(c), in)
	if err != nil {
		if err.Error() == "control not found" {
			return errResp(c, http.StatusNotFound, "control not found", "CK_CONTROL_NOT_FOUND")
		}
		log.Error().Err(err).Msg("create vvt control link")
		return errResp(c, http.StatusInternalServerError, "failed to link VVT", "CK_VVT_LINK_FAILED")
	}
	return c.JSON(http.StatusCreated, link)
}

// DeleteVVTControlLink handles DELETE /api/v1/vaktcomply/vvt-links/:id
func (h *Handler) DeleteVVTControlLink(c echo.Context) error {
	if err := h.service.UnlinkVVTFromControl(c.Request().Context(), orgID(c), c.Param("id")); err != nil {
		if err.Error() == "link not found" {
			return errResp(c, http.StatusNotFound, "link not found", "CK_VVT_LINK_NOT_FOUND")
		}
		log.Error().Err(err).Msg("delete vvt control link")
		return errResp(c, http.StatusInternalServerError, "failed to remove link", "CK_VVT_UNLINK_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}
