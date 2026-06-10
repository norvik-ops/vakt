// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth_test

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"

	"github.com/matharnica/vakt/internal/auth"
)

// TestHumanValidationError_NeverLeaksRawValidatorString ensures that raw
// go-playground/validator output (e.g. "Key: 'Password' Error:Field validation
// for 'Password' failed on the 'min' tag") never reaches the API response.
// This is a P1 security-UX invariant: raw struct/tag names must not leak.
func TestHumanValidationError_NeverLeaksRawValidatorString(t *testing.T) {
	v := validator.New()

	type loginBody struct {
		Email    string `validate:"required,email"`
		Password string `validate:"required,min=10,max=72"`
	}

	cases := []struct {
		name     string
		input    any
		wantNot  []string // substrings that must NOT appear (raw validator noise)
		wantSome string   // non-empty string that must appear
	}{
		{
			name:    "password too short",
			input:   loginBody{Email: "a@b.de", Password: "short"},
			wantNot: []string{"Key:", "Error:Field validation", "failed on the", "'min' tag", "loginBody"},
			wantSome: "10",
		},
		{
			name:    "invalid email",
			input:   loginBody{Email: "not-an-email", Password: "validpassword"},
			wantNot: []string{"Key:", "Error:Field validation", "failed on the", "'email' tag"},
			wantSome: "E-Mail",
		},
		{
			name:    "missing required password",
			input:   loginBody{Email: "a@b.de", Password: ""},
			wantNot: []string{"Key:", "Error:Field validation", "failed on the"},
			wantSome: "Passwort",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rawErr := v.Struct(tc.input)
			if rawErr == nil {
				t.Fatal("expected validation error, got nil")
			}

			msg := auth.HumanValidationErrorForTest(rawErr)

			assert.NotEmpty(t, msg)
			assert.Contains(t, msg, tc.wantSome,
				"human message should contain expected hint")

			for _, forbidden := range tc.wantNot {
				assert.NotContains(t, msg, forbidden,
					"raw validator noise must not appear in user-facing message")
			}
		})
	}
}

// TestHumanValidationError_NonValidatorError ensures non-validator errors also
// return a safe generic message rather than panicking or leaking internals.
func TestHumanValidationError_NonValidatorError(t *testing.T) {
	msg := auth.HumanValidationErrorForTest(assert.AnError)
	assert.NotEmpty(t, msg)
	assert.NotContains(t, msg, "assert.AnError")
}
