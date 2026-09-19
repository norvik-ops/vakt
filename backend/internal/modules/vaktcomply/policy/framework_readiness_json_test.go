// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFramework_ReadinessScoreAlwaysSerialized guards R1-G-38.
//
// ReadinessScore carried `omitempty`, so a framework at exactly 0 % readiness —
// the initial state of every newly enabled framework before any evidence — was
// dropped from the JSON entirely. The Auditor-Portal then rendered "Bereitschaft
// %" with no number, which is the whole reason an external auditor opens it.
// The field must be present for every value, 0 included.
func TestFramework_ReadinessScoreAlwaysSerialized(t *testing.T) {
	raw, err := json.Marshal(Framework{Name: "ISO 27001", ReadinessScore: 0})
	require.NoError(t, err)

	var m map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &m))

	got, present := m["readiness_score"]
	assert.True(t, present, "readiness_score must be serialized even at 0 %% (R1-G-38)")
	assert.Equal(t, "0", string(got))

	// A non-zero value round-trips as well.
	raw, err = json.Marshal(Framework{Name: "ISO 27001", ReadinessScore: 42.5})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &m))
	assert.Equal(t, "42.5", string(m["readiness_score"]))
}
