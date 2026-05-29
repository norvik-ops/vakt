// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/db"
)

// ListDBPolicyTemplates handles GET /api/v1/vaktcomply/templates
// Query params: ?category=policy|dpia|avv
//
// Returns DB-backed compliance templates from ck_policy_templates.
// Templates are global (no org_id) — they are the same for all tenants.
func (h *Handler) ListDBPolicyTemplates(c echo.Context) error {
	ctx := c.Request().Context()
	category := c.QueryParam("category")

	if category != "" && category != "policy" && category != "dpia" && category != "avv" {
		return errResp(c, http.StatusBadRequest, "invalid category; must be policy, dpia, or avv", "INVALID_CATEGORY")
	}

	arg := db.ListCKPolicyTemplatesParams{}
	if category != "" {
		arg.Category = pgtype.Text{String: category, Valid: true}
	}

	rows, queryErr := h.q.ListCKPolicyTemplates(ctx, arg)
	if queryErr != nil {
		log.Error().Err(queryErr).Msg("ListDBPolicyTemplates: query failed")
		return errResp(c, http.StatusInternalServerError, "failed to list templates", "DB_ERROR")
	}

	templates := make([]DBPolicyTemplate, 0, len(rows))
	for _, r := range rows {
		templates = append(templates, templateListRowToDTO(r))
	}
	return c.JSON(http.StatusOK, templates)
}

// GetDBPolicyTemplate handles GET /api/v1/vaktcomply/templates/:id
//
// Returns a single DB-backed compliance template by UUID.
func (h *Handler) GetDBPolicyTemplate(c echo.Context) error {
	ctx := c.Request().Context()
	id := c.Param("id")
	if id == "" {
		return errResp(c, http.StatusBadRequest, "missing template id", "MISSING_ID")
	}

	r, err := h.q.GetCKPolicyTemplateByID(ctx, id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("GetDBPolicyTemplate: not found")
		return errResp(c, http.StatusNotFound, "template not found", "TEMPLATE_NOT_FOUND")
	}

	return c.JSON(http.StatusOK, templateGetRowToDTO(r))
}

// templateListRowToDTO converts a ListCKPolicyTemplatesRow to DBPolicyTemplate.
// COALESCE(framework, ”) means empty string signals DB NULL — convert back to nil.
func templateListRowToDTO(r db.ListCKPolicyTemplatesRow) DBPolicyTemplate {
	var fw *string
	if r.Framework != "" {
		fw = &r.Framework
	}
	return DBPolicyTemplate{
		ID:          r.ID,
		Category:    r.Category,
		Name:        r.Name,
		Description: r.Description,
		Content:     r.Content,
		Tags:        r.Tags,
		Framework:   fw,
		CreatedAt:   r.CreatedAt,
	}
}

// templateGetRowToDTO converts a GetCKPolicyTemplateByIDRow to DBPolicyTemplate.
func templateGetRowToDTO(r db.GetCKPolicyTemplateByIDRow) DBPolicyTemplate {
	var fw *string
	if r.Framework != "" {
		fw = &r.Framework
	}
	return DBPolicyTemplate{
		ID:          r.ID,
		Category:    r.Category,
		Name:        r.Name,
		Description: r.Description,
		Content:     r.Content,
		Tags:        r.Tags,
		Framework:   fw,
		CreatedAt:   r.CreatedAt,
	}
}
