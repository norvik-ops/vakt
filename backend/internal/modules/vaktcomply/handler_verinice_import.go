// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S88-4: verinice-(.vna)-Import HTTP handlers (upload → dry-run → commit).

package vaktcomply

import (
	"io"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/audit"
	"github.com/matharnica/vakt/internal/shared/veriniceimport"
)

// readVNAUpload extracts the uploaded .vna bytes, bounded to MaxArchiveSize.
func readVNAUpload(c echo.Context) ([]byte, error) {
	fh, err := c.FormFile("file")
	if err != nil {
		return nil, errResp(c, http.StatusBadRequest, "file is required", "CK_BAD_REQUEST")
	}
	if fh.Size > veriniceimport.MaxArchiveSize {
		return nil, errResp(c, http.StatusRequestEntityTooLarge, "file too large", "CK_FILE_TOO_LARGE")
	}
	src, err := fh.Open()
	if err != nil {
		return nil, errResp(c, http.StatusInternalServerError, "failed to open upload", "CK_UPLOAD_FAILED")
	}
	defer src.Close()
	data, err := io.ReadAll(io.LimitReader(src, veriniceimport.MaxArchiveSize+1))
	if err != nil {
		return nil, errResp(c, http.StatusInternalServerError, "failed to read upload", "CK_UPLOAD_FAILED")
	}
	return data, nil
}

// PreviewVeriniceImport handles POST /api/v1/vaktcomply/verinice-import/preview
func (h *Handler) PreviewVeriniceImport(c echo.Context) error {
	data, errResponse := readVNAUpload(c)
	if errResponse != nil {
		return errResponse
	}
	preview, err := h.service.PreviewVeriniceImport(data)
	if err != nil {
		log.Warn().Err(err).Msg("verinice import preview")
		return errResp(c, http.StatusBadRequest, "failed to parse .vna file", "CK_VNA_PARSE_FAILED")
	}
	return c.JSON(http.StatusOK, preview)
}

// CommitVeriniceImport handles POST /api/v1/vaktcomply/verinice-import/commit
func (h *Handler) CommitVeriniceImport(c echo.Context) error {
	data, errResponse := readVNAUpload(c)
	if errResponse != nil {
		return errResponse
	}
	res, err := h.service.CommitVeriniceImport(c.Request().Context(), orgID(c), userID(c), data)
	if err != nil {
		log.Warn().Err(err).Msg("verinice import commit")
		return errResp(c, http.StatusBadRequest, "failed to import .vna file", "CK_VNA_IMPORT_FAILED")
	}
	// Structured audit-log entry per import (who, what counts).
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID: orgID(c), UserID: userID(c), Action: "import",
		ResourceType: "vakt-comply/verinice-import",
		ResourceName: "verinice .vna",
		IPAddress:    c.RealIP(),
		Details: map[string]string{
			"assets_created":   strconv.Itoa(res.AssetsCreated),
			"controls_created": strconv.Itoa(res.ControlsCreated),
			"risks_created":    strconv.Itoa(res.RisksCreated),
			"skipped":          strconv.Itoa(res.Skipped),
		},
	})
	return c.JSON(http.StatusOK, res)
}
