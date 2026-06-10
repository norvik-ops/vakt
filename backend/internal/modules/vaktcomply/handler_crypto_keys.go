// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ListCryptoKeys handles GET /api/v1/vaktcomply/crypto-keys.
func (h *Handler) ListCryptoKeys(c echo.Context) error {
	keys, err := h.service.ListCryptoKeys(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list crypto keys")
		return errResp(c, http.StatusInternalServerError, "failed to list crypto keys", "CK_LIST_CRYPTO_KEYS_FAILED")
	}
	return c.JSON(http.StatusOK, keys)
}

// CreateCryptoKey handles POST /api/v1/vaktcomply/crypto-keys.
func (h *Handler) CreateCryptoKey(c echo.Context) error {
	var in CreateCryptoKeyInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	key, err := h.service.CreateCryptoKey(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create crypto key")
		return errResp(c, http.StatusInternalServerError, "failed to create crypto key", "CK_CREATE_CRYPTO_KEY_FAILED")
	}
	return c.JSON(http.StatusCreated, key)
}

// RotateCryptoKey handles POST /api/v1/vaktcomply/crypto-keys/:id/rotate.
func (h *Handler) RotateCryptoKey(c echo.Context) error {
	keyID := c.Param("id")
	key, err := h.service.RecordKeyRotation(c.Request().Context(), orgID(c), keyID)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "crypto key not found", "CK_CRYPTO_KEY_NOT_FOUND")
		}
		log.Error().Err(err).Str("key_id", keyID).Msg("rotate crypto key")
		return errResp(c, http.StatusInternalServerError, "failed to rotate crypto key", "CK_ROTATE_CRYPTO_KEY_FAILED")
	}
	return c.JSON(http.StatusOK, key)
}

// DeleteCryptoKey handles DELETE /api/v1/vaktcomply/crypto-keys/:id.
func (h *Handler) DeleteCryptoKey(c echo.Context) error {
	keyID := c.Param("id")
	if err := h.service.DeleteCryptoKey(c.Request().Context(), orgID(c), keyID); err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "crypto key not found", "CK_CRYPTO_KEY_NOT_FOUND")
		}
		log.Error().Err(err).Str("key_id", keyID).Msg("delete crypto key")
		return errResp(c, http.StatusInternalServerError, "failed to delete crypto key", "CK_DELETE_CRYPTO_KEY_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}
