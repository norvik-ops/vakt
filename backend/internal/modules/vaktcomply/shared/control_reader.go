// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package shared holds cross-sub-package interfaces for vaktcomply.
package shared

import "context"

// ControlReader lets sub-packages read compliance controls without importing the policy package directly.
type ControlReader interface {
	ListControls(ctx context.Context, orgID string) ([]Control, error)
}

// Control is the minimal control shape exposed across sub-packages.
type Control struct {
	ID, FrameworkID, OrgID, ControlID, Title, Domain string
}
