// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vakthr

import (
	"context"
	"time"
)

// AccessReviewTrigger is called when an offboarding checklist run completes.
// The real implementation lives in vaktcomply (injected at startup); the noop
// is used in tests and when vaktcomply is disabled.
type AccessReviewTrigger interface {
	TriggerOffboardingReview(ctx context.Context, in OffboardingReviewInput) error
}

// OffboardingReviewInput carries the context for a triggered access review.
type OffboardingReviewInput struct {
	OrgID       string
	RunID       string
	Department  string
	CompletedAt time.Time
}

// NoopAccessReviewTrigger satisfies AccessReviewTrigger without doing anything.
type NoopAccessReviewTrigger struct{}

func (n *NoopAccessReviewTrigger) TriggerOffboardingReview(_ context.Context, _ OffboardingReviewInput) error {
	return nil
}
