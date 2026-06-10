// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S70-4: HTTP handlers for Contractor/Freelancer lifecycle management.

package vakthr

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ListContractors handles GET /api/v1/vakthr/contractors.
func (h *Handler) ListContractors(c echo.Context) error {
	contractors, err := h.Service.ListContractors(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list contractors")
		return errResp(c, http.StatusInternalServerError, "failed to list contractors", "HR_CONTRACTORS_LIST_FAILED")
	}
	if contractors == nil {
		contractors = []Contractor{}
	}
	return c.JSON(http.StatusOK, contractors)
}

// GetContractor handles GET /api/v1/vakthr/contractors/:id.
func (h *Handler) GetContractor(c echo.Context) error {
	contractor, err := h.Service.GetContractor(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		return errResp(c, http.StatusNotFound, "contractor not found", "HR_CONTRACTOR_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, contractor)
}

// CreateContractor handles POST /api/v1/vakthr/contractors.
func (h *Handler) CreateContractor(c echo.Context) error {
	var in CreateContractorInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "HR_INVALID_BODY")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	contractor, err := h.Service.CreateContractor(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create contractor")
		return errResp(c, http.StatusInternalServerError, "failed to create contractor", "HR_CONTRACTOR_CREATE_FAILED")
	}
	return c.JSON(http.StatusCreated, contractor)
}

// UpdateContractor handles PUT /api/v1/vakthr/contractors/:id.
func (h *Handler) UpdateContractor(c echo.Context) error {
	var in UpdateContractorInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "HR_INVALID_BODY")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	contractor, err := h.Service.UpdateContractor(c.Request().Context(), orgID(c), c.Param("id"), in)
	if err != nil {
		log.Error().Err(err).Msg("update contractor")
		return errResp(c, http.StatusInternalServerError, "failed to update contractor", "HR_CONTRACTOR_UPDATE_FAILED")
	}
	return c.JSON(http.StatusOK, contractor)
}
