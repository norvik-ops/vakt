// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package assets

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/audit"
	"github.com/matharnica/vakt/internal/shared/pagination"
)

// csvMaxBytes caps both CSV uploads.
const csvMaxBytes = 5 * 1024 * 1024 // 5 MB

// Handler handles HTTP requests for the asset inventory. The error codes keep
// their historic VB_ prefix: clients and the OpenAPI spec match on them.
type Handler struct {
	service  *Service
	validate *validator.Validate
}

// NewHandler creates a new asset handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service, validate: validator.New()}
}

func (h *Handler) audit(c echo.Context, action, resourceID, resourceName string) {
	orgID, _ := c.Get("org_id").(string)
	userID, _ := c.Get("user_id").(string)
	audit.Write(c.Request().Context(), h.service.db, audit.WriteEntry{
		OrgID:        orgID,
		UserID:       userID,
		Action:       action,
		ResourceType: "vaktcomply/asset",
		ResourceID:   resourceID,
		ResourceName: resourceName,
		IPAddress:    c.RealIP(),
	})
}

// CreateAsset handles POST /assets.
func (h *Handler) CreateAsset(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)

	var input CreateAssetInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "VB_BAD_REQUEST",
		})
	}
	if err := h.validate.Struct(input); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": "Ungültige Eingabe",
			"code":  "VALIDATION_ERROR",
		})
	}

	asset, err := h.service.CreateAsset(c.Request().Context(), orgID, input)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("create asset failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to create asset",
			"code":  "VB_CREATE_ASSET_ERROR",
		})
	}
	h.audit(c, "create", asset.ID, asset.Name)
	return c.JSON(http.StatusCreated, asset)
}

// ListAssets handles GET /assets.
func (h *Handler) ListAssets(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	_, limit, meta := pagination.FromRequest(c)
	// The service uses page number rather than offset.
	page := meta.Page
	tag := c.QueryParam("tag")

	assets, total, err := h.service.ListAssets(c.Request().Context(), orgID, page, limit, tag)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("list assets failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to list assets",
			"code":  "VB_LIST_ASSETS_ERROR",
		})
	}
	pagination.Complete(&meta, total)
	return c.JSON(http.StatusOK, pagination.Wrap(assets, meta))
}

// GetAsset handles GET /assets/:id.
func (h *Handler) GetAsset(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	assetID := c.Param("id")

	asset, err := h.service.GetAsset(c.Request().Context(), orgID, assetID)
	if err != nil {
		log.Debug().Err(err).Str("asset_id", assetID).Msg("get asset failed")
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "asset not found",
			"code":  "VB_ASSET_NOT_FOUND",
		})
	}
	// Augment response with soft-link field (best-effort; null when unlinked or column absent).
	pnaID, _ := h.service.GetAssetProtectionNeedID(c.Request().Context(), orgID, assetID)
	asset.ProtectionNeedID = pnaID
	return c.JSON(http.StatusOK, asset)
}

// GetAssetProtectionNeed handles GET /assets/:id/protection-need.
// Returns {"protection_need_id": "uuid-or-null"} for the given asset.
func (h *Handler) GetAssetProtectionNeed(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	assetID := c.Param("id")
	pnaID, err := h.service.GetAssetProtectionNeedID(c.Request().Context(), orgID, assetID)
	if err != nil {
		log.Error().Err(err).Str("asset_id", assetID).Msg("get asset protection need id")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to load protection need link",
			"code":  "VB_PROTECTION_NEED_LOAD_FAILED",
		})
	}
	return c.JSON(http.StatusOK, map[string]any{"protection_need_id": pnaID})
}

// UpdateAsset handles PUT /assets/:id.
func (h *Handler) UpdateAsset(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	assetID := c.Param("id")

	var input UpdateAssetInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "VB_BAD_REQUEST",
		})
	}
	if err := h.validate.Struct(input); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": "Ungültige Eingabe",
			"code":  "VALIDATION_ERROR",
		})
	}

	asset, err := h.service.UpdateAsset(c.Request().Context(), orgID, assetID, input)
	if err != nil {
		log.Error().Err(err).Str("asset_id", assetID).Msg("update asset failed")
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "asset not found or update failed",
			"code":  "VB_ASSET_NOT_FOUND",
		})
	}
	h.audit(c, "update", assetID, asset.Name)
	return c.JSON(http.StatusOK, asset)
}

// DeleteAsset handles DELETE /assets/:id.
func (h *Handler) DeleteAsset(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	assetID := c.Param("id")

	if err := h.service.DeleteAsset(c.Request().Context(), orgID, assetID); err != nil {
		log.Debug().Err(err).Str("asset_id", assetID).Msg("delete asset failed")
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "asset not found",
			"code":  "VB_ASSET_NOT_FOUND",
		})
	}
	h.audit(c, "delete", assetID, "")
	return c.NoContent(http.StatusNoContent)
}

// GetClassificationSummary handles GET /assets/classification-summary (S67-3).
func (h *Handler) GetClassificationSummary(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	summary, err := h.service.GetClassificationSummary(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("get classification summary")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to get classification summary",
			"code":  "VB_CLASSIFICATION_SUMMARY_FAILED",
		})
	}
	return c.JSON(http.StatusOK, summary)
}

// ImportAssets handles POST /assets/import (multipart/form-data, field "file").
func (h *Handler) ImportAssets(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)

	c.Request().Body = http.MaxBytesReader(c.Response().Writer, c.Request().Body, csvMaxBytes)

	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "field 'file' is required",
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

	inserted, errored, errs, err := h.service.ImportAssetsCSV(c.Request().Context(), orgID, src)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("CSV import failed")
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "CSV import failed",
			"code":  "VB_IMPORT_PARSE_ERROR",
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"inserted": inserted,
		"errored":  errored,
		"errors":   errs,
	})
}

// ImportAssetsCSVExtended handles POST /assets/import/csv.
//
// Multipart upload, field "file", max 5 MB.
// CSV format (first row = header): name,type,ip,owner,criticality
// Response: {"imported": N, "skipped": N, "errors": [...]}
func (h *Handler) ImportAssetsCSVExtended(c echo.Context) error {
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

	imported, skipped, errs, parseErr := h.service.ImportAssetsCSVExtended(c.Request().Context(), orgID, src)
	if parseErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": parseErr.Error(),
			"code":  "VB_IMPORT_PARSE_ERROR",
		})
	}
	log.Info().
		Str("org_id", orgID).
		Str("user_id", userID).
		Int("imported", imported).
		Int("skipped", skipped).
		Msg("assets CSV import complete")

	if errs == nil {
		errs = []string{}
	}
	return c.JSON(http.StatusOK, map[string]any{
		"imported": imported,
		"skipped":  skipped,
		"errors":   errs,
	})
}
