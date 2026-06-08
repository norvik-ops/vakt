// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vakthr

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ListAccessConcepts handles GET /api/v1/hr/access-concepts.
func (h *Handler) ListAccessConcepts(c echo.Context) error {
	concepts, err := h.Service.ListAccessConcepts(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list access concepts")
		return errResp(c, http.StatusInternalServerError, "failed to list access concepts", "HR_LIST_ACCESS_CONCEPTS_FAILED")
	}
	if concepts == nil {
		concepts = []AccessConcept{}
	}
	return c.JSON(http.StatusOK, concepts)
}

// CreateAccessConcept handles POST /api/v1/hr/access-concepts.
func (h *Handler) CreateAccessConcept(c echo.Context) error {
	var in CreateAccessConceptInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "HR_INVALID_BODY")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	concept, err := h.Service.CreateAccessConcept(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create access concept")
		return errResp(c, http.StatusInternalServerError, "failed to create access concept", "HR_CREATE_ACCESS_CONCEPT_FAILED")
	}
	return c.JSON(http.StatusCreated, concept)
}

// GetAccessConcept handles GET /api/v1/hr/access-concepts/:id.
func (h *Handler) GetAccessConcept(c echo.Context) error {
	id := c.Param("id")
	concept, err := h.Service.GetAccessConcept(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusNotFound, "access concept not found", "HR_ACCESS_CONCEPT_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, concept)
}

// UpdateAccessConcept handles PATCH /api/v1/hr/access-concepts/:id.
func (h *Handler) UpdateAccessConcept(c echo.Context) error {
	id := c.Param("id")
	var in UpdateAccessConceptInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "HR_INVALID_BODY")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	concept, err := h.Service.UpdateAccessConcept(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		log.Error().Err(err).Str("concept_id", id).Msg("update access concept")
		return errResp(c, http.StatusInternalServerError, "failed to update access concept", "HR_UPDATE_ACCESS_CONCEPT_FAILED")
	}
	return c.JSON(http.StatusOK, concept)
}

// DeleteAccessConcept handles DELETE /api/v1/hr/access-concepts/:id.
func (h *Handler) DeleteAccessConcept(c echo.Context) error {
	id := c.Param("id")
	if err := h.Service.DeleteAccessConcept(c.Request().Context(), orgID(c), id); err != nil {
		log.Error().Err(err).Str("concept_id", id).Msg("delete access concept")
		return errResp(c, http.StatusInternalServerError, "failed to delete access concept", "HR_DELETE_ACCESS_CONCEPT_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ListAccessRoles handles GET /api/v1/hr/access-concepts/:id/roles.
func (h *Handler) ListAccessRoles(c echo.Context) error {
	conceptID := c.Param("id")
	roles, err := h.Service.ListAccessRoles(c.Request().Context(), orgID(c), conceptID)
	if err != nil {
		log.Error().Err(err).Str("concept_id", conceptID).Msg("list access roles")
		return errResp(c, http.StatusInternalServerError, "failed to list access roles", "HR_LIST_ACCESS_ROLES_FAILED")
	}
	if roles == nil {
		roles = []AccessRole{}
	}
	return c.JSON(http.StatusOK, roles)
}

// AddAccessRole handles POST /api/v1/hr/access-concepts/:id/roles.
func (h *Handler) AddAccessRole(c echo.Context) error {
	conceptID := c.Param("id")
	var in CreateAccessRoleInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "HR_INVALID_BODY")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	role, err := h.Service.AddAccessRole(c.Request().Context(), orgID(c), conceptID, in)
	if err != nil {
		log.Error().Err(err).Str("concept_id", conceptID).Msg("add access role")
		return errResp(c, http.StatusInternalServerError, "failed to add access role", "HR_ADD_ACCESS_ROLE_FAILED")
	}
	return c.JSON(http.StatusCreated, role)
}

// UpdateAccessRole handles PATCH /api/v1/hr/access-concepts/:id/roles/:rid.
func (h *Handler) UpdateAccessRole(c echo.Context) error {
	roleID := c.Param("rid")
	var in UpdateAccessRoleInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "HR_INVALID_BODY")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	role, err := h.Service.UpdateAccessRole(c.Request().Context(), orgID(c), roleID, in)
	if err != nil {
		log.Error().Err(err).Str("role_id", roleID).Msg("update access role")
		return errResp(c, http.StatusInternalServerError, "failed to update access role", "HR_UPDATE_ACCESS_ROLE_FAILED")
	}
	return c.JSON(http.StatusOK, role)
}

// DeleteAccessRole handles DELETE /api/v1/hr/access-concepts/:id/roles/:rid.
func (h *Handler) DeleteAccessRole(c echo.Context) error {
	roleID := c.Param("rid")
	if err := h.Service.DeleteAccessRole(c.Request().Context(), orgID(c), roleID); err != nil {
		log.Error().Err(err).Str("role_id", roleID).Msg("delete access role")
		return errResp(c, http.StatusInternalServerError, "failed to delete access role", "HR_DELETE_ACCESS_ROLE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// SnapshotAccessConceptVersion handles POST /api/v1/hr/access-concepts/:id/versions.
// Captures a versioned snapshot of all current roles for the access concept.
func (h *Handler) SnapshotAccessConceptVersion(c echo.Context) error {
	conceptID := c.Param("id")
	summary, err := h.Service.SnapshotVersion(c.Request().Context(), orgID(c), conceptID)
	if err != nil {
		log.Error().Err(err).Str("concept_id", conceptID).Msg("snapshot access concept version")
		return errResp(c, http.StatusInternalServerError, "failed to snapshot version", "HR_SNAPSHOT_VERSION_FAILED")
	}
	return c.JSON(http.StatusCreated, summary)
}

// ListAccessConceptVersions handles GET /api/v1/hr/access-concepts/:id/versions.
func (h *Handler) ListAccessConceptVersions(c echo.Context) error {
	conceptID := c.Param("id")
	versions, err := h.Service.ListAccessConceptVersions(c.Request().Context(), orgID(c), conceptID)
	if err != nil {
		log.Error().Err(err).Str("concept_id", conceptID).Msg("list access concept versions")
		return errResp(c, http.StatusInternalServerError, "failed to list versions", "HR_LIST_VERSIONS_FAILED")
	}
	if versions == nil {
		versions = []AccessConceptVersionSummary{}
	}
	return c.JSON(http.StatusOK, versions)
}
