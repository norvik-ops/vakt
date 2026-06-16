// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S88-2: Backup-/Restore-Nachweis HTTP handlers.

package vaktcomply

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ListBackupJobs handles GET /api/v1/vaktcomply/backup/jobs
func (h *Handler) ListBackupJobs(c echo.Context) error {
	jobs, err := h.service.ListBackupJobs(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list backup jobs")
		return errResp(c, http.StatusInternalServerError, "failed to list backup jobs", "CK_BACKUP_LIST_FAILED")
	}
	return c.JSON(http.StatusOK, jobs)
}

// GetBackupSummary handles GET /api/v1/vaktcomply/backup/summary
func (h *Handler) GetBackupSummary(c echo.Context) error {
	sum, err := h.service.GetBackupSummary(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("backup summary")
		return errResp(c, http.StatusInternalServerError, "failed to compute backup summary", "CK_BACKUP_SUMMARY_FAILED")
	}
	return c.JSON(http.StatusOK, sum)
}

// CreateBackupJob handles POST /api/v1/vaktcomply/backup/jobs
func (h *Handler) CreateBackupJob(c echo.Context) error {
	var in BackupJobInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, err.Error(), "CK_VALIDATION_ERROR")
	}
	job, err := h.service.CreateBackupJob(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create backup job")
		return errResp(c, http.StatusInternalServerError, "failed to create backup job", "CK_BACKUP_CREATE_FAILED")
	}
	return c.JSON(http.StatusCreated, job)
}

// UpdateBackupJob handles PUT /api/v1/vaktcomply/backup/jobs/:id
func (h *Handler) UpdateBackupJob(c echo.Context) error {
	var in BackupJobInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, err.Error(), "CK_VALIDATION_ERROR")
	}
	job, err := h.service.UpdateBackupJob(c.Request().Context(), orgID(c), c.Param("id"), in)
	if err != nil {
		if err.Error() == "backup job not found" {
			return errResp(c, http.StatusNotFound, "backup job not found", "CK_BACKUP_NOT_FOUND")
		}
		log.Error().Err(err).Msg("update backup job")
		return errResp(c, http.StatusInternalServerError, "failed to update backup job", "CK_BACKUP_UPDATE_FAILED")
	}
	return c.JSON(http.StatusOK, job)
}

// DeleteBackupJob handles DELETE /api/v1/vaktcomply/backup/jobs/:id
func (h *Handler) DeleteBackupJob(c echo.Context) error {
	if err := h.service.DeleteBackupJob(c.Request().Context(), orgID(c), c.Param("id")); err != nil {
		if err.Error() == "backup job not found" {
			return errResp(c, http.StatusNotFound, "backup job not found", "CK_BACKUP_NOT_FOUND")
		}
		log.Error().Err(err).Msg("delete backup job")
		return errResp(c, http.StatusInternalServerError, "failed to delete backup job", "CK_BACKUP_DELETE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ListRestoreTests handles GET /api/v1/vaktcomply/backup/jobs/:id/restore-tests
func (h *Handler) ListRestoreTests(c echo.Context) error {
	tests, err := h.service.ListRestoreTests(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		log.Error().Err(err).Msg("list restore tests")
		return errResp(c, http.StatusInternalServerError, "failed to list restore tests", "CK_RESTORE_LIST_FAILED")
	}
	return c.JSON(http.StatusOK, tests)
}

// CreateRestoreTest handles POST /api/v1/vaktcomply/backup/jobs/:id/restore-tests
func (h *Handler) CreateRestoreTest(c echo.Context) error {
	var in RestoreTestInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, err.Error(), "CK_VALIDATION_ERROR")
	}
	test, err := h.service.CreateRestoreTest(c.Request().Context(), orgID(c), c.Param("id"), in)
	if err != nil {
		if err.Error() == "backup job not found" {
			return errResp(c, http.StatusNotFound, "backup job not found", "CK_BACKUP_NOT_FOUND")
		}
		log.Error().Err(err).Msg("create restore test")
		return errResp(c, http.StatusInternalServerError, "failed to create restore test", "CK_RESTORE_CREATE_FAILED")
	}
	return c.JSON(http.StatusCreated, test)
}
