// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package admin

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// stagingHTTPClient is used for the promote-webhook outbound call.
// A 30-second timeout prevents hanging when the webhook host is unreachable.
var stagingHTTPClient = &http.Client{Timeout: 30 * time.Second}

// StagingHandler exposes staging-only endpoints for promoting builds to demo.
// Only registered when VAKT_STAGING=true.
type StagingHandler struct {
	promoteURL    string
	promoteSecret string
}

func NewStagingHandler(promoteURL, promoteSecret string) *StagingHandler {
	return &StagingHandler{promoteURL: promoteURL, promoteSecret: promoteSecret}
}

// GetStagingInfo handles GET /api/v1/admin/staging/info.
func (h *StagingHandler) GetStagingInfo(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{"staging": true})
}

// Promote handles POST /api/v1/admin/staging/promote.
// Calls the local vakt-promote-webhook on the host, which re-tags :staging → :latest
// and restarts the demo containers without requiring any GitHub credentials.
func (h *StagingHandler) Promote(c echo.Context) error {
	if h.promoteSecret == "" {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "VAKT_PROMOTE_SECRET not configured",
		})
	}

	req, err := http.NewRequestWithContext(c.Request().Context(), http.MethodPost, h.promoteURL, nil)
	if err != nil {
		log.Error().Err(err).Msg("promote request failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "promote request failed"})
	}
	req.Header.Set("X-Promote-Secret", h.promoteSecret)

	resp, err := stagingHTTPClient.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("promote webhook unreachable")
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "promote webhook unreachable"})
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "promote webhook returned non-200"})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "triggered"})
}
