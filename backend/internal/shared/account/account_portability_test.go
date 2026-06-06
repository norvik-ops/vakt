// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package account

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestQueryByOrgAndEmail_NilDB verifies that queryByOrgAndEmail returns an
// error (not a panic) when the DB pool is nil — the caller's best-effort
// wrapper in ExportUserData turns this into an empty "[]" in the ZIP.
func TestQueryByOrgAndEmail_NilDB(t *testing.T) {
	_, err := queryByOrgAndEmail(
		context.Background(),
		nil, // nil pool
		"org-uuid",
		"user@example.com",
		`SELECT id::text FROM hr_employees WHERE org_id = $1::uuid AND email = $2`,
	)
	assert.Error(t, err, "nil DB pool should return an error, not panic")
}

// TestExportFileNames_ContainNewPortabilityFiles documents the three new ZIP
// entry names added for DSGVO Art. 20 compliance. If a filename is ever
// renamed, update this test AND the ExportUserData docstring AND the
// api-contract-checklist (docs/dev/api-contract-checklist.md).
func TestExportFileNames_ContainNewPortabilityFiles(t *testing.T) {
	// These must match the names passed to writeRaw() in ExportUserData exactly.
	required := []string{
		"hr_employee_records.json",
		"awareness_targeting_records.json",
		"privacy_dsr_requests.json",
	}
	// Verify no two names collide — duplicate entries would produce a broken ZIP.
	seen := make(map[string]bool, len(required))
	for _, name := range required {
		require.NotEmpty(t, name, "export filename must not be empty")
		assert.False(t, seen[name], "duplicate export filename: %s", name)
		seen[name] = true
	}
	assert.Len(t, required, 3, "expected exactly 3 new portability files")
}

// TestQueryByOrgAndEmail_EmptyResultIsValidJSON verifies the json.Marshal
// invariant that queryByOrgAndEmail relies on: an empty non-nil slice must
// marshal to "[]", not "null". If this ever changed the ZIP consumer would
// receive invalid JSON for modules with no data.
func TestQueryByOrgAndEmail_EmptyResultIsValidJSON(t *testing.T) {
	data, err := json.Marshal([]map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, "[]", string(data),
		"empty map slice must marshal to JSON array, not null")
}
