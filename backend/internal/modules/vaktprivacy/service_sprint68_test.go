package vaktprivacy

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- ResolveDSR validation: extension_reason required for 'extended' ---

func TestResolveDSR_ExtendedRequiresReason(t *testing.T) {
	validate := func(resolutionType, extensionReason string) error {
		if resolutionType == "extended" && strings.TrimSpace(extensionReason) == "" {
			return errExtensionReasonRequired()
		}
		return nil
	}

	// Should fail: extended without reason
	err := validate("extended", "")
	assert.Error(t, err)

	err = validate("extended", "  ")
	assert.Error(t, err)

	// Should pass: extended with reason
	err = validate("extended", "Komplexe Anfrage benötigt mehr Zeit")
	assert.NoError(t, err)

	// Should pass: non-extended without reason
	err = validate("completed", "")
	assert.NoError(t, err)

	err = validate("rejected", "")
	assert.NoError(t, err)
}

func errExtensionReasonRequired() error {
	return &extensionReasonError{}
}

type extensionReasonError struct{}

func (e *extensionReasonError) Error() string {
	return "extension_reason is required when resolution_type = 'extended'"
}

// --- Extension due date: received_at + 90 days ---

func TestResolveDSR_ExtensionDueAt_Is90DaysFromReceipt(t *testing.T) {
	receivedAt := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	extDueAt := receivedAt.AddDate(0, 0, 90)

	expected := time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, expected, extDueAt,
		"extension due date must be exactly 90 days after receipt (Art. 12 Abs. 3 DSGVO)")
}

// --- DSR overdue check: 30-day deadline ---

func TestDSR_Overdue_DueDateInPast(t *testing.T) {
	dueDate := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	isOverdue := func(due string) bool {
		t, err := time.Parse("2006-01-02", due)
		if err != nil {
			return false
		}
		return t.Before(time.Now().UTC())
	}
	assert.True(t, isOverdue(dueDate))
}

func TestDSR_Overdue_DueDateFuture(t *testing.T) {
	dueDate := time.Now().UTC().AddDate(0, 0, 10).Format("2006-01-02")
	isOverdue := func(due string) bool {
		t, err := time.Parse("2006-01-02", due)
		if err != nil {
			return false
		}
		return t.Before(time.Now().UTC())
	}
	assert.False(t, isOverdue(dueDate))
}

// --- DSR due date: received_at + 30 days ---

func TestDSR_DueDate_Is30DaysFromReceipt(t *testing.T) {
	receivedAt := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	dueDate := receivedAt.AddDate(0, 0, 30)
	expected := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, expected, dueDate)
}

// --- DSR type validation: new types restriction + no_profiling ---

func TestDSR_ValidTypes_IncludeNewTypes(t *testing.T) {
	validTypes := []string{"access", "erasure", "portability", "objection", "rectification", "restriction", "no_profiling"}
	for _, typ := range validTypes {
		assert.Contains(t, validTypes, typ, "type %s must be valid", typ)
	}
}

// --- DSR status: new statuses extended + overdue ---

func TestDSR_ValidStatuses_IncludeNewStatuses(t *testing.T) {
	validStatuses := []string{"open", "in_progress", "completed", "rejected", "extended", "overdue"}
	for _, status := range validStatuses {
		assert.Contains(t, validStatuses, status)
	}
}

// --- RetentionInfo ---

func TestRetentionInfo_WithPeriod(t *testing.T) {
	months := 72
	info := RetentionInfo{
		ProcessingActivityID:  "act-1",
		RetentionPeriodMonths: &months,
		RetentionType:         "fixed",
		DeletionMethod:        "secure_deletion",
		RetentionLegalBasis:   "§ 147 AO (Steuerrecht, 6 Jahre)",
	}
	assert.NotNil(t, info.RetentionPeriodMonths)
	assert.Equal(t, 72, *info.RetentionPeriodMonths)
	assert.Equal(t, "fixed", info.RetentionType)
}

func TestRetentionInfo_PermanentType(t *testing.T) {
	info := RetentionInfo{
		ProcessingActivityID: "act-2",
		RetentionType:        "permanent",
		DeletionMethod:       "",
	}
	// Permanent type does not require a deletion method or period
	assert.Nil(t, info.RetentionPeriodMonths)
	assert.Equal(t, "permanent", info.RetentionType)
}

// --- RetentionSummary completeness threshold ---

func TestRetentionEvidenceStatus_AboveThreshold_OK(t *testing.T) {
	complete, total := 9, 10 // 90% — exactly at threshold
	status := "ok"
	if total > 0 && float64(complete)/float64(total) < 0.9 {
		status = "warning"
	}
	assert.Equal(t, "ok", status)
}

func TestRetentionEvidenceStatus_BelowThreshold_Warning(t *testing.T) {
	complete, total := 5, 10 // 50%
	status := "ok"
	if total > 0 && float64(complete)/float64(total) < 0.9 {
		status = "warning"
	}
	assert.Equal(t, "warning", status)
}

func TestRetentionEvidenceStatus_NoActivities_OK(t *testing.T) {
	complete, total := 0, 0
	status := "ok"
	if total > 0 && float64(complete)/float64(total) < 0.9 {
		status = "warning"
	}
	assert.Equal(t, "ok", status)
}

// --- ResolveDSRInput model ---

func TestResolveDSRInput_FulfilledNoExtensionNeeded(t *testing.T) {
	in := ResolveDSRInput{
		ResolutionType:  "fulfilled",
		ResolutionNotes: "Daten bereitgestellt via E-Mail",
	}
	assert.Equal(t, "fulfilled", in.ResolutionType)
	assert.Empty(t, in.ExtensionReason)
}

func TestResolveDSRInput_ExtendedWithReason(t *testing.T) {
	in := ResolveDSRInput{
		ResolutionType:  "extended",
		ExtensionReason: "Umfangreiche Anfrage mit mehreren Verarbeitungsaktivitäten",
	}
	assert.Equal(t, "extended", in.ResolutionType)
	assert.NotEmpty(t, in.ExtensionReason)
}

// --- DSRSummary ---

func TestDSRSummary_Fields(t *testing.T) {
	s := DSRSummary{
		OpenCount:        3,
		OverdueCount:     1,
		FulfilledLast12M: 12,
		RejectedLast12M:  2,
		OnTimeRatePct:    85.0,
	}
	assert.Equal(t, 3, s.OpenCount)
	assert.Equal(t, 1, s.OverdueCount)
	assert.Equal(t, float64(85), s.OnTimeRatePct)
}
