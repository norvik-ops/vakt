// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package risk

import "errors"

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("not found")

// ErrRiskNotMarkedAccepted is returned when a formal risk acceptance is attempted
// for a risk whose register status is not 'accepted'. The formal acceptance records
// WHO accepted the residual risk and WHY (ISO 27001 6.1.3 / 8.3); the decision that
// the risk is accepted at all is the register status, and it has to exist first.
var ErrRiskNotMarkedAccepted = errors.New("risk status must be 'accepted' before formal acceptance")
