// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktvault

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAggregateProjectHealth is a regression test for GetProjectHealth
// crashing ProjectDetailPage.tsx in production: the handler used to return
// the raw []SecretHealth (one entry per secret) while the frontend's
// ProjectHealth type — and its `health.issues.length` render code — expects
// a single aggregate {score, issues} object. Any secret in a project made
// `health` an array, so `health.issues` was undefined and the page crashed.
func TestAggregateProjectHealth_NoSecrets(t *testing.T) {
	got := aggregateProjectHealth(nil)
	assert.Equal(t, ProjectHealth{Score: 100, Issues: []string{}}, got)
}

func TestAggregateProjectHealth_AveragesScoresAndLabelsIssues(t *testing.T) {
	got := aggregateProjectHealth([]SecretHealth{
		{Key: "API_KEY", HealthScore: 80, Issues: []string{"secret older than 90 days"}},
		{Key: "DB_PASSWORD", HealthScore: 40, Issues: []string{"not rotated in over 90 days", "never accessed since creation"}},
	})
	assert.Equal(t, 60, got.Score) // (80+40)/2
	assert.Equal(t, []string{
		"API_KEY: secret older than 90 days",
		"DB_PASSWORD: not rotated in over 90 days",
		"DB_PASSWORD: never accessed since creation",
	}, got.Issues)
}
