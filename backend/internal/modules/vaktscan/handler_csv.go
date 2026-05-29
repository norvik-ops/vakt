// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

const (
	csvMaxBytes = 5 * 1024 * 1024 // 5 MB
	csvMaxRows  = 500
)

// ImportFindingsCSV handles POST /api/v1/vaktscan/findings/import/csv.
//
// Multipart upload, field "file", max 5 MB.
// CSV format (first row = header): title,severity,description,asset,status
//
// Severity aliases accepted (case-insensitive):
//
//	critical | hoch | high → "critical"
//	high     | mittel      → depends on exact string (see mapSeverity)
//
// At most 500 data rows are processed per request.
// Response: {"imported": N, "skipped": N, "errors": [...]}
func (h *Handler) ImportFindingsCSV(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)

	// Enforce 5 MB limit before reading.
	c.Request().Body = http.MaxBytesReader(c.Response().Writer, c.Request().Body, csvMaxBytes)

	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "multipart field 'file' is required",
			"code":  "VB_BAD_REQUEST",
		})
	}

	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to open uploaded file",
			"code":  "VB_IMPORT_ERROR",
		})
	}
	defer src.Close()

	imported, skipped, errs, parseErr := h.parseFindingsCSV(c, orgID, src)
	if parseErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": parseErr.Error(),
			"code":  "VB_IMPORT_PARSE_ERROR",
		})
	}

	if errs == nil {
		errs = []string{}
	}
	return c.JSON(http.StatusOK, map[string]any{
		"imported": imported,
		"skipped":  skipped,
		"errors":   errs,
	})
}

// parseFindingsCSV reads up to csvMaxRows rows from r and upserts findings.
// Returns (imported, skipped, rowErrors, fatalError).
func (h *Handler) parseFindingsCSV(c echo.Context, orgID string, r io.Reader) (int, int, []string, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		return 0, 0, nil, fmt.Errorf("read CSV header: %w", err)
	}

	colIdx := make(map[string]int, len(header))
	for i, h := range header {
		colIdx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	if _, ok := colIdx["title"]; !ok {
		return 0, 0, nil, fmt.Errorf("CSV missing required column \"title\"")
	}

	col := func(record []string, name string) string {
		if i, ok := colIdx[name]; ok && i < len(record) {
			return strings.TrimSpace(record[i])
		}
		return ""
	}

	ctx := c.Request().Context()
	cfg, _ := h.service.repo.GetSLAConfig(ctx, orgID)

	var imported, skipped int
	var errs []string
	rowNum := 0

	for {
		record, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return imported, skipped, errs, fmt.Errorf("read CSV row: %w", readErr)
		}

		rowNum++
		if rowNum > csvMaxRows {
			skipped++
			errs = append(errs, fmt.Sprintf("row %d: exceeded maximum of %d rows per import", rowNum, csvMaxRows))
			continue
		}

		title := col(record, "title")
		if title == "" {
			skipped++
			errs = append(errs, fmt.Sprintf("row %d: title is empty, skipped", rowNum))
			continue
		}

		severity := mapSeverity(col(record, "severity"))
		description := col(record, "description")
		assetRef := col(record, "asset")
		status := col(record, "status")
		if status == "" {
			status = "open"
		}

		// Resolve asset: accept UUID or name.
		assetID, resolveErr := h.service.repo.ResolveAssetRef(ctx, orgID, assetRef)
		if resolveErr != nil {
			skipped++
			errs = append(errs, fmt.Sprintf("row %d (%q): asset %q not found", rowNum, title, assetRef))
			continue
		}

		slaDueAt := calcSLADueAt(cfg, severity)
		rawID := title

		f := Finding{
			OrgID:       orgID,
			AssetID:     assetID,
			Title:       title,
			Description: description,
			Severity:    severity,
			Status:      status,
			Scanner:     "csv",
			RawID:       rawID,
			SLADueAt:    slaDueAt,
		}

		if _, upsertErr := h.service.repo.UpsertFindingByRawID(ctx, orgID, f); upsertErr != nil {
			skipped++
			errs = append(errs, fmt.Sprintf("row %d (%q): %s", rowNum, title, upsertErr))
			continue
		}
		imported++
	}

	return imported, skipped, errs, nil
}

