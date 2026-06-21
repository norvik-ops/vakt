// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package audit

import (
	"context"
	"fmt"
)

// --- Internal Audit Records (FR-CK15) ---

// ListAuditRecords returns all internal audit records for the organisation.
func (s *Service) ListAuditRecords(ctx context.Context, orgID string) ([]AuditRecord, error) {
	records, err := s.repo.ListAuditRecords(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list audit records: %w", err)
	}
	if records == nil {
		records = []AuditRecord{}
	}
	return records, nil
}

// GetAuditRecord returns a single internal audit record by ID.
func (s *Service) GetAuditRecord(ctx context.Context, orgID, id string) (*AuditRecord, error) {
	return s.repo.GetAuditRecord(ctx, orgID, id)
}

// CreateAuditRecord persists a new internal audit record.
func (s *Service) CreateAuditRecord(ctx context.Context, orgID string, in CreateAuditRecordInput) (*AuditRecord, error) {
	return s.repo.CreateAuditRecord(ctx, orgID, in)
}

// UpdateAuditRecord modifies an existing internal audit record.
func (s *Service) UpdateAuditRecord(ctx context.Context, orgID, id string, in UpdateAuditRecordInput) (*AuditRecord, error) {
	return s.repo.UpdateAuditRecord(ctx, orgID, id, in)
}
