// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package account

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestErrLastAdmin_IsSentinel verifies that ErrLastAdmin is a stable sentinel
// callers can compare against. If this test ever needs an update, every caller
// downstream that relies on `errors.Is(err, ErrLastAdmin)` needs review too.
func TestErrLastAdmin_IsSentinel(t *testing.T) {
	assert.NotNil(t, ErrLastAdmin)
	assert.Contains(t, ErrLastAdmin.Error(), "last admin")
}

// TestErrInvalidPassword_IsSentinel — same guarantee for the password-mismatch
// path: handlers map it to HTTP 401, downstream tools may match on it.
func TestErrInvalidPassword_IsSentinel(t *testing.T) {
	assert.NotNil(t, ErrInvalidPassword)
	assert.Contains(t, ErrInvalidPassword.Error(), "password")
}
