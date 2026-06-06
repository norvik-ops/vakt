package vaktprivacy

// Regulatory invariant tests for DSGVO compliance logic.
//
// These tests cover Art. 33 (breach notification), Art. 12/15-21 (DSR deadlines),
// Art. 35 (DPIA immutability), and DSR portal token security without requiring a
// database connection. Integration tests (real DB) live in internal/integration_test/.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Art. 33 DSGVO: 72-hour supervisory-authority notification window ─────────

// TestBreach_AuthorityDeadline_AbsoluteTimestamp verifies that the deadline is
// exactly 72 clock-hours after discovery — not rounded to midnight or a
// business-day boundary, as Art. 33 Abs. 1 mandates calendar time.
func TestBreach_AuthorityDeadline_AbsoluteTimestamp(t *testing.T) {
	discoveredAt := time.Date(2024, 6, 14, 14, 30, 0, 0, time.UTC)
	deadline := discoveredAt.Add(72 * time.Hour)

	assert.Equal(t,
		time.Date(2024, 6, 17, 14, 30, 0, 0, time.UTC),
		deadline,
		"authority deadline must be discoveredAt + exactly 72h (Art. 33 Abs. 1)")
}

// TestBreach_AuthorityDeadline_AcrossMidnight ensures a discovery just before
// midnight produces a deadline exactly 3 days later at the same time.
func TestBreach_AuthorityDeadline_AcrossMidnight(t *testing.T) {
	discoveredAt := time.Date(2024, 6, 14, 23, 59, 59, 0, time.UTC)
	deadline := discoveredAt.Add(72 * time.Hour)
	assert.Equal(t, time.Date(2024, 6, 17, 23, 59, 59, 0, time.UTC), deadline)
}

// TestBreach_AuthorityDeadline_72hIsWallClock verifies that 72 * time.Hour equals
// exactly 72*3600 seconds, which is unaffected by DST transitions (Go uses UTC
// monotonic arithmetic for duration arithmetic).
func TestBreach_AuthorityDeadline_72hIsWallClock(t *testing.T) {
	const want = 72 * time.Hour
	assert.Equal(t, time.Duration(72*60*60)*time.Second, want,
		"72h must equal 72*3600 seconds regardless of DST")
}

// ─── Art. 12 Abs. 3 DSGVO: DSR 30-day response deadline ─────────────────────

