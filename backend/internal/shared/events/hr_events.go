// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package events contains shared cross-module event interfaces and types.
// Modules must not import each other directly (CLAUDE.md module isolation rule);
// shared event types live here so both producer and consumer reference the same
// definition without creating a circular or cross-module dependency.
package events

import (
	"context"
	"time"
)

// AccessReviewTrigger is called when an offboarding checklist run completes.
// The real implementation lives in vaktcomply (HRAccessReviewTrigger); the noop
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
