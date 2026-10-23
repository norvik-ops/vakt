// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package main

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
)

func TestIsArchivedTask(t *testing.T) {
	cases := map[string]bool{
		"vaktscan:trivy":                      true,
		"vaktaware:send_campaign":             true,
		"vault:git_scan":                      true,
		"privacy:avv_expiry_check":            true,
		"hr:contractor_expiry_check":          true,
		"bsi:feed_sync":                       true,
		"alerting:sla_overdue_check":          true,
		"alerting:dsr_overdue_check":          true,
		"vaktcomply:record_evidence":          true,
		"vaktcomply:ai_evidence_suggestion":   true,
		vaktcomply.TaskISMSKPISnapshot:        false,
		vaktcomply.TaskEvidenceStalenessCheck: false,
		"retention:run":                       false,
		"vaktcomplyx:anything":                false,
	}
	for task, want := range cases {
		assert.Equal(t, want, isArchivedTask(task), task)
	}
}

// TestArchivedTaskMiddleware: a task an archived module queued before the update
// is acknowledged without running; ISMS tasks run as before.
func TestArchivedTaskMiddleware(t *testing.T) {
	ran := map[string]bool{}
	h := archivedTaskMiddleware(asynq.HandlerFunc(func(_ context.Context, task *asynq.Task) error {
		ran[task.Type()] = true
		return nil
	}))
	for _, typ := range []string{"vaktaware:send_campaign", "vaktscan:trivy", vaktcomply.TaskISMSKPISnapshot} {
		assert.NoError(t, h.ProcessTask(context.Background(), asynq.NewTask(typ, nil)))
	}
	assert.False(t, ran["vaktaware:send_campaign"])
	assert.False(t, ran["vaktscan:trivy"])
	assert.True(t, ran[vaktcomply.TaskISMSKPISnapshot])
}
