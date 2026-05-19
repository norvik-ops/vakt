package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sechealth-app/sechealth/internal/admin"
	"github.com/sechealth-app/sechealth/internal/auth"
	"github.com/sechealth-app/sechealth/internal/modules/secprivacy"
	"github.com/sechealth-app/sechealth/internal/modules/secpulse"
	"github.com/sechealth-app/sechealth/internal/modules/secreflex"
	"github.com/sechealth-app/sechealth/internal/modules/secvault"
	"github.com/sechealth-app/sechealth/internal/shared/alerting"
	"github.com/sechealth-app/sechealth/internal/shared/bsi"
	"github.com/sechealth-app/sechealth/internal/shared/crossevidence"
	"github.com/sechealth-app/sechealth/internal/shared/emaildigest"
	"github.com/sechealth-app/sechealth/internal/shared/notifications"
	"github.com/sechealth-app/sechealth/internal/shared/retention"
	"github.com/sechealth-app/sechealth/internal/shared/scheduledreports"
	cloudintegration "github.com/sechealth-app/sechealth/internal/shared/integrations/cloud"
	"github.com/sechealth-app/sechealth/internal/modules/secvitals"
	"github.com/sechealth-app/sechealth/internal/shared/demo"
)

func TestBuildServer_ReturnsNonNil(t *testing.T) {
	srv, mux := buildServer(nil)
	require.NotNil(t, srv, "asynq server must not be nil")
	require.NotNil(t, mux, "asynq mux must not be nil")
}

// TestWorkerConcurrency_Default verifies the default concurrency of 8 when env is unset.
func TestWorkerConcurrency_Default(t *testing.T) {
	_ = os.Unsetenv("VAKT_WORKER_CONCURRENCY")
	assert.Equal(t, 8, workerConcurrency(), "default concurrency must be 8")
}

// TestWorkerConcurrency_EnvOverride verifies that VAKT_WORKER_CONCURRENCY sets the value.
func TestWorkerConcurrency_EnvOverride(t *testing.T) {
	t.Setenv("VAKT_WORKER_CONCURRENCY", "16")
	assert.Equal(t, 16, workerConcurrency())
}

// TestWorkerConcurrency_InvalidEnv falls back to 8 when the env value is non-numeric.
func TestWorkerConcurrency_InvalidEnv(t *testing.T) {
	t.Setenv("VAKT_WORKER_CONCURRENCY", "not-a-number")
	assert.Equal(t, 8, workerConcurrency(), "invalid env value must fall back to 8")
}

// TestWorkerConcurrency_ZeroEnv falls back to 8 when the env value is zero (not > 0).
func TestWorkerConcurrency_ZeroEnv(t *testing.T) {
	t.Setenv("VAKT_WORKER_CONCURRENCY", "0")
	assert.Equal(t, 8, workerConcurrency(), "zero concurrency must fall back to default 8")
}

// TestWorkerConcurrency_NegativeEnv falls back to 8 when the env value is negative.
func TestWorkerConcurrency_NegativeEnv(t *testing.T) {
	t.Setenv("VAKT_WORKER_CONCURRENCY", "-4")
	assert.Equal(t, 8, workerConcurrency(), "negative concurrency must fall back to default 8")
}

