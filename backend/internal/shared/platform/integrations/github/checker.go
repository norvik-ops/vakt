package github

import "context"

// CheckResult holds the outcome of a single compliance check against a GitHub repository.
type CheckResult struct {
	Type    string         `json:"type"`
	Status  string         `json:"status"` // pass | fail | unknown
	Details map[string]any `json:"details,omitempty"`
}

// RunAllChecks executes all configured compliance checks for a repository.
// It collects results from all checks and returns them together; individual
// check failures are captured as "unknown" status rather than aborting the run.
func RunAllChecks(ctx context.Context, client *Client, owner, repo string) ([]CheckResult, error) {
	var results []CheckResult

	// 1. Determine default branch
	branch, _ := client.GetDefaultBranch(ctx, owner, repo)
	if branch == "" {
		branch = "main"
	}

	// 2. Branch protection
	bp, err := client.GetBranchProtection(ctx, owner, repo, branch)
	if err != nil {
		results = append(results, CheckResult{
			Type:    "branch_protection",
			Status:  "unknown",
			Details: map[string]any{"error": err.Error()},
		})
	} else {
		status := "pass"
		if !bp.Enabled || !bp.RequiresPRReviews {
			status = "fail"
		}
		results = append(results, CheckResult{
			Type:   "branch_protection",
			Status: status,
			Details: map[string]any{
				"enabled":                bp.Enabled,
				"requires_pr_reviews":    bp.RequiresPRReviews,
				"required_approvals":     bp.RequiredApprovals,
				"requires_status_checks": bp.RequiresStatusChecks,
				"enforces_admins":        bp.EnforcesAdmins,
				"default_branch":         branch,
			},
		})

		// 3. PR review required (sub-check derived from branch protection)
		prStatus := "pass"
		if !bp.RequiresPRReviews {
			prStatus = "fail"
		}
		results = append(results, CheckResult{
			Type:   "pr_review_required",
			Status: prStatus,
			Details: map[string]any{
				"requires_pr_reviews": bp.RequiresPRReviews,
				"required_approvals":  bp.RequiredApprovals,
			},
		})
	}

	// 4. Dependency alerts
	depEnabled, err := client.GetDependencyAlerts(ctx, owner, repo)
	if err != nil {
		results = append(results, CheckResult{
			Type:    "dependency_alerts",
			Status:  "unknown",
			Details: map[string]any{"error": err.Error()},
		})
	} else {
		depStatus := "pass"
		if !depEnabled {
			depStatus = "fail"
		}
		results = append(results, CheckResult{
			Type:   "dependency_alerts",
			Status: depStatus,
			Details: map[string]any{
				"enabled": depEnabled,
			},
		})
	}

	// 5. Secret scanning
	ssEnabled, err := client.GetSecretScanning(ctx, owner, repo)
	if err != nil {
		results = append(results, CheckResult{
			Type:    "secret_scanning",
			Status:  "unknown",
			Details: map[string]any{"error": err.Error()},
		})
	} else {
		ssStatus := "pass"
		if !ssEnabled {
			ssStatus = "fail"
		}
		results = append(results, CheckResult{
			Type:   "secret_scanning",
			Status: ssStatus,
			Details: map[string]any{
				"enabled": ssEnabled,
			},
		})
	}

	return results, nil
}
