// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package risk

import (
	"context"
	"time"
)

// ListRisksPaged returns a page of risks plus the total count.
func (s *Service) ListRisksPaged(ctx context.Context, orgID string, offset, limit int) ([]Risk, int, error) {
	return s.repo.ListRisksPaged(ctx, orgID, offset, limit)
}

// ListRisksCursor returns risks using keyset pagination.
func (s *Service) ListRisksCursor(ctx context.Context, orgID string, cursorID string, cursorTS time.Time, limit int) ([]Risk, error) {
	return s.repo.ListRisksCursor(ctx, orgID, cursorID, cursorTS, limit)
}
