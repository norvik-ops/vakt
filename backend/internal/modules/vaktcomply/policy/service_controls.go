// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy

import "context"

// BulkUpdateControlStatus sets manual_status for multiple controls in a single query.
func (s *Service) BulkUpdateControlStatus(ctx context.Context, orgID string, ids []string, status string) error {
	return s.repo.BulkUpdateControlStatus(ctx, orgID, ids, status)
}
