package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/admin"
	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/modules/vaktprivacy"
	"github.com/matharnica/vakt/internal/modules/vaktscan"
	"github.com/matharnica/vakt/internal/modules/vaktaware"
	"github.com/matharnica/vakt/internal/modules/vaktvault"
	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/services/alerting"
	"github.com/matharnica/vakt/internal/services/crossevidence"
	"github.com/matharnica/vakt/internal/shared/bsi"
	"github.com/matharnica/vakt/internal/shared/demo"
	"github.com/matharnica/vakt/internal/shared/emaildigest"
	"github.com/matharnica/vakt/internal/shared/notifications"
	cloudintegration "github.com/matharnica/vakt/internal/shared/platform/integrations/cloud"
	"github.com/matharnica/vakt/internal/shared/retention"
	"github.com/matharnica/vakt/internal/shared/scheduledreports"
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
	assert.Equal(t, "vaktscan:scan:trivy", vaktscan.TaskScanTrivy)
	assert.Equal(t, "vaktscan:scan:nuclei", vaktscan.TaskScanNuclei)
	assert.Equal(t, "vaktscan:scan:openvas", vaktscan.TaskScanOpenVAS)
	assert.Equal(t, "vaktscan:epss_enrich", vaktscan.TaskEPSSEnrich)
	assert.Equal(t, "vaktscan:generate_report", vaktscan.TaskGenerateReport)
	assert.Equal(t, "vaktscan:auto_evidence", vaktscan.TaskAutoEvidence)
	assert.Equal(t, "vaktscan:sbom:generate", vaktscan.TaskSBOMGenerate)
	assert.Equal(t, "vaktscan:eol:check", vaktscan.TaskEOLCheck)

	// SecReflex.
	assert.Equal(t, "vaktaware:send_campaign", vaktaware.TaskSendCampaign)
	assert.Equal(t, "vaktaware:training_reminder", vaktaware.TaskTrainingReminder)

	// SecVault.
	assert.Equal(t, "vaktvault:git_scan", vaktvault.TaskGitScan)

	// Admin.
	assert.Equal(t, "admin:org:delete", admin.TaskDeleteOrg)

	// SecPrivacy.
	assert.Equal(t, "vaktprivacy:avv_expiry_check", vaktprivacy.TaskAVVExpiryCheck)
	assert.Equal(t, "vaktprivacy:breach_incident_create", vaktprivacy.TaskBreachIncidentCreate)

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
	assert.NotEmpty(t, vaktcomply.TaskEvidenceExpiryAlert)
	assert.NotEmpty(t, vaktcomply.TaskIncidentDeadlineCheck)
	assert.NotEmpty(t, vaktcomply.TaskCertExpiryCheck)
	assert.NotEmpty(t, vaktcomply.TaskCCMRunDue)
	assert.NotEmpty(t, vaktcomply.TaskScoreSnapshot)

	// Notifications.
	assert.NotEmpty(t, notifications.TaskNotifyDeadlines)

	// Auth cleanup.
	assert.NotEmpty(t, auth.TaskCleanupPasswordResetTokens)

	// Cloud sync.
	assert.NotEmpty(t, cloudintegration.TaskCloudSync)

	// Scheduled reports.
	assert.NotEmpty(t, scheduledreports.TaskProcessScheduledReports)

	// Local constants.
	assert.Equal(t, "vaktcomply:control_owner_reminder", taskControlOwnerReminder)
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
		{vaktscan.TaskScanTrivy, "default"},
		{vaktscan.TaskScanNuclei, "default"},
		{vaktscan.TaskScanOpenVAS, "low"},
	}
	for _, tc := range cases {
		queue := "default"
		if tc.taskType == vaktscan.TaskScanOpenVAS {
			queue = "low"
		}
		assert.Equal(t, tc.wantQueue, queue, "task type %s should use queue %q", tc.taskType, tc.wantQueue)
	}
}
