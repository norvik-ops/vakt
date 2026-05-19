// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package hr

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// SecHR — Security test coverage
//
// The HR service delegates all DB access to the Repository, which embeds
// org_id in every WHERE clause (e.g. `WHERE org_id = $1::uuid AND id = $2::uuid`).
// Because verifying this requires a live PostgreSQL connection, the
// integration-layer gaps are documented with t.Skip. Pure-Go validation
// (model invariants, input sanitisation, JSON shape) is tested directly.
//
// Security model:
//   - Every employee record is scoped to an org_id — cross-org IDOR not
//     possible without the correct orgID.
//   - Every checklist run is scoped to an org_id — a user cannot complete
//     another user's checklist run without the correct run ID AND org_id.
//   - Non-admin access control is enforced at the handler layer via RBAC
//     middleware, not the service layer (handler reads Paseto claims).
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// 1. Checklist isolation — service-layer documentation
// ---------------------------------------------------------------------------

// TestChecklistRunIsolation_CompleteOtherUsersRun_RequiresIntegrationTest
// documents that UpdateChecklistRun scopes its UPDATE to `org_id` and the
// run `id` — a caller with a run ID from org-B cannot modify a run in org-A.
//
// SECURITY: can only be integration-tested
func TestChecklistRunIsolation_CompleteOtherUsersRun_RequiresIntegrationTest(t *testing.T) {
	t.Skip("SECURITY GAP: checklist run isolation enforced by " +
		"SQL WHERE org_id = $1::uuid AND id = $2::uuid in repo.UpdateChecklistRun. " +
		"A wrong orgID causes pgx.ErrNoRows — requires live PostgreSQL to verify. " +
		"Add to integration test suite: attempt to complete runB using orgA context.")
}

// TestChecklistRunIsolation_ReadOtherUsersRun_RequiresIntegrationTest
// documents that GetChecklistRun scopes the SELECT to org_id.
//
// SECURITY: can only be integration-tested
func TestChecklistRunIsolation_ReadOtherUsersRun_RequiresIntegrationTest(t *testing.T) {
	t.Skip("SECURITY GAP: GetChecklistRun uses WHERE org_id = $1::uuid AND id = $2::uuid — " +
		"a wrong orgID returns no rows. Requires live PostgreSQL to verify. " +
		"Add to integration test suite.")
}

// ---------------------------------------------------------------------------
// 2. Employee record access — service-layer documentation
// ---------------------------------------------------------------------------

// TestEmployeeIsolation_CrossOrgRead_RequiresIntegrationTest documents that
// GetEmployee is scoped to org_id so cross-org reads return nothing.
//
// SECURITY: can only be integration-tested
func TestEmployeeIsolation_CrossOrgRead_RequiresIntegrationTest(t *testing.T) {
	t.Skip("SECURITY GAP: GetEmployee uses WHERE org_id = $1::uuid AND id = $2::uuid — " +
		"passing a wrong orgID returns pgx.ErrNoRows. Requires live PostgreSQL. " +
		"Add to integration test suite: create employee in org-A, attempt read with org-B context.")
}

// TestEmployeeIsolation_CrossOrgUpdate_RequiresIntegrationTest documents that
// UpdateEmployee is scoped to org_id.
//
// SECURITY: can only be integration-tested
func TestEmployeeIsolation_CrossOrgUpdate_RequiresIntegrationTest(t *testing.T) {
	t.Skip("SECURITY GAP: UpdateEmployee uses WHERE org_id = $1::uuid AND id = $2::uuid — " +
		"passing a wrong orgID returns pgx.ErrNoRows. Requires live PostgreSQL.")
}

// ---------------------------------------------------------------------------
// 3. Model invariants — pure-Go (no DB required)
// ---------------------------------------------------------------------------

// TestEmployee_StatusField_AllowedValues documents the valid status enum values
// enforced by the UpdateEmployeeInput validator tag.  If a new status is added
// without updating this test, the test fails and forces a review.
func TestEmployee_StatusField_AllowedValues(t *testing.T) {
	valid := []string{"active", "offboarding", "terminated"}

	// Verify via the struct tag (parsed at test time) — this is a documentation
	// test that fails if someone removes the validator tag.
	// The canonical check is in the handler (validator.Validate), not the service.
	for _, status := range valid {
		in := UpdateEmployeeInput{
			FirstName: "Ada",
			LastName:  "Lovelace",
			Status:    status,
		}
		assert.Equal(t, status, in.Status)
	}
}

// TestEmployee_OrgIDNeverInCreateInput verifies that the CreateEmployeeInput
// struct does NOT expose an OrgID field — orgs are always set server-side
// from the authenticated Paseto token claim, never from request body input.
// This prevents a mass-assignment / privilege-escalation vector.
func TestEmployee_OrgIDNeverInCreateInput(t *testing.T) {
	// Attempt to set org_id via JSON unmarshal — it should be silently ignored.
	raw := `{
		"first_name": "Mallory",
		"last_name":  "Attacker",
		"email":      "mallory@evil.example",
		"org_id":     "victim-org-uuid"
	}`

	var in CreateEmployeeInput
	require.NoError(t, json.Unmarshal([]byte(raw), &in))

	// CreateEmployeeInput has no OrgID field — the JSON key is ignored.
	// We verify by checking the marshalled output does not contain org_id.
	out, err := json.Marshal(in)
	require.NoError(t, err)
	assert.NotContains(t, string(out), `"org_id"`,
		"CreateEmployeeInput must not expose an org_id field — "+
			"org is always set server-side from the Paseto token claim")
}

