// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package risklevel maps the 5×5 risk score (likelihood × impact, 1–25) to a
// risk level. It is the backend twin of frontend/src/shared/utils/riskLevel.ts:
// both cut at 15 / 10 / 5 (R1-KW-17). Gross and residual risk use the same cut.
//
// Not covered here: the BSI 200-3 risk analysis (gering/mittel/hoch/sehr hoch)
// is a separate method with its own levels and does not use this score.
package risklevel

// Lower bounds of the levels. SQL that counts critical risks passes
// CriticalFrom as a parameter instead of a literal.
const (
	CriticalFrom = 15
	HighFrom     = 10
	MediumFrom   = 5
)

// Level is a risk level of the risk register.
type Level string

const (
	Critical Level = "critical" // 15–25
	High     Level = "high"     // 10–14
	Medium   Level = "medium"   // 5–9
	Low      Level = "low"      // 1–4
)

// Of returns the level for a risk score.
func Of(score int) Level {
	switch {
	case score >= CriticalFrom:
		return Critical
	case score >= HighFrom:
		return High
	case score >= MediumFrom:
		return Medium
	default:
		return Low
	}
}
