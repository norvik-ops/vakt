// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package main

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/modules/vaktaware"
	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/modules/vakthr"
	"github.com/matharnica/vakt/internal/modules/vaktprivacy"
	"github.com/matharnica/vakt/internal/modules/vaktscan"
	"github.com/matharnica/vakt/internal/modules/vaktvault"
	"github.com/matharnica/vakt/internal/services/alerting"
	"github.com/matharnica/vakt/internal/shared/platform/events"
	"github.com/stretchr/testify/assert"
)

func TestTaskModule(t *testing.T) {
	cases := map[string]string{
		vaktscan.TaskScanTrivy:                "vaktscan",
		vaktaware.TaskSendCampaign:            "vaktaware",
		vaktaware.TaskAutoEnrollment:          "vaktaware",
		vaktvault.TaskGitScan:                 "vaktvault",
		vaktvault.TaskQuarterlyAccessReview:   "vaktvault",
		vaktprivacy.TaskAVVExpiryCheck:        "vaktprivacy",
		vaktprivacy.TaskDSRDeadlineCheck:      "vaktprivacy",
		vakthr.TaskContractorExpiryCheck:      "vakthr",
		vaktcomply.TaskISMSKPISnapshot:        "",
		vaktcomply.TaskEvidenceStalenessCheck: "",
		"retention:run":                       "",
		"bsi:feed_sync":                       "vaktscan",
		alerting.TaskSLAOverdueCheck:          "vaktscan",
		alerting.TaskDSROverdueCheck:          "vaktprivacy",
	}
	for task, want := range cases {
		assert.Equal(t, want, taskModule(task), task)
	}
}

// TestModuleGuardMiddleware covers the R1-KW-15 follow-up: a task queued for a
// switched-off module must not run, ISMS tasks run as before.
func TestModuleGuardMiddleware(t *testing.T) {
	ran := map[string]bool{}
	next := asynq.HandlerFunc(func(_ context.Context, task *asynq.Task) error {
		ran[task.Type()] = true
		return nil
	})
	h := moduleGuardMiddleware(&config.Config{ModulesEnabled: "vaktcomply,vaktscan"})(next)

	for _, typ := range []string{vaktaware.TaskSendCampaign, vaktscan.TaskScanTrivy, vaktcomply.TaskISMSKPISnapshot} {
		assert.NoError(t, h.ProcessTask(context.Background(), asynq.NewTask(typ, nil)))
	}
	assert.False(t, ran[vaktaware.TaskSendCampaign], "Aware ist aus — keine Kampagne")
	assert.True(t, ran[vaktscan.TaskScanTrivy])
	assert.True(t, ran[vaktcomply.TaskISMSKPISnapshot])
}

// TestEvidenceSourceEnabled: cross-module evidence all runs as
// "vaktcomply:record_evidence"; the handler filters by the event source instead.
func TestEvidenceSourceEnabled(t *testing.T) {
	cfg := &config.Config{ModulesEnabled: "vaktcomply"}
	assert.False(t, evidenceSourceEnabled(cfg, events.SourceSecreflex), "Aware aus")
	assert.False(t, evidenceSourceEnabled(cfg, events.SourceSecpulse), "Scan aus")
	assert.True(t, evidenceSourceEnabled(cfg, events.SourceSecvitals), "Comply ist Kern")
	assert.True(t, evidenceSourceEnabled(nil, events.SourceSecreflex))
}
