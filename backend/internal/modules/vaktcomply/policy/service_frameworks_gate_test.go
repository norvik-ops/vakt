// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// DORA and TISAX were removed from the offering (2026-07-06): both are marked
// status "draft" in builtinAvailable, so EnableFramework must reject any attempt
// to enable them. The draft check runs before any DB access (see
// service_frameworks.go), so a Service with a nil pool exercises the guard
// without a database.
func TestEnableFramework_RejectsRemovedFrameworks(t *testing.T) {
	s := NewService(nil)
	for _, name := range []string{"DORA", "TISAX", "dora", "tisax"} {
		_, err := s.EnableFramework(context.Background(), "org-test", name, "")
		require.Error(t, err, "enabling %q must be rejected", name)
		require.Contains(t, err.Error(), "draft status",
			"framework %q must be rejected as draft, got: %v", name, err)
	}
}
