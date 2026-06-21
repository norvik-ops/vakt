// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package audit

import "context"

// --- Control status-change approval workflow (4-eyes) ---
//
// These thin delegation wrappers expose the approval repository operations to
// the root vaktcomply handlers via service.Audit. The handlers own the HTTP and
// RBAC logic; the audit service owns the persistence boundary.

// GetOrgMemberRole returns the role name of a user in an organisation.
// Returns pgx.ErrNoRows if the user is not a member.
func (s *Service) GetOrgMemberRole(ctx context.Context, userID, orgID string) (string, error) {
	return s.repo.GetOrgMemberRole(ctx, userID, orgID)
}

// CreateApprovalRequest inserts a new pending approval request.
func (s *Service) CreateApprovalRequest(
	ctx context.Context,
	orgID, controlID, requestedBy, requestedStatus, currentStatus, comment string,
) (*Approval, error) {
	return s.repo.CreateApprovalRequest(ctx, orgID, controlID, requestedBy, requestedStatus, currentStatus, comment)
}

// ListPendingApprovals returns all pending approvals for an org.
func (s *Service) ListPendingApprovals(ctx context.Context, orgID string) ([]ApprovalWithDetails, error) {
	return s.repo.ListPendingApprovals(ctx, orgID)
}

// CountPendingApprovals returns the number of pending approvals for an org.
func (s *Service) CountPendingApprovals(ctx context.Context, orgID string) (int, error) {
	return s.repo.CountPendingApprovals(ctx, orgID)
}

// ReviewApproval marks an approval as approved or rejected and optionally updates the control status.
func (s *Service) ReviewApproval(
	ctx context.Context,
	orgID, approvalID, reviewerID string,
	approve bool,
	comment string,
) error {
	return s.repo.ReviewApproval(ctx, orgID, approvalID, reviewerID, approve, comment)
}

// OrgApprovalRequired returns whether the organisation requires approval for control status changes.
func (s *Service) OrgApprovalRequired(ctx context.Context, orgID string) (bool, error) {
	return s.repo.OrgApprovalRequired(ctx, orgID)
}

// SetOrgApprovalRequired updates the approval_required flag for an organisation.
func (s *Service) SetOrgApprovalRequired(ctx context.Context, orgID string, required bool) error {
	return s.repo.SetOrgApprovalRequired(ctx, orgID, required)
}
