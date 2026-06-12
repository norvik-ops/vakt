// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vakthr

import (
	sharedevents "github.com/matharnica/vakt/internal/shared/events"
)

// AccessReviewTrigger is called when an offboarding checklist run completes.
// Definition lives in internal/shared/events — type alias for backward compatibility.
type AccessReviewTrigger = sharedevents.AccessReviewTrigger

// OffboardingReviewInput carries the context for a triggered access review.
// Definition lives in internal/shared/events — type alias for backward compatibility.
type OffboardingReviewInput = sharedevents.OffboardingReviewInput

// NoopAccessReviewTrigger satisfies AccessReviewTrigger without doing anything.
// Definition lives in internal/shared/events — type alias for backward compatibility.
type NoopAccessReviewTrigger = sharedevents.NoopAccessReviewTrigger
