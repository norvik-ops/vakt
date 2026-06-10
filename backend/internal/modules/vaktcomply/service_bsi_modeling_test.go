// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSuggestedBausteine_KnownTypes(t *testing.T) {
	svc := &Service{}

	tests := []struct {
		assetType string
		expected  []string
	}{
		{"server", []string{"SYS.1.1", "OPS.1.1.3", "ISMS.1"}},
		{"workstation", []string{"SYS.2.1", "OPS.1.2.3", "ISMS.1"}},
		{"network", []string{"NET.1.1", "NET.3.1", "ISMS.1"}},
		{"application", []string{"APP.1.1", "OPS.1.1.5", "ISMS.1"}},
		{"database", []string{"APP.4.3", "OPS.1.1.3", "ISMS.1"}},
	}

	for _, tt := range tests {
		t.Run(tt.assetType, func(t *testing.T) {
			got := svc.GetSuggestedBausteine(tt.assetType)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestGetSuggestedBausteine_UnknownType(t *testing.T) {
	svc := &Service{}
	got := svc.GetSuggestedBausteine("unknown-type")
	require.Len(t, got, 1)
	assert.Equal(t, "ISMS.1", got[0])
}

func TestGetSuggestedBausteine_EmptyString(t *testing.T) {
	svc := &Service{}
	got := svc.GetSuggestedBausteine("")
	require.Len(t, got, 1)
	assert.Equal(t, "ISMS.1", got[0])
}

func TestCreateBSIModelingInput_Validation(t *testing.T) {
	validate := validator.New()

	t.Run("valid R1", func(t *testing.T) {
		in := CreateBSIModelingInput{
			AssetID:   "00000000-0000-0000-0000-000000000001",
			ControlID: "00000000-0000-0000-0000-000000000002",
			Priority:  "R1",
		}
		assert.NoError(t, validate.Struct(in))
	})

	t.Run("valid R2", func(t *testing.T) {
		in := CreateBSIModelingInput{
			AssetID:   "a",
			ControlID: "b",
			Priority:  "R2",
		}
		assert.NoError(t, validate.Struct(in))
	})

	t.Run("valid R3", func(t *testing.T) {
		in := CreateBSIModelingInput{
			AssetID:   "a",
			ControlID: "b",
			Priority:  "R3",
		}
		assert.NoError(t, validate.Struct(in))
	})

	t.Run("invalid priority", func(t *testing.T) {
		in := CreateBSIModelingInput{
			AssetID:   "a",
			ControlID: "b",
			Priority:  "R4",
		}
		assert.Error(t, validate.Struct(in))
	})

	t.Run("missing asset_id", func(t *testing.T) {
		in := CreateBSIModelingInput{
			ControlID: "b",
			Priority:  "R1",
		}
		assert.Error(t, validate.Struct(in))
	})

	t.Run("missing control_id", func(t *testing.T) {
		in := CreateBSIModelingInput{
			AssetID:  "a",
			Priority: "R1",
		}
		assert.Error(t, validate.Struct(in))
	})
}

func TestUpdateBSIModelingInput_CheckStatusValidation(t *testing.T) {
	validate := validator.New()

	validStatuses := []string{"yes", "partial", "no", "not_applicable"}
	for _, s := range validStatuses {
		s := s
		t.Run("valid_"+s, func(t *testing.T) {
			cs := s
			in := UpdateBSIModelingInput{
				Priority:    "R1",
				CheckStatus: &cs,
			}
			assert.NoError(t, validate.Struct(in))
		})
	}

	t.Run("invalid check_status", func(t *testing.T) {
		cs := "unknown"
		in := UpdateBSIModelingInput{
			Priority:    "R1",
			CheckStatus: &cs,
		}
		assert.Error(t, validate.Struct(in))
	})

	t.Run("nil check_status is valid", func(t *testing.T) {
		in := UpdateBSIModelingInput{
			Priority:    "R2",
			CheckStatus: nil,
		}
		assert.NoError(t, validate.Struct(in))
	})
}
