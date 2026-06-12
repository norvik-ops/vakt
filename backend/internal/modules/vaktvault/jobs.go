package vaktvault

// Job type constants for SecVault Asynq tasks.
const (
	TaskGitScan               = "vaktvault:git_scan"
	TaskQuarterlyAccessReview = "vault:quarterly_access_review"

	// Queue is the dedicated Asynq queue for Vakt Vault background jobs.
	Queue = "vaktvault"
)
