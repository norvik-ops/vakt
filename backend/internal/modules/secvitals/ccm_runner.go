package secvitals

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunCheck executes a CCM check and returns the status and output.
// status is one of "pass", "fail", "unknown".
func RunCheck(ctx context.Context, db *pgxpool.Pool, check CCMCheck) (status string, output string, err error) {
	switch check.CheckType {
	case "http_endpoint":
		return runHTTPEndpointCheck(ctx, check)
	case "trivy_no_critical":
		return runTrivyNoCriticalCheck(ctx, db, check)
	case "evidence_freshness":
		return runEvidenceFreshnessCheck(ctx, db, check)
	case "custom_script":
		return "unknown", "custom_script not supported in this build", nil
	default:
		return "unknown", fmt.Sprintf("unknown check type: %s", check.CheckType), nil
	}
}

// runHTTPEndpointCheck performs a GET request and passes if the response status is 2xx.
func runHTTPEndpointCheck(ctx context.Context, check CCMCheck) (string, string, error) {
	url, ok := check.Config["url"]
	if !ok || url == "" {
		return "fail", "config missing: url", nil
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "fail", fmt.Sprintf("build request: %s", err.Error()), nil
	}

	resp, err := client.Do(req)
	if err != nil {
		return "fail", fmt.Sprintf("request failed: %s", err.Error()), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return "pass", fmt.Sprintf("HTTP %d OK", resp.StatusCode), nil
	}
	return "fail", fmt.Sprintf("HTTP %d", resp.StatusCode), nil
}

// runTrivyNoCriticalCheck queries vb_findings for any open critical findings for the org.
// Passes if no critical findings exist.
func runTrivyNoCriticalCheck(ctx context.Context, db *pgxpool.Pool, check CCMCheck) (string, string, error) {
	var count int
	err := db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM vb_findings
		WHERE org_id = $1::uuid
		  AND severity = 'critical'
		  AND status NOT IN ('resolved', 'false_positive')`,
		check.OrgID,
	).Scan(&count)
	if err != nil {
		// vb_findings may not exist if the SecPulse module is disabled.
		return "unknown", "SecPulse module required for this check type", nil
	}

	if count == 0 {
		return "pass", "No open critical findings", nil
	}
	return "fail", fmt.Sprintf("%d open critical finding(s) found", count), nil
}

// runEvidenceFreshnessCheck verifies that at least one evidence item for the control
// was updated within the configured max_days window.
func runEvidenceFreshnessCheck(ctx context.Context, db *pgxpool.Pool, check CCMCheck) (string, string, error) {
	maxDaysStr, ok := check.Config["max_days"]
	if !ok || maxDaysStr == "" {
		maxDaysStr = "90"
	}

	maxDays, err := strconv.Atoi(maxDaysStr)
	if err != nil || maxDays < 1 {
		return "fail", fmt.Sprintf("invalid config max_days: %s", maxDaysStr), nil
	}

	threshold := time.Now().UTC().AddDate(0, 0, -maxDays)

	var count int
	queryErr := db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM ck_evidence
		WHERE control_id = $1::uuid
		  AND org_id = $2::uuid
		  AND updated_at >= $3`,
		check.ControlID, check.OrgID, threshold,
	).Scan(&count)
	if queryErr != nil {
		return "unknown", fmt.Sprintf("query failed: %s", queryErr.Error()), nil
	}

	if count > 0 {
		return "pass", fmt.Sprintf("Evidence updated within last %d days (%d item(s))", maxDays, count), nil
	}
	return "fail", fmt.Sprintf("No evidence updated within last %d days", maxDays), nil
}
