// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	auditmod "github.com/matharnica/vakt/internal/modules/vaktcomply/audit"
	"github.com/matharnica/vakt/internal/shared/audit"
)

// isOrgAdmin returns true when the authenticated user has the Admin role in the organisation.
func (h *Handler) isOrgAdmin(c echo.Context) (bool, error) {
	uid := userID(c)
	oid := orgID(c)
	if uid == "" || oid == "" {
		return false, nil
	}
	roleName, err := h.service.Audit.GetOrgMemberRole(c.Request().Context(), uid, oid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return roleName == "Admin", nil
}

// ─── Request approval ─────────────────────────────────────────────────────────

// ApprovalRequestInput is the validated body for POST /controls/:id/approval-request.
type ApprovalRequestInput struct {
	RequestedStatus string `json:"requested_status" validate:"required,oneof=missing in_progress implemented not_applicable"`
	Comment         string `json:"comment"          validate:"max=2000"`
}

// RequestControlApproval handles POST /api/v1/vaktcomply/controls/:id/approval-request.
// Non-admin users submit a status-change request; admins get a 409 telling them to use the direct PATCH.
func (h *Handler) RequestControlApproval(c echo.Context) error {
	controlID := c.Param("id")

	var in ApprovalRequestInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "validation error", "CK_VALIDATION_ERROR")
	}

	admin, err := h.isOrgAdmin(c)
	if err != nil {
		log.Error().Err(err).Msg("check admin role for approval request")
		return errResp(c, http.StatusInternalServerError, "role check failed", "CK_INTERNAL")
	}
	if admin {
		return errResp(c, http.StatusConflict,
			"admins können Status direkt ändern — kein Genehmigungsantrag nötig",
			"CK_APPROVAL_ADMIN_DIRECT",
		)
	}

	// Fetch current control status.
	ctrl, err := h.service.GetControl(c.Request().Context(), orgID(c), controlID)
	if err != nil {
		log.Error().Err(err).Msg("get control for approval request")
		return errResp(c, http.StatusNotFound, "control not found", "CK_NOT_FOUND")
	}

	approval, err := h.service.Audit.CreateApprovalRequest(
		c.Request().Context(),
		orgID(c), controlID, userID(c),
		in.RequestedStatus, ctrl.Status, in.Comment,
	)
	if err != nil {
		log.Error().Err(err).Msg("create approval request")
		return errResp(c, http.StatusInternalServerError, "failed to create approval request", "CK_INTERNAL")
	}

	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID:        orgID(c),
		UserID:       userID(c),
		Action:       "request_approval",
		ResourceType: "vakt-comply/control",
		ResourceID:   controlID,
		ResourceName: ctrl.Title,
		IPAddress:    c.RealIP(),
	})

	return c.JSON(http.StatusCreated, approval)
}

// ─── List pending approvals ───────────────────────────────────────────────────

// ListPendingApprovals handles GET /api/v1/vaktcomply/approvals.
// Admin-only: returns all pending approval requests for the org.
func (h *Handler) ListPendingApprovals(c echo.Context) error {
	admin, err := h.isOrgAdmin(c)
	if err != nil {
		log.Error().Err(err).Msg("check admin role for list approvals")
		return errResp(c, http.StatusInternalServerError, "role check failed", "CK_INTERNAL")
	}
	if !admin {
		return errResp(c, http.StatusForbidden, "admin role required", "CK_FORBIDDEN")
	}

	approvals, err := h.service.Audit.ListPendingApprovals(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list pending approvals")
		return errResp(c, http.StatusInternalServerError, "failed to list approvals", "CK_INTERNAL")
	}
	if approvals == nil {
		approvals = []auditmod.ApprovalWithDetails{}
	}
	return c.JSON(http.StatusOK, approvals)
}

// CountPendingApprovals handles GET /api/v1/vaktcomply/approvals/count.
// Returns the number of pending approvals — used for the nav badge.
func (h *Handler) CountPendingApprovals(c echo.Context) error {
	count, err := h.service.Audit.CountPendingApprovals(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("count pending approvals")
		return errResp(c, http.StatusInternalServerError, "failed to count approvals", "CK_INTERNAL")
	}
	return c.JSON(http.StatusOK, map[string]int{"count": count})
}

// ─── Review helpers ───────────────────────────────────────────────────────────

// ReviewCommentInput is the body for approve/reject endpoints.
type ReviewCommentInput struct {
	Comment string `json:"comment" validate:"max=2000"`
}

func (h *Handler) reviewApproval(c echo.Context, approve bool) error {
	approvalID := c.Param("id")

	admin, err := h.isOrgAdmin(c)
	if err != nil {
		log.Error().Err(err).Msg("check admin role for review approval")
		return errResp(c, http.StatusInternalServerError, "role check failed", "CK_INTERNAL")
	}
	if !admin {
		return errResp(c, http.StatusForbidden, "admin role required", "CK_FORBIDDEN")
	}

	var in ReviewCommentInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}

	if err := h.service.Audit.ReviewApproval(
		c.Request().Context(),
		orgID(c), approvalID, userID(c),
		approve, in.Comment,
	); err != nil {
		log.Error().Err(err).Msg("review approval")
		return errResp(c, http.StatusInternalServerError, "failed to review approval", "CK_INTERNAL")
	}

	action := "reject_approval"
	if approve {
		action = "approve_approval"
	}
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID:        orgID(c),
		UserID:       userID(c),
		Action:       action,
		ResourceType: "vakt-comply/control-approval",
		ResourceID:   approvalID,
		IPAddress:    c.RealIP(),
	})

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// ApproveApproval handles POST /api/v1/vaktcomply/approvals/:id/approve.
func (h *Handler) ApproveApproval(c echo.Context) error { return h.reviewApproval(c, true) }

// RejectApproval handles POST /api/v1/vaktcomply/approvals/:id/reject.
func (h *Handler) RejectApproval(c echo.Context) error { return h.reviewApproval(c, false) }

// ─── Org setting ──────────────────────────────────────────────────────────────

// GetApprovalSetting handles GET /api/v1/vaktcomply/org/approval-setting.
func (h *Handler) GetApprovalSetting(c echo.Context) error {
	required, err := h.service.Audit.OrgApprovalRequired(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get approval setting")
		return errResp(c, http.StatusInternalServerError, "failed to get setting", "CK_INTERNAL")
	}
	return c.JSON(http.StatusOK, map[string]bool{"approval_required": required})
}

// UpdateApprovalSettingInput is the body for PUT /api/v1/vaktcomply/org/approval-setting.
type UpdateApprovalSettingInput struct {
	ApprovalRequired bool `json:"approval_required"`
}

// UpdateApprovalSetting handles PUT /api/v1/vaktcomply/org/approval-setting.
// Admin-only: toggles the 4-eyes requirement for the org.
func (h *Handler) UpdateApprovalSetting(c echo.Context) error {
	admin, err := h.isOrgAdmin(c)
	if err != nil {
		log.Error().Err(err).Msg("check admin role for update approval setting")
		return errResp(c, http.StatusInternalServerError, "role check failed", "CK_INTERNAL")
	}
	if !admin {
		return errResp(c, http.StatusForbidden, "admin role required", "CK_FORBIDDEN")
	}

	var in UpdateApprovalSettingInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}

	if err := h.service.Audit.SetOrgApprovalRequired(c.Request().Context(), orgID(c), in.ApprovalRequired); err != nil {
		log.Error().Err(err).Msg("update approval setting")
		return errResp(c, http.StatusInternalServerError, "failed to update setting", "CK_INTERNAL")
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
