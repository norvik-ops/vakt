// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvitals

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ListDBPolicyTemplates handles GET /api/v1/secvitals/templates
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

	var templates []DBPolicyTemplate

	if category == "" {
		pgRows, queryErr := h.db.Query(ctx, `
			SELECT id, category, name, description, content, tags, framework, created_at
			FROM ck_policy_templates
			ORDER BY category, name
		`)
		if queryErr != nil {
			log.Error().Err(queryErr).Msg("ListDBPolicyTemplates: query failed")
			return errResp(c, http.StatusInternalServerError, "failed to list templates", "DB_ERROR")
		}
		defer pgRows.Close()

		for pgRows.Next() {
			var t DBPolicyTemplate
			if scanErr := pgRows.Scan(&t.ID, &t.Category, &t.Name, &t.Description, &t.Content, &t.Tags, &t.Framework, &t.CreatedAt); scanErr != nil {
				log.Error().Err(scanErr).Msg("ListDBPolicyTemplates: scan failed")
				continue
			}
			templates = append(templates, t)
		}
		if pgRows.Err() != nil {
			log.Error().Err(pgRows.Err()).Msg("ListDBPolicyTemplates: rows error")
			return errResp(c, http.StatusInternalServerError, "failed to read templates", "DB_ERROR")
		}
	} else {
		pgRows, queryErr := h.db.Query(ctx, `
			SELECT id, category, name, description, content, tags, framework, created_at
			FROM ck_policy_templates
			WHERE category = $1
			ORDER BY name
		`, category)
		if queryErr != nil {
			log.Error().Err(queryErr).Msg("ListDBPolicyTemplates: query failed")
			return errResp(c, http.StatusInternalServerError, "failed to list templates", "DB_ERROR")
		}
		defer pgRows.Close()

		for pgRows.Next() {
			var t DBPolicyTemplate
			if scanErr := pgRows.Scan(&t.ID, &t.Category, &t.Name, &t.Description, &t.Content, &t.Tags, &t.Framework, &t.CreatedAt); scanErr != nil {
				log.Error().Err(scanErr).Msg("ListDBPolicyTemplates: scan failed")
				continue
			}
			templates = append(templates, t)
		}
		if pgRows.Err() != nil {
			log.Error().Err(pgRows.Err()).Msg("ListDBPolicyTemplates: rows error")
			return errResp(c, http.StatusInternalServerError, "failed to read templates", "DB_ERROR")
		}
	}

	if templates == nil {
		templates = []DBPolicyTemplate{}
	}
	return c.JSON(http.StatusOK, templates)
}

// GetDBPolicyTemplate handles GET /api/v1/secvitals/templates/:id
//
// Returns a single DB-backed compliance template by UUID.
func (h *Handler) GetDBPolicyTemplate(c echo.Context) error {
	ctx := c.Request().Context()
	id := c.Param("id")
	if id == "" {
		return errResp(c, http.StatusBadRequest, "missing template id", "MISSING_ID")
	}

	var t DBPolicyTemplate
	err := h.db.QueryRow(ctx, `
		SELECT id, category, name, description, content, tags, framework, created_at
		FROM ck_policy_templates
		WHERE id = $1
	`, id).Scan(&t.ID, &t.Category, &t.Name, &t.Description, &t.Content, &t.Tags, &t.Framework, &t.CreatedAt)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("GetDBPolicyTemplate: not found")
		return errResp(c, http.StatusNotFound, "template not found", "TEMPLATE_NOT_FOUND")
	}

	return c.JSON(http.StatusOK, t)
}