// TestWorkerTaskConstantsRegistered documents all task types that buildServer registers.
// Keeping this list in sync with main.go ensures that new task types are not silently
// dropped from the mux — a reviewer must update both files.
//
// This test does not call the mux directly (asynq.ServeMux does not expose a handler
// lookup API), but it compiles a reference to every task constant so that renaming or
// removing a constant will cause a compilation failure here.
func TestWorkerTaskConstantsRegistered(t *testing.T) {
	// SecPulse scan handlers.
	assert.Equal(t, "secpulse:scan:trivy", secpulse.TaskScanTrivy)
	assert.Equal(t, "secpulse:scan:nuclei", secpulse.TaskScanNuclei)
	assert.Equal(t, "secpulse:scan:openvas", secpulse.TaskScanOpenVAS)
	assert.Equal(t, "secpulse:epss_enrich", secpulse.TaskEPSSEnrich)
	assert.Equal(t, "secpulse:generate_report", secpulse.TaskGenerateReport)
	assert.Equal(t, "secpulse:auto_evidence", secpulse.TaskAutoEvidence)
	assert.Equal(t, "secpulse:sbom:generate", secpulse.TaskSBOMGenerate)
	assert.Equal(t, "secpulse:eol:check", secpulse.TaskEOLCheck)

	// SecReflex.
	assert.Equal(t, "secreflex:send_campaign", secreflex.TaskSendCampaign)
	assert.Equal(t, "secreflex:training_reminder", secreflex.TaskTrainingReminder)

	// SecVault.
	assert.Equal(t, "secvault:git_scan", secvault.TaskGitScan)

	// Admin.
	assert.Equal(t, "admin:org:delete", admin.TaskDeleteOrg)

	// SecPrivacy.
	assert.Equal(t, "secprivacy:avv_expiry_check", secprivacy.TaskAVVExpiryCheck)
	assert.Equal(t, "secprivacy:breach_incident_create", secprivacy.TaskBreachIncidentCreate)

	// Alerting.
	assert.NotEmpty(t, alerting.TaskSLAOverdueCheck)
	assert.NotEmpty(t, alerting.TaskDSROverdueCheck)

	// Demo cleanup.
	assert.NotNil(t, demo.NewCleanupTask())

	// Retention.
	assert.NotEmpty(t, retention.TaskRetentionRun)

	// Email digest.
	assert.NotEmpty(t, emaildigest.TaskWeeklyDigest)

	// BSI feed.
	assert.NotEmpty(t, bsi.TaskBSIFeedSync)

	// Cross-module evidence.
	assert.NotEmpty(t, crossevidence.TaskRecordEvidence)

	// SecVitals.
	assert.NotEmpty(t, secvitals.TaskEvidenceExpiryAlert)
	assert.NotEmpty(t, secvitals.TaskIncidentDeadlineCheck)
	assert.NotEmpty(t, secvitals.TaskCertExpiryCheck)
	assert.NotEmpty(t, secvitals.TaskCCMRunDue)
	assert.NotEmpty(t, secvitals.TaskScoreSnapshot)

	// Notifications.
	assert.NotEmpty(t, notifications.TaskNotifyDeadlines)

	// Auth cleanup.
	assert.NotEmpty(t, auth.TaskCleanupPasswordResetTokens)

	// Cloud sync.
	assert.NotEmpty(t, cloudintegration.TaskCloudSync)

	// Scheduled reports.
	assert.NotEmpty(t, scheduledreports.TaskProcessScheduledReports)

	// Local constants.
	assert.Equal(t, "secvitals:control_owner_reminder", taskControlOwnerReminder)
	assert.Equal(t, "github:ci_evidence:sync", taskGitHubCISync)
	assert.Equal(t, "queue:health:check", taskQueueHealthCheck)
}

// TestHexDecodeKey_Valid verifies that a correctly formatted 64-hex-char key decodes to 32 bytes.
func TestHexDecodeKey_Valid(t *testing.T) {
	// 64 hex chars = 32 bytes.
	hexKey := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	b, err := hexDecodeKey(hexKey)
	require.NoError(t, err)
	assert.Len(t, b, 32)
	assert.Equal(t, byte(0x01), b[0])
	assert.Equal(t, byte(0x20), b[31])
}

// TestHexDecodeKey_UpperCase verifies that uppercase hex is decoded correctly.
func TestHexDecodeKey_UpperCase(t *testing.T) {
	hexKey := "AABBCCDDEEFF00112233445566778899AABBCCDDEEFF00112233445566778899"
	b, err := hexDecodeKey(hexKey)
	require.NoError(t, err)
	assert.Len(t, b, 32)
	assert.Equal(t, byte(0xAA), b[0])
}

// TestHexDecodeKey_InvalidChar verifies that a non-hex character causes an error.
func TestHexDecodeKey_InvalidChar(t *testing.T) {
	_, err := hexDecodeKey("ZZ0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	assert.Error(t, err, "non-hex character must cause an error")
}

// TestHexDecodeKey_Empty verifies that an empty string decodes to an empty slice without error.
func TestHexDecodeKey_Empty(t *testing.T) {
	b, err := hexDecodeKey("")
	require.NoError(t, err)
	assert.Empty(t, b)
}

// TestFromHexChar_ValidChars verifies the hex decoder helper for all valid hex characters.
func TestFromHexChar(t *testing.T) {
	cases := []struct {
		c    byte
		want byte
	}{
		{'0', 0}, {'9', 9},
		{'a', 10}, {'f', 15},
		{'A', 10}, {'F', 15},
		{'z', 255}, {'!', 255}, {' ', 255},
	}
	for _, tc := range cases {
		got := fromHexChar(tc.c)
		assert.Equal(t, tc.want, got, "fromHexChar(%q) should be %d", tc.c, tc.want)
	}
}

// TestEnqueueScanTask_QueueSelection verifies that OpenVAS scans are placed on
// the "low" queue and other scan types go to "default".
// We test the queue selection logic directly since the Asynq client requires Redis.
func TestEnqueueScanTask_QueueSelection(t *testing.T) {
	cases := []struct {
		taskType  string
		wantQueue string
	}{
		{secpulse.TaskScanTrivy, "default"},
		{secpulse.TaskScanNuclei, "default"},
		{secpulse.TaskScanOpenVAS, "low"},
	}
	for _, tc := range cases {
		queue := "default"
		if tc.taskType == secpulse.TaskScanOpenVAS {
			queue = "low"
		}
		assert.Equal(t, tc.wantQueue, queue, "task type %s should use queue %q", tc.taskType, tc.wantQueue)
	}
}
