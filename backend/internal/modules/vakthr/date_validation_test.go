// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vakthr

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/require"
)

// SA14-02: a malformed date used to travel all the way into Postgres, which
// rejected it with SQLSTATE 22007 — surfacing as a 500 for what is plainly a
// client mistake. The fix validates the shape at the handler boundary instead.
//
// This test exercises the real validator against the real struct tags, so it
// fails if a tag is dropped or misspelled. A tag on a struct nobody validates
// would be a no-op; CreateEmployee/UpdateEmployee both call validate.Struct
// (handler.go:122 / the Update counterpart), which is what makes this binding.
func TestEmployeeDateValidation(t *testing.T) {
	v := validator.New()

	t.Run("valid ISO date passes", func(t *testing.T) {
		require.NoError(t, v.Struct(CreateEmployeeInput{
			Email: "a@b.de", FirstName: "A", LastName: "B", StartDate: "2026-07-24",
		}))
	})

	t.Run("empty date passes (field is optional)", func(t *testing.T) {
		require.NoError(t, v.Struct(CreateEmployeeInput{
			Email: "a@b.de", FirstName: "A", LastName: "B", StartDate: "",
		}))
	})

	// Each of these previously reached Postgres and produced a 22007 → 500.
	for _, bad := range []string{
		"24.07.2026", // German format
		"2026-13-01", // month 13
		"2026-02-30", // day that does not exist
		"tomorrow",
		"2026/07/24",
	} {
		t.Run("rejects "+bad, func(t *testing.T) {
			err := v.Struct(CreateEmployeeInput{
				Email: "a@b.de", FirstName: "A", LastName: "B", StartDate: bad,
			})
			require.Error(t, err,
				"a malformed date must be rejected at the boundary (422), not become a 22007/500 in Postgres")
		})
	}
}