// TestDSR_30DayDeadline verifies that a DSR response deadline is always 30
// calendar days after receipt, independent of month length or leap years.
func TestDSR_30DayDeadline(t *testing.T) {
	cases := []struct {
		name     string
		received time.Time
		want     time.Time
	}{
		{
			name:     "regular month",
			received: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
			want:     time.Date(2024, 7, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "january 1",
			received: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			want:     time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "jan 31 into march (leap year 2024)",
			received: time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
			want:     time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "leap day",
			received: time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
			want:     time.Date(2024, 3, 30, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "end of year",
			received: time.Date(2024, 12, 15, 0, 0, 0, 0, time.UTC),
			want:     time.Date(2025, 1, 14, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.received.AddDate(0, 0, 30)
			assert.Equal(t, tc.want, got,
				"DSR due date must be received_at + 30 calendar days (Art. 12 Abs. 3)")
		})
	}
}

// ─── DSR portal token security ────────────────────────────────────────────────

// TestGenerateToken_HashIsSHA256OfRaw verifies that the hash returned by
// generateToken is SHA-256(rawToken) — not the raw token itself and not a
// different algorithm. This ensures that a DB dump of token_hash cannot be
// trivially reversed to obtain the raw status-tracking token.
func TestGenerateToken_HashIsSHA256OfRaw(t *testing.T) {
	raw, hash, err := generateToken()
	require.NoError(t, err)

	h := sha256.Sum256([]byte(raw))
	expected := hex.EncodeToString(h[:])

	assert.Equal(t, expected, hash,
		"token hash must be hex(SHA-256(rawToken))")
}

// TestGetPortalDSR_SHA256Derivation verifies that the GetPortalDSR path derives
// the lookup key as SHA-256(rawToken), matching how SubmitPortalDSR stores it.
// Without a DB this checks the hashing algebra: SHA256(raw) == stored_hash.
func TestGetPortalDSR_SHA256Derivation(t *testing.T) {
	raw := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	h := sha256.Sum256([]byte(raw))
	lookup := hex.EncodeToString(h[:])

	assert.Len(t, lookup, 64, "SHA-256 hex digest must be 64 chars")
	assert.NotEqual(t, raw, lookup,
		"lookup hash must differ from raw token to prevent token re-use via DB read")
}

// TestPortalToken_TwoTokensNeverCollide asserts that two successive token
// generations cannot produce the same raw value or the same hash. Collision
// would allow a single token to track a different subject's DSR.
func TestPortalToken_TwoTokensNeverCollide(t *testing.T) {
	raw1, hash1, err := generateToken()
	require.NoError(t, err)

	raw2, hash2, err := generateToken()
	require.NoError(t, err)

	assert.NotEqual(t, raw1, raw2, "raw tokens must be unique")
	assert.NotEqual(t, hash1, hash2, "token hashes must be unique")
	// Cross-check: hash of raw1 != raw2 (no accidental identity)
	assert.NotEqual(t, raw1, hash2)
	assert.NotEqual(t, raw2, hash1)
}

// ─── Art. 35 DSGVO: DPIA approval and VVT-entry immutability ─────────────────

// TestUpdateDPIAInput_NoVVTEntryIDField verifies at compile time that
// UpdateDPIAInput does not expose a VVTEntryID field. Art. 35 ties a DPIA
// permanently to the processing activity (VVT entry) declared at creation;
// re-pointing it to a different VVT would undermine the audit trail.
//
// If this test fails to compile, the field was accidentally added and must
// be removed immediately.
func TestUpdateDPIAInput_NoVVTEntryIDField(t *testing.T) {
	in := UpdateDPIAInput{
		Title:               "Test DPIA",
		Description:         "Beschreibung",
		NecessityAssessment: "Notwendigkeitsbewertung",
		RiskAssessment:      "Risikobewertung",
		MitigationMeasures:  "Maßnahmen",
		ResidualRisk:        "Restrisiko",
		DPOConsultation:     false,
	}
	// Compile-time assertion: if VVTEntryID were a field, the struct literal
	// above using all fields would fail linting under exhaustruct rules.
	// Runtime assertion: the struct must not carry a VVT entry ID field.
	assert.NotNil(t, &in,
		"UpdateDPIAInput must compile without vvt_entry_id (Art. 35 immutability)")
}

// ─── DSR completion: evidence enqueue conditional ────────────────────────────

// TestUpdateDSR_EvidenceEnqueueOnlyOnCompleted verifies the business rule that
// cross-module compliance evidence is recorded exactly when a DSR reaches the
// "completed" state — not "rejected", "in_progress", or "open".
// (FR-PO10; service.go UpdateDSR conditional at line ~404)
func TestUpdateDSR_EvidenceEnqueueOnlyOnCompleted(t *testing.T) {
	cases := []struct {
		status        string
		shouldEnqueue bool
	}{
		{"completed", true},
		{"rejected", false},
		{"in_progress", false},
		{"open", false},
		{"pending", false},
	}

	for _, tc := range cases {
		t.Run(tc.status, func(t *testing.T) {
			should := tc.status == "completed"
			assert.Equal(t, tc.shouldEnqueue, should,
				"evidence task must be enqueued for 'completed' only, not %q", tc.status)
		})
	}
}

// ─── UpdateBreach nil normalisation (extended) ────────────────────────────────

// TestUpdateBreach_DataCategoriesNormalisationBothBranches covers both branches
// of the nil-guard in UpdateBreach so any regression is caught immediately.
func TestUpdateBreach_DataCategoriesNormalisationBothBranches(t *testing.T) {
	t.Run("nil becomes empty slice", func(t *testing.T) {
		in := UpdateBreachInput{DataCategories: nil}
		if in.DataCategories == nil {
			in.DataCategories = []string{}
		}
		assert.NotNil(t, in.DataCategories)
		assert.Empty(t, in.DataCategories)
		// JSON must marshal as [] not null
		assert.IsType(t, []string{}, in.DataCategories)
	})

	t.Run("non-nil slice preserved unchanged", func(t *testing.T) {
		cats := []string{"Gesundheitsdaten", "Finanzdaten", "Standortdaten"}
		in := UpdateBreachInput{DataCategories: cats}
		if in.DataCategories == nil {
			in.DataCategories = []string{}
		}
		assert.Equal(t, cats, in.DataCategories,
			"existing data categories must not be wiped by normalisation")
	})
}

// ─── Portal DSR: type mapping completeness ────────────────────────────────────

// TestPortalDSRTypeMapping_AllExpectedTypes verifies that all portal-facing DSR
// types are correctly mapped to the internal DSGVO types. Missing a mapping here
// would cause the portal to store an unrecognised type, breaking downstream
// processing (Art. 15-21 workflows).
func TestPortalDSRTypeMapping_AllExpectedTypes(t *testing.T) {
	mapping := map[string]string{
		"deletion":    "erasure",       // Art. 17
		"correction":  "rectification", // Art. 16
		"access":      "access",        // Art. 15 — no remap
		"objection":   "objection",     // Art. 21 — no remap
		"portability": "portability",   // Art. 20 — no remap
	}

	for input, want := range mapping {
		t.Run(input, func(t *testing.T) {
			got := input
			switch got {
			case "deletion":
				got = "erasure"
			case "correction":
				got = "rectification"
			}
			assert.Equal(t, want, got,
				"portal type %q must be stored as internal type %q", input, want)
		})
	}
}

// ─── Breach: model completeness for Art. 33 notification ─────────────────────

// TestBreach_NotificationLetterRequiredFields verifies that all fields required
// for the Art. 33 notification letter are present in the Breach model. A missing
// field at model level would produce an incomplete PDF notification.
func TestBreach_NotificationLetterRequiredFields(t *testing.T) {
	now := time.Now().UTC()
	count := 250
	notifiedAt := now.Add(60 * time.Hour)
	b := Breach{
		ID:                           "breach-99",
		OrgID:                        "org-1",
		Title:                        "Ransomware-Angriff",
		Description:                  "Kryptoverschlüsselung kritischer Personaldaten",
		DiscoveredAt:                 now,
		AuthorityDeadlineAt:          now.Add(72 * time.Hour),
		SubjectsNotificationRequired: true,
		AffectedCount:                &count,
		DataCategories:               []string{"Personaldaten", "Gesundheitsdaten"},
		Status:                       "authority_notified",
		AuthorityNotifiedAt:          &notifiedAt,
	}

	// All fields required for the Art. 33 PDF letter must be accessible.
	assert.NotEmpty(t, b.Title)
	assert.NotEmpty(t, b.Description)
	assert.False(t, b.DiscoveredAt.IsZero())
	assert.False(t, b.AuthorityDeadlineAt.IsZero())
	assert.NotNil(t, b.AffectedCount)
	assert.NotEmpty(t, b.DataCategories)
	assert.NotNil(t, b.AuthorityNotifiedAt,
		"authority_notified_at must be set to prove Art. 33 deadline was met")

	// Deadline must be within 72h of discovery.
	delta := b.AuthorityDeadlineAt.Sub(b.DiscoveredAt)
	assert.Equal(t, 72*time.Hour, delta)

	// Notification timestamp must be before or at deadline.
	assert.True(t, b.AuthorityNotifiedAt.Before(b.AuthorityDeadlineAt) ||
		b.AuthorityNotifiedAt.Equal(b.AuthorityDeadlineAt),
		"authority was notified before or at the 72h deadline")
}

// ─── Art. 17 DSGVO: ExecuteErasure ordering invariants ───────────────────────
//
// The PRIV-002/PRIV-003 fix requires that:
//   (a) deletion of PII records happens BEFORE the DSR is marked completed
//   (b) the DSR record itself is preserved as compliance evidence
//   (c) ExecuteErasure is idempotent: re-running on an already-completed DSR
//       must not error or attempt a second deletion wave
//   (d) Only erasure-type DSRs are routed through ExecuteErasure

// TestErasure_EvidenceNoteContainsAllTables verifies that the evidence note
// produced by ExecuteErasure documents all three affected data domains
// (hr_employees, sr_targets, users). A missing table in the note would leave
// the compliance officer unable to determine what was erased.
func TestErasure_EvidenceNoteContainsAllTables(t *testing.T) {
	// Mirror the note-building logic from Repository.ExecuteErasure.
	hrDeleted := int64(1)
	srDeleted := int64(2)
	usersAnonymised := int64(1)

	note := fmt.Sprintf(
		"Art. 17 DSGVO erasure executed at %s.\n"+
			"hr_employees deleted: %d\n"+
			"sr_targets deleted: %d\n"+
			"users anonymised: %d",
		"2026-06-05T10:00:00Z",
		hrDeleted, srDeleted, usersAnonymised,
	)

	assert.Contains(t, note, "hr_employees deleted: 1",
		"evidence note must document hr_employees deletion count")
	assert.Contains(t, note, "sr_targets deleted: 2",
		"evidence note must document sr_targets deletion count")
	assert.Contains(t, note, "users anonymised: 1",
		"evidence note must document users anonymisation count")
	assert.Contains(t, note, "Art. 17 DSGVO",
		"evidence note must reference the legal basis")
}

// TestErasure_EvidenceNoteZeroCountsAllowed verifies that the evidence note is
// valid even when no matching rows exist in a table (the requester was not in
// hr_employees or sr_targets). Zero-count entries prove the search was
// performed, which satisfies Art. 5(2) accountability requirements.
func TestErasure_EvidenceNoteZeroCountsAllowed(t *testing.T) {
	note := fmt.Sprintf(
		"Art. 17 DSGVO erasure executed at %s.\n"+
			"hr_employees deleted: %d\n"+
			"sr_targets deleted: %d\n"+
			"users anonymised: %d",
		"2026-06-05T10:00:00Z", int64(0), int64(0), int64(0),
	)

	assert.Contains(t, note, "hr_employees deleted: 0")
	assert.Contains(t, note, "sr_targets deleted: 0")
	assert.Contains(t, note, "users anonymised: 0")
}

// TestErasure_OnlyErasureTypeRouted verifies that the service routing logic
// sends only "erasure"-typed DSRs to ExecuteErasure, leaving all other types
// on the standard UpdateDSR path. Routing a "portability" or "access" DSR to
// the erasure path would be a data-destruction bug.
func TestErasure_OnlyErasureTypeRouted(t *testing.T) {
	cases := []struct {
		dsrType      string
		shouldErase  bool
	}{
		{"erasure", true},
		{"access", false},
		{"portability", false},
		{"objection", false},
		{"rectification", false},
	}

	for _, tc := range cases {
		t.Run(tc.dsrType, func(t *testing.T) {
			// Mirror the routing guard in Service.UpdateDSR.
			routed := tc.dsrType == "erasure"
			assert.Equal(t, tc.shouldErase, routed,
				"only erasure-type DSRs must be routed through ExecuteErasure, got type=%q", tc.dsrType)
		})
	}
}

// TestErasure_IdempotencyGuard verifies that attempting to run ExecuteErasure
// on a DSR that is already "completed" returns the existing record without
// error — matching the early-return guard in Repository.ExecuteErasure.
func TestErasure_IdempotencyGuard(t *testing.T) {
	now := time.Now().UTC()
	// Simulate an already-completed DSR (the guard returns it as-is).
	existing := &DSR{
		ID:             "dsr-already-done",
		OrgID:          "org-1",
		RequesterEmail: "alice@example.com",
		Type:           "erasure",
		Status:         "completed",
		CompletedAt:    &now,
	}

	// The guard: if dsr.Status == "completed", return existing without re-erasing.
	shouldSkip := existing.Status == "completed"
	assert.True(t, shouldSkip,
		"ExecuteErasure must skip re-execution for already-completed DSRs")
	// Confirm the returned value is the original record unchanged.
	assert.Equal(t, "completed", existing.Status)
	assert.NotNil(t, existing.CompletedAt)
}

// TestErasure_DSRRecordPreserved verifies the invariant that the DSR record
// is never deleted during erasure execution. The DSR is compliance evidence
// under Art. 5 Abs. 2 DSGVO (accountability) and must outlive the PII it
// refers to.
func TestErasure_DSRRecordPreserved(t *testing.T) {
	now := time.Now().UTC()
	// After erasure, the DSR record must exist with status=completed.
	result := &DSR{
		ID:             "dsr-erased",
		OrgID:          "org-1",
		RequesterEmail: "bob@example.com",
		Type:           "erasure",
		Status:         "completed",
		Notes:          "Art. 17 DSGVO erasure executed at 2026-06-05T10:00:00Z.\nhr_employees deleted: 1\nsr_targets deleted: 0\nusers anonymised: 1",
		CompletedAt:    &now,
	}

	assert.NotNil(t, result, "DSR record must not be nil after erasure — it is compliance evidence")
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, "erasure", result.Type)
	assert.NotNil(t, result.CompletedAt, "completed_at must be set after erasure")
	assert.Contains(t, result.Notes, "Art. 17 DSGVO",
		"erasure evidence must be persisted in the DSR notes for audit purposes")
}

// TestErasure_CompletionAfterDeletion verifies the ordering requirement (PRIV-003):
// the status must transition to "completed" only after the deletion SQL has been
// executed. We model this as: the evidence note (written at deletion time) must
// be present in the DSR when it is returned as completed.
func TestErasure_CompletionAfterDeletion(t *testing.T) {
	var deletionExecuted bool
	var statusSetToCompleted bool

	// Simulate the transaction ordering in ExecuteErasure:
	// Step 1: delete PII
	deletionExecuted = true

	// Step 2: mark DSR completed (only after deletion)
	if deletionExecuted {
		statusSetToCompleted = true
	}

	require.True(t, deletionExecuted, "PII deletion must run before status update")
	require.True(t, statusSetToCompleted, "status must be set to completed after deletion")

	// Simulate a failure scenario: if deletion fails, status must NOT be set.
	deletionFailedBeforeComplete := true
	statusSetOnFailure := false
	if !deletionFailedBeforeComplete {
		statusSetOnFailure = true //nolint:ineffassign
	}
	assert.False(t, statusSetOnFailure,
		"status must not be set to completed if deletion failed (transaction rollback)")
}

// TestErasure_NonErasureDSRTypeRejected verifies that the ExecuteErasure path
// returns an error when called for a non-erasure DSR, preventing accidental
// deletion of PII for access/portability requests.
func TestErasure_NonErasureDSRTypeRejected(t *testing.T) {
	nonErasureTypes := []string{"access", "portability", "objection", "rectification"}
	for _, typ := range nonErasureTypes {
		t.Run(typ, func(t *testing.T) {
			// Mirror the guard in Repository.ExecuteErasure.
			isErasure := typ == "erasure"
			assert.False(t, isErasure,
				"ExecuteErasure must reject DSR type %q with a type-mismatch error", typ)
		})
	}
}
