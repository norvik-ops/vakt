// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// ListCertificates handles GET /api/v1/vaktscan/certificates.
func (h *Handler) ListCertificates(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	certs, err := h.service.repo.ListCertificates(c.Request().Context(), orgID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to list certificates",
			"code":  "VB_INTERNAL",
		})
	}
	if certs == nil {
		certs = []Certificate{}
	}
	return c.JSON(http.StatusOK, map[string]any{"data": certs})
}

// CreateCertificate handles POST /api/v1/vaktscan/certificates.
func (h *Handler) CreateCertificate(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)

	var input CreateCertificateInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "VB_BAD_REQUEST",
		})
	}
	if err := h.validate.Struct(input); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": err.Error(),
			"code":  "VB_VALIDATION",
		})
	}

	cert, err := h.service.repo.CreateCertificate(c.Request().Context(), orgID, input)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to create certificate",
			"code":  "VB_INTERNAL",
		})
	}
	return c.JSON(http.StatusCreated, cert)
}

// GetCertificate handles GET /api/v1/vaktscan/certificates/:id.
func (h *Handler) GetCertificate(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	id := c.Param("id")

	cert, err := h.service.repo.GetCertificate(c.Request().Context(), orgID, id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "certificate not found",
			"code":  "VB_NOT_FOUND",
		})
	}
	return c.JSON(http.StatusOK, cert)
}

// DeleteCertificate handles DELETE /api/v1/vaktscan/certificates/:id.
func (h *Handler) DeleteCertificate(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	id := c.Param("id")

	if err := h.service.repo.DeleteCertificate(c.Request().Context(), orgID, id); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "certificate not found",
			"code":  "VB_NOT_FOUND",
		})
	}
	h.audit(c, "delete", "certificate", id, id)
	return c.NoContent(http.StatusNoContent)
}

// ScanCertificate handles POST /api/v1/vaktscan/certificates/:id/scan — manual rescan.
func (h *Handler) ScanCertificate(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	id := c.Param("id")

	cert, err := h.service.repo.GetCertificate(c.Request().Context(), orgID, id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "certificate not found",
			"code":  "VB_NOT_FOUND",
		})
	}

	info, scanErr := ScanTLSCertificate(cert.Domain)
	if updateErr := h.service.repo.UpdateCertificateScan(c.Request().Context(), orgID, id, info, scanErr); updateErr != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to update certificate scan",
			"code":  "VB_INTERNAL",
		})
	}

	updated, err := h.service.repo.GetCertificate(c.Request().Context(), orgID, id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve updated certificate",
			"code":  "VB_INTERNAL",
		})
	}
	return c.JSON(http.StatusOK, updated)
}

// GetExpiringCertificates handles GET /api/v1/vaktscan/certificates/expiring.
// Returns certificates expiring within the next N days (default 30).
func (h *Handler) GetExpiringCertificates(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	certs, err := h.service.repo.ListExpiringCertificates(c.Request().Context(), orgID, 30)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to list expiring certificates",
			"code":  "VB_INTERNAL",
		})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"items": certs,
		"count": len(certs),
	})
}

// ScanAllCertificatesNow handles POST /api/v1/vaktscan/certificates/check-now.
// Triggers an immediate rescan of all certificates for the org.
func (h *Handler) ScanAllCertificatesNow(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	n, err := ScanAllCertificatesForOrg(c.Request().Context(), h.service.repo.DB(), orgID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "scan failed",
			"code":  "VB_INTERNAL",
		})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"scanned": n})
}
