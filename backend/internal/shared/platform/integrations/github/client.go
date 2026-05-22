package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client is a minimal GitHub REST API client.
type Client struct {
	token      string
	httpClient *http.Client
}

// NewClient creates a new GitHub API client using the given personal access token.
func NewClient(token string) *Client {
	return &Client{
		token:      token,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// BranchProtection holds branch protection rule information for a single branch.
type BranchProtection struct {
	Enabled              bool `json:"enabled"`
	RequiresPRReviews    bool `json:"requires_pr_reviews"`
	RequiredApprovals    int  `json:"required_approvals"`
	RequiresStatusChecks bool `json:"requires_status_checks"`
	EnforcesAdmins       bool `json:"enforces_admins"`
}

// RepoSecurityStatus holds security feature status for a repository.
type RepoSecurityStatus struct {
	DependencyAlertsEnabled bool `json:"dependency_alerts_enabled"`
	SecretScanningEnabled   bool `json:"secret_scanning_enabled"`
	CodeScanningEnabled     bool `json:"code_scanning_enabled"`
}

// doGitHubRequest performs a GitHub API request and returns the HTTP response.
func (c *Client) doGitHubRequest(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	return resp, nil
}

// GetDefaultBranch fetches the default branch name for a repository.
func (c *Client) GetDefaultBranch(ctx context.Context, owner, repo string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)
	resp, err := c.doGitHubRequest(ctx, url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github api returned %d for repo info", resp.StatusCode)
	}

	var result struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode repo response: %w", err)
	}
	return result.DefaultBranch, nil
}

// GetBranchProtection fetches branch protection rules for a specific branch.
// Returns a BranchProtection with Enabled=false (not an error) when protection is not configured.
func (c *Client) GetBranchProtection(ctx context.Context, owner, repo, branch string) (*BranchProtection, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/branches/%s/protection", owner, repo, branch)
	resp, err := c.doGitHubRequest(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 404 means branch protection is not enabled — not an error.
	if resp.StatusCode == http.StatusNotFound {
		return &BranchProtection{Enabled: false}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned %d for branch protection", resp.StatusCode)
	}

	var raw struct {
		RequiredPullRequestReviews *struct {
			RequiredApprovingReviewCount int `json:"required_approving_review_count"`
		} `json:"required_pull_request_reviews"`
		EnforceAdmins *struct {
			Enabled bool `json:"enabled"`
		} `json:"enforce_admins"`
		RequiredStatusChecks *struct {
			Strict bool `json:"strict"`
		} `json:"required_status_checks"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode branch protection response: %w", err)
	}

	bp := &BranchProtection{
		Enabled: true,
	}
	if raw.RequiredPullRequestReviews != nil {
		bp.RequiresPRReviews = true
		bp.RequiredApprovals = raw.RequiredPullRequestReviews.RequiredApprovingReviewCount
	}
	if raw.EnforceAdmins != nil {
		bp.EnforcesAdmins = raw.EnforceAdmins.Enabled
	}
	if raw.RequiredStatusChecks != nil {
		bp.RequiresStatusChecks = true
	}

	return bp, nil
}

// GetDependencyAlerts checks if vulnerability alerts are enabled for a repository.
// Returns true when enabled (HTTP 204), false when disabled (HTTP 404).
func (c *Client) GetDependencyAlerts(ctx context.Context, owner, repo string) (bool, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/vulnerability-alerts", owner, repo)
	resp, err := c.doGitHubRequest(ctx, url)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("github api returned %d for vulnerability alerts", resp.StatusCode)
	}
}

// GetSecretScanning checks if secret scanning is enabled for a repository.
func (c *Client) GetSecretScanning(ctx context.Context, owner, repo string) (bool, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)
	resp, err := c.doGitHubRequest(ctx, url)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("github api returned %d for repo info", resp.StatusCode)
	}

	var result struct {
		SecurityAndAnalysis *struct {
			SecretScanning *struct {
				Status string `json:"status"`
			} `json:"secret_scanning"`
		} `json:"security_and_analysis"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("decode repo response: %w", err)
	}

	if result.SecurityAndAnalysis == nil || result.SecurityAndAnalysis.SecretScanning == nil {
		return false, nil
	}
	return result.SecurityAndAnalysis.SecretScanning.Status == "enabled", nil
}