// ImportAssetsCSVNew handles POST /api/v1/vaktscan/assets/import/csv.
//
// Multipart upload, field "file", max 5 MB.
// CSV format (first row = header): name,type,ip,owner,criticality
// Response: {"imported": N, "skipped": N, "errors": [...]}
func (h *Handler) ImportAssetsCSVNew(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	userID, _ := c.Get("user_id").(string)

	c.Request().Body = http.MaxBytesReader(c.Response().Writer, c.Request().Body, csvMaxBytes)

	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "multipart field 'file' is required",
			"code":  "VB_BAD_REQUEST",
		})
	}

	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to open uploaded file",
			"code":  "VB_IMPORT_ERROR",
		})
	}
	defer src.Close()

	imported, skipped, errs, parseErr := parseAssetsCSVNew(c, orgID, userID, src, h.service)
	if parseErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": parseErr.Error(),
			"code":  "VB_IMPORT_PARSE_ERROR",
		})
	}

	if errs == nil {
		errs = []string{}
	}
	return c.JSON(http.StatusOK, map[string]any{
		"imported": imported,
		"skipped":  skipped,
		"errors":   errs,
	})
}

// parseAssetsCSVNew reads the extended asset CSV format:
// name,type,ip,owner,criticality
func parseAssetsCSVNew(c echo.Context, orgID, userID string, r io.Reader, svc *Service) (int, int, []string, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		return 0, 0, nil, fmt.Errorf("read CSV header: %w", err)
	}

	colIdx := make(map[string]int, len(header))
	for i, h := range header {
		colIdx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	if _, ok := colIdx["name"]; !ok {
		return 0, 0, nil, fmt.Errorf("CSV missing required column \"name\"")
	}

	col := func(record []string, name string) string {
		if i, ok := colIdx[name]; ok && i < len(record) {
			return strings.TrimSpace(record[i])
		}
		return ""
	}

	ctx := c.Request().Context()

	var rows []CSVAssetRow
	var skipped int
	var errs []string
	rowNum := 0

	for {
		record, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return 0, skipped, errs, fmt.Errorf("read CSV row: %w", readErr)
		}

		rowNum++
		if rowNum > csvMaxRows {
			skipped++
			errs = append(errs, fmt.Sprintf("row %d: exceeded maximum of %d rows per import", rowNum, csvMaxRows))
			continue
		}

		name := col(record, "name")
		if name == "" {
			skipped++
			errs = append(errs, fmt.Sprintf("row %d: name is empty, skipped", rowNum))
			continue
		}

		assetType := col(record, "type")
		if assetType == "" {
			assetType = "server"
		}

		criticality := col(record, "criticality")
		if criticality == "" {
			criticality = "medium"
		}

		// ip is stored as external_url (there is no dedicated ip column in vb_assets).
		ip := col(record, "ip")

		rows = append(rows, CSVAssetRow{
			Name:        name,
			Type:        assetType,
			Criticality: criticality,
			ExternalURL: ip,
		})
	}

	if len(rows) == 0 {
		return 0, skipped, errs, nil
	}

	inserted, errored, bulkErrs := svc.repo.BulkCreateAssets(ctx, orgID, rows)
	log.Info().
		Str("org_id", orgID).
		Str("user_id", userID).
		Int("inserted", inserted).
		Int("errored", errored).
		Msg("assets CSV import complete")

	errs = append(errs, bulkErrs...)
	return inserted, skipped + errored, errs, nil
}

// mapSeverity normalises a raw severity string to the canonical set used in
// vb_findings (critical | high | medium | low | info).
func mapSeverity(raw string) string {
	switch strings.ToLower(raw) {
	case "critical", "kritisch":
		return "critical"
	case "hoch", "high":
		return "high"
	case "mittel", "medium":
		return "medium"
	case "niedrig", "low":
		return "low"
	case "info", "informational":
		return "info"
	default:
		return "medium"
	}
}
