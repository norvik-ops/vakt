// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"fmt"

	"github.com/matharnica/vakt/internal/db"
)

// GetMyTaskControls returns controls owned by a user in an org (by display name).
func (r *Repository) GetMyTaskControls(ctx context.Context, orgID, ownerDisplayName string) ([]MyTask, error) {
	rows, err := r.q.ListCKMyTaskControls(ctx, db.ListCKMyTaskControlsParams{
		OrgID: orgID,
		Owner: ownerDisplayName,
	})
	if err != nil {
		return nil, fmt.Errorf("list my task controls: %w", err)
	}
	tasks := make([]MyTask, 0, len(rows))
	for _, row := range rows {
		tasks = append(tasks, MyTask{
			ID:          row.ID,
			Title:       row.Title,
			Type:        "control",
			Status:      row.ManualStatus,
			FrameworkID: row.FrameworkID,
		})
	}
	return tasks, nil
}

// GetMyTaskRisks returns risks owned by a user in an org (by display name).
func (r *Repository) GetMyTaskRisks(ctx context.Context, orgID, ownerDisplayName string) ([]MyTask, error) {
	rows, err := r.q.ListCKMyTaskRisks(ctx, db.ListCKMyTaskRisksParams{
		OrgID: orgID,
		Owner: ownerDisplayName,
	})
	if err != nil {
		return nil, fmt.Errorf("list my task risks: %w", err)
	}
	tasks := make([]MyTask, 0, len(rows))
	for _, row := range rows {
		tasks = append(tasks, MyTask{
			ID:     row.ID,
			Title:  row.Title,
			Type:   "risk",
			Status: row.Status,
		})
	}
	return tasks, nil
}
