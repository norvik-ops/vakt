// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package percent rounds a 0–100 share to the whole percent every view shows.
//
// One rounding rule for the dashboard, the framework pages and every PDF:
// half rounds up (12,5 % → 13 %). The PDFs used fmt "%.0f", which rounds half
// to even (12,5 % → 12 %), while the dashboard rounded half up — the same
// readiness printed as two numbers. The frontend mirrors this rule in
// shared/utils/percent.ts (roundPercent).
//
// The small epsilon absorbs float noise: 14 covered + 1 partial of 100 is
// exactly 14,5 %, but (14.5/100)*100 evaluates to 14.499999999999998 and would
// otherwise round down.
package percent

import (
	"fmt"
	"math"
)

// epsilon is far below any real share (1/10000 of a control at 10^5 controls)
// and far above float64 noise around 0–100.
const epsilon = 1e-9

// Round returns the share rounded to a whole percent, half up.
func Round(pct float64) int {
	return int(math.Floor(pct + 0.5 + epsilon))
}

// Format returns the rounded share as "13%".
func Format(pct float64) string {
	return fmt.Sprintf("%d%%", Round(pct))
}
