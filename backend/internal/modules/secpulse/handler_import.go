package secpulse

import (
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ImportFindings handles POST /api/v1/secpulse/findings/import
//
// Query params:
//
//	?asset_id=<uuid>
//	?format=sarif|cyclonedx|csv
//
// Body: multipart/form-data with field "file".
func (h *Handler) ImportFindings(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	assetID := c.QueryParam("asset_id")
	format := c.QueryParam("format")

	if assetID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "query param asset_id is required",
			"code":  "VB_BAD_REQUEST",
		})
	}

	validFormats := map[string]bool{"sarif": true, "cyclonedx": true, "csv": true}
	if !validFormats[format] {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "query param format must be one of: sarif, cyclonedx, csv",
			"code":  "VB_BAD_REQUEST",
		})
	}

	c.Request().Body = http.MaxBytesReader(c.Response().Writer, c.Request().Body, 5*1024*1024)

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

	data, err := io.ReadAll(src)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to read uploaded file",
			"code":  "VB_IMPORT_ERROR",
		})
	}

	ctx := c.Request().Context()
	var count int

	switch format {
	case "sarif":
		count, err = h.service.ImportSARIF(ctx, orgID, assetID, data)
	case "cyclonedx":
		count, err = h.service.ImportCycloneDX(ctx, orgID, assetID, data)
	case "csv":
		count, err = h.service.ImportCSV(ctx, orgID, assetID, data)
	}

	if err != nil {
		log.Error().Err(err).
			Str("org_id", orgID).
			Str("asset_id", assetID).
			Str("format", format).
			Msg("finding import failed")

		status := http.StatusBadRequest
		code := "VB_IMPORT_PARSE_ERROR"

		// Asset-not-found errors are 404.
		if count == 0 && isNotFoundError(err) {
			status = http.StatusNotFound
			code = "VB_ASSET_NOT_FOUND"
		}

		return c.JSON(status, map[string]string{
			"error": err.Error(),
			"code":  code,
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"imported": count,
		"format":   format,
		"asset_id": assetID,
	})
}

// isNotFoundError checks whether the error message indicates a missing asset.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "asset not found") || strings.Contains(msg, "not accessible")
}
