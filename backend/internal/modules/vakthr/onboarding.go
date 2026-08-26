// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vakthr

import (
	sharedevents "github.com/matharnica/vakt/internal/shared/events"
)

// EmployeeOnboardingTrigger is called when a new employee record is created.
// Definition lives in internal/shared/events — type alias, same shape as
// AccessReviewTrigger in access_review.go.
type EmployeeOnboardingTrigger = sharedevents.EmployeeOnboardingTrigger

// NewEmployeeInput carries the context of a newly created employee.
// Definition lives in internal/shared/events — type alias.
type NewEmployeeInput = sharedevents.NewEmployeeInput

// NoopEmployeeOnboardingTrigger satisfies EmployeeOnboardingTrigger without
// doing anything. Definition lives in internal/shared/events — type alias.
type NoopEmployeeOnboardingTrigger = sharedevents.NoopEmployeeOnboardingTrigger