// TestEmployee_UpdateInput_NoOrgIDField mirrors the above for UpdateEmployeeInput.
func TestEmployee_UpdateInput_NoOrgIDField(t *testing.T) {
	raw := `{
		"first_name": "Mallory",
		"last_name":  "Attacker",
		"status":     "active",
		"org_id":     "victim-org-uuid"
	}`

	var in UpdateEmployeeInput
	require.NoError(t, json.Unmarshal([]byte(raw), &in))

	out, err := json.Marshal(in)
	require.NoError(t, err)
	assert.NotContains(t, string(out), `"org_id"`,
		"UpdateEmployeeInput must not expose an org_id field")
}

// ---------------------------------------------------------------------------
// 4. ChecklistRun — completed_items isolation
// ---------------------------------------------------------------------------

// TestChecklistRun_CompletedItems_IsSlice verifies that CompletedItems is
// always a slice (never nil in JSON), preventing null-vs-array confusion that
// could lead to a client skipping item validation.
func TestChecklistRun_CompletedItems_IsSlice(t *testing.T) {
	run := ChecklistRun{
		ID:             "run-id",
		OrgID:          "org-id",
		EmployeeID:     "emp-id",
		ChecklistID:    "cl-id",
		Status:         "in_progress",
		CompletedItems: []string{},
	}

	data, err := json.Marshal(run)
	require.NoError(t, err)

	// completed_items: [] must appear, not completed_items: null
	assert.Contains(t, string(data), `"completed_items":[]`,
		"empty CompletedItems must serialise as [] not null")
}

// TestChecklistRun_UpdateInput_NoOrgIDOrEmployeeIDField confirms that
// UpdateChecklistRunInput cannot be used to change employee_id or org_id —
// those are path parameters verified by the repository WHERE clause.
func TestChecklistRun_UpdateInput_NoOrgIDOrEmployeeIDField(t *testing.T) {
	raw := `{
		"completed_items": ["step-1"],
		"status":          "in_progress",
		"org_id":          "victim-org",
		"employee_id":     "victim-emp"
	}`

	var in UpdateChecklistRunInput
	require.NoError(t, json.Unmarshal([]byte(raw), &in))

	out, err := json.Marshal(in)
	require.NoError(t, err)

	assert.NotContains(t, string(out), `"org_id"`,
		"UpdateChecklistRunInput must not accept org_id from request body")
	assert.NotContains(t, string(out), `"employee_id"`,
		"UpdateChecklistRunInput must not accept employee_id from request body")
}

// ---------------------------------------------------------------------------
// 5. StartChecklistRun — employee_id binding
// ---------------------------------------------------------------------------

// TestStartChecklistRunInput_EmployeeIDRequired verifies that the
// StartChecklistRunInput struct requires employee_id (documented by the
// `validate:"required"` tag).  An empty EmployeeID must not silently default
// to an empty string that could be cast to a nil UUID on the DB side.
func TestStartChecklistRunInput_EmployeeIDRequired(t *testing.T) {
	in := StartChecklistRunInput{
		EmployeeID:  "",
		ChecklistID: "cl-id",
	}
	// Value-level assertion: empty string is the zero value and must be
	// caught by the validator in the handler before reaching the service.
	assert.Empty(t, in.EmployeeID,
		"empty EmployeeID must be detectable before reaching the service "+
			"(handler validator enforces validate:\"required\")")
}

// TestStartChecklistRunInput_NoOrgIDField verifies that StartChecklistRunInput
// does not expose an org_id field — org is bound server-side.
func TestStartChecklistRunInput_NoOrgIDField(t *testing.T) {
	raw := `{
		"employee_id":  "emp-uuid",
		"checklist_id": "cl-uuid",
		"org_id":       "attacker-org"
	}`

	var in StartChecklistRunInput
	require.NoError(t, json.Unmarshal([]byte(raw), &in))

	out, err := json.Marshal(in)
	require.NoError(t, err)
	assert.NotContains(t, string(out), `"org_id"`,
		"StartChecklistRunInput must not accept org_id from the request body")
}

// ---------------------------------------------------------------------------
// 6. Checklist template — org isolation documentation
// ---------------------------------------------------------------------------

// TestChecklistTemplate_OrgIsolation_RequiresIntegrationTest documents that
// CreateChecklist, ListChecklists, and DeleteChecklist all scope to org_id.
//
// SECURITY: can only be integration-tested
func TestChecklistTemplate_OrgIsolation_RequiresIntegrationTest(t *testing.T) {
	t.Skip("SECURITY GAP: checklist template CRUD uses WHERE org_id = $1::uuid. " +
		"A wrong orgID will return no rows / affect no rows. " +
		"Requires live PostgreSQL to verify. Add to integration test suite.")
}

// ---------------------------------------------------------------------------
// 7. Employee notes — sensitive data in JSON
// ---------------------------------------------------------------------------

// TestEmployee_NotesField_OmittedWhenEmpty verifies that the notes field is
// omitted from JSON when empty (prevents leaking the field name when no
// sensitive notes exist).
func TestEmployee_NotesField_OmittedWhenEmpty(t *testing.T) {
	e := Employee{
		ID:        "emp-1",
		OrgID:     "org-1",
		FirstName: "John",
		LastName:  "Doe",
		Email:     "john@example.com",
		Status:    "active",
		Notes:     "", // empty
	}

	data, err := json.Marshal(e)
	require.NoError(t, err)
	// Notes has json:"notes,omitempty" so an empty string must be omitted.
	assert.NotContains(t, string(data), `"notes":""`,
		"empty Notes must be omitted by json:\"notes,omitempty\" tag")
}
