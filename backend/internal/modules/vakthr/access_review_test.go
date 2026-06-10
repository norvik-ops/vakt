// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vakthr

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Gap 4 — vakthr Offboarding → vaktcomply Access-Review-Kampagne
//
// The AccessReviewTrigger interface is injected into Service via
// WithAccessReviewTrigger. The real implementation lives in vaktcomply
// (HRAccessReviewTrigger); the noop is used when vaktcomply is disabled.
// ---------------------------------------------------------------------------

// captureAccessReviewTrigger records calls for pure-Go assertion.
type captureAccessReviewTrigger struct {
	calls []OffboardingReviewInput
}

func (c *captureAccessReviewTrigger) TriggerOffboardingReview(_ context.Context, in OffboardingReviewInput) error {
	c.calls = append(c.calls, in)
	return nil
}

// TestNoopAccessReviewTrigger_ReturnsNil verifies that the noop satisfies the
// interface and never returns an error.
func TestNoopAccessReviewTrigger_ReturnsNil(t *testing.T) {
	noop := &NoopAccessReviewTrigger{}
	err := noop.TriggerOffboardingReview(context.Background(), OffboardingReviewInput{
		OrgID:       "org-1",
		RunID:       "run-1",
		Department:  "Engineering",
		CompletedAt: time.Now(),
	})
	assert.NoError(t, err)
}

// TestWithAccessReviewTrigger_NilFallsBackToNoop verifies that passing nil to
// WithAccessReviewTrigger does not panic and leaves the service using the noop.
func TestWithAccessReviewTrigger_NilFallsBackToNoop(t *testing.T) {
	svc := &Service{accessReview: &NoopAccessReviewTrigger{}}
	svc.WithAccessReviewTrigger(nil)
	require.NotNil(t, svc.accessReview)
	// Calling the noop must not panic.
	err := svc.accessReview.TriggerOffboardingReview(context.Background(), OffboardingReviewInput{})
	assert.NoError(t, err)
}

// TestWithAccessReviewTrigger_SetsImplementation verifies that a real trigger
// replaces the noop after injection.
func TestWithAccessReviewTrigger_SetsImplementation(t *testing.T) {
	cap := &captureAccessReviewTrigger{}
	svc := &Service{accessReview: &NoopAccessReviewTrigger{}}
	svc.WithAccessReviewTrigger(cap)
	assert.Same(t, cap, svc.accessReview)
}

// TestOffboardingReviewInput_ContainsRequiredFields ensures the input struct
// carries OrgID, RunID, Department and CompletedAt — the vaktcomply
// implementation needs all four to create a meaningful campaign.
func TestOffboardingReviewInput_ContainsRequiredFields(t *testing.T) {
	now := time.Now().UTC()
	in := OffboardingReviewInput{
		OrgID:       "org-123",
		RunID:       "run-456",
		Department:  "Finance",
		CompletedAt: now,
	}
	assert.Equal(t, "org-123", in.OrgID)
	assert.Equal(t, "run-456", in.RunID)
	assert.Equal(t, "Finance", in.Department)
	assert.Equal(t, now, in.CompletedAt)
}

// TestOffboardingTrigger_OnlyCalledForOffboardingType_RequiresIntegrationTest
// documents that fireCompletionEvidence calls TriggerOffboardingReview only
// when checklist.Type == "offboarding". The condition is in service.go line
//
//	~370. Requires a live PostgreSQL connection to exercise via CompleteStep.
func TestOffboardingTrigger_OnlyCalledForOffboardingType_RequiresIntegrationTest(t *testing.T) {
	t.Skip("INTEGRATION: fireCompletionEvidence branches on checklist.Type == \"offboarding\". " +
		"Requires live PostgreSQL to exercise via CompleteStep. " +
		"Add to integration test suite: complete an onboarding run → no access review created; " +
		"complete an offboarding run → access review campaign created in vaktcomply.")
}
