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

// NewEmployeeInput carries the context of a newly created employee to the
// modules that react to an entry (Eintritt).
//
// It carries the employee id ALREADY RESOLVED, for the same reason SubjectRef
// does (see erasure.go): the consuming module must never look the employee up
// in hr_employees itself. vakthr owns that table; a cross-prefix read here
// would recreate the ordering trap ADR-0079 was shaped to remove.
type NewEmployeeInput struct {
	// OrgID scopes the event to one organisation.
	OrgID string
	// EmployeeID is the hr_employees id of the newly created employee.
	EmployeeID string
}

// EmployeeOnboardingTrigger is called when a new employee record is created.
// The real implementation lives in vaktaware (EnrollmentTrigger), which enrols
// the new employee into the campaigns its active `new_employee` rules point at;
// the noop is used in tests and when vaktaware is disabled.
//
// R1-SA25-01 — why this interface exists at all: vaktaware already had the
// consumer (HandleNewEmployeeEnrollment) and a comment claiming "is called by
// the HR event subscriber when a new employee is created". There was no
// subscriber, and no producer either. The consumer had ZERO callers since it
// was written, so BSI ORP.3's automatic induction of new employees never once
// ran. A comment is not a wiring; only a call is.
type EmployeeOnboardingTrigger interface {
	// TriggerNewEmployeeEnrollment reacts to a new employee entry. It is
	// synchronous and returns its error rather than logging it away: the
	// caller decides whether a failed enrolment is fatal for the entry.
	TriggerNewEmployeeEnrollment(ctx context.Context, in NewEmployeeInput) error
}

// NoopEmployeeOnboardingTrigger satisfies EmployeeOnboardingTrigger without
// doing anything. It is the default when vaktaware is not wired or disabled.
type NoopEmployeeOnboardingTrigger struct{}

func (n *NoopEmployeeOnboardingTrigger) TriggerNewEmployeeEnrollment(_ context.Context, _ NewEmployeeInput) error {
	return nil
}
