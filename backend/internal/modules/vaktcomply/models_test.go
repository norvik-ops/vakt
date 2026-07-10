// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsNotFoundMatchesWrappedISMSScopeError is a regression test for
// GetISMSScope returning 500 instead of a graceful 200-null: the repository
// used to return a disconnected fmt.Errorf("isms scope not found") that
// isNotFound() could never match via errors.Is, so every org without an
// ISMS scope yet saw CK_GET_ISMS_SCOPE_FAILED. The fix wraps ErrNotFound.
func TestIsNotFoundMatchesWrappedISMSScopeError(t *testing.T) {
	err := fmt.Errorf("isms scope not found: %w", ErrNotFound)
	assert.True(t, isNotFound(err))
}
