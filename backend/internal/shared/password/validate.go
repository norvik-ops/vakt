// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package password provides shared password-strength validation used across
// the Vakt platform (auth service, invite acceptance, admin CLI).
// The canonical policy lives here so all entry-points are consistent.
package password

import (
	"errors"
	"strings"
	"unicode"
)

// ErrWeakPassword is returned when a password does not satisfy Vakt's minimum
// complexity policy.
var ErrWeakPassword = errors.New("password must be at least 10 characters and contain uppercase, digit, and special character")

// ValidateStrength checks that the password satisfies the Vakt platform policy:
//   - At least 10 characters
//   - At least one uppercase letter (A–Z)
//   - At least one decimal digit (0–9)
//   - At least one special character (!@#$%^&*()-_=+[]{}|;:'",.<>?/`~\)
//
// Returns ErrWeakPassword when any requirement is not satisfied.
func ValidateStrength(password string) error {
	if len(password) < 10 {
		return ErrWeakPassword
	}
	var hasUpper, hasDigit, hasSpecial bool
	const special = "!@#$%^&*()-_=+[]{}|;:'\",.<>?/`~\\"
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		case strings.ContainsRune(special, r):
			hasSpecial = true
		}
	}
	if !hasUpper || !hasDigit || !hasSpecial {
		return ErrWeakPassword
	}
	return nil
}
