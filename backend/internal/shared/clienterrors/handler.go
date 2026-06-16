// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package clienterrors

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/logsafe"
)

// Handler wires the client-error endpoints to the repository.
type Handler struct {
	repo *Repository
}

// NewHandler constructs a Handler.
func NewHandler(repo *Repository) *Handler { return &Handler{repo: repo} }

// reportPayload mirrors what the React ErrorBoundary posts.
type reportPayload struct {
	Message        string `json:"message"`
	Stack          string `json:"stack"`
	ComponentStack string `json:"component_stack"`
	URL            string `json:"url"`
	TraceID        string `json:"trace_id"`
}

// Record handles POST /api/v1/errors. It logs the (sanitized) error for ops
// visibility and persists it. org_id/user_id are nil for pre-login errors.
func (h *Handler) Record(c echo.Context) error {
	var p reportPayload
	if err := c.Bind(&p); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	var orgID, userID *string
	if v, ok := c.Get("org_id").(string); ok && v != "" {
		orgID = &v
	}
	if v, ok := c.Get("user_id").(string); ok && v != "" {
		userID = &v
	}

	in := RecordInput{
		OrgID: orgID, UserID: userID,
		Message: p.Message, Stack: p.Stack, ComponentStack: p.ComponentStack,
		URL: p.URL, UserAgent: c.Request().Header.Get("User-Agent"), TraceID: p.TraceID,
	}

	log.Error().
		Str("source", "client").
		Str("url", logsafe.SanitizeField(in.URL, 512)).
		Str("trace_id", logsafe.SanitizeField(in.TraceID, 64)).
		Str("message", logsafe.SanitizeField(in.Message, 500)).
		Msg("client-side error boundary triggered")

	if err := h.repo.Record(c.Request().Context(), in); err != nil {
		log.Warn().Err(err).Msg("client error: persist failed (logged only)")
	}
	return c.NoContent(http.StatusNoContent)
}

// List handles GET /api/v1/admin/client-errors (Admin only) — last 200 entries
// for the caller's org plus unscoped pre-login errors.
func (h *Handler) List(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	entries, err := h.repo.ListForOrg(c.Request().Context(), orgID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "query failed"})
	}
	return c.JSON(http.StatusOK, entries)
}
