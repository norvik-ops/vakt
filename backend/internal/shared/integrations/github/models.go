package github

import "time"

// Integration represents a GitHub repository integration for an organisation.
type Integration struct {
	ID           string     `json:"id"`
	OrgID        string     `json:"org_id"`
	RepoOwner    string     `json:"repo_owner"`
	RepoName     string     `json:"repo_name"`
	LastSyncedAt *time.Time `json:"last_synced_at,omitempty"`
	SyncStatus   string     `json:"sync_status"`
	SyncError    string     `json:"sync_error,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// StoredCheckResult represents a persisted check result from integrations_github_checks.
type StoredCheckResult struct {
	ID            string         `json:"id"`
	IntegrationID string         `json:"integration_id"`
	CheckType     string         `json:"check_type"`
	Status        string         `json:"status"`
	Details       map[string]any `json:"details,omitempty"`
	CheckedAt     time.Time      `json:"checked_at"`
}

// AddIntegrationInput holds the input fields for creating a new GitHub integration.
type AddIntegrationInput struct {
	RepoOwner   string `json:"repo_owner"   validate:"required"`
	RepoName    string `json:"repo_name"    validate:"required"`
	AccessToken string `json:"access_token" validate:"required"`
}
