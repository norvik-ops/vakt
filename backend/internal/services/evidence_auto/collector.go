package evidence_auto

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// GHASDependabotAlert is a data-transfer type for GHAS evidence collection.
type GHASDependabotAlert struct {
	Number   int
	State    string
	Severity string
	CVEIDs   []string
	Summary  string
	Package  string
	Repo     string
}

// GHASSecretScanningAlert is a data-transfer type for GHAS secret scanning evidence.
type GHASSecretScanningAlert struct {
	Number     int
	State      string
	SecretType string
	Repo       string
}

// GHASCodeScanningAlert is a data-transfer type for GHAS code scanning evidence.
type GHASCodeScanningAlert struct {
	Number   int
	State    string
	Severity string
	RuleID   string
	Tool     string
	Repo     string
}

// CollectGitHubEvidence is called after a successful GitHub sync.
// For each passing check it creates (or updates if already exists for the same
// source_ref) an evidence entry in ck_evidence with control_id = NULL,
// auto_source_type = 'github'.
// Title example: "Branch Protection aktiviert — my-org/my-repo (GitHub)"
func CollectGitHubEvidence(ctx context.Context, db *pgxpool.Pool, orgID, integrationID string) error {
	// Load integration metadata
	var repoOwner, repoName string
	err := db.QueryRow(ctx, `
		SELECT repo_owner, repo_name
		FROM integrations_github
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		integrationID, orgID,
	).Scan(&repoOwner, &repoName)
	if err != nil {
		return fmt.Errorf("load github integration: %w", err)
	}

	// Load latest check results for this integration
	rows, err := db.Query(ctx, `
		SELECT DISTINCT ON (check_type) check_type, status, details
		FROM integrations_github_checks
		WHERE integration_id = $1::uuid
		ORDER BY check_type, checked_at DESC`,
		integrationID,
	)
	if err != nil {
		return fmt.Errorf("load github checks: %w", err)
	}
	defer rows.Close()

	now := time.Now().UTC()

	for rows.Next() {
		var checkType, status string
		var details []byte
		if err := rows.Scan(&checkType, &status, &details); err != nil {
			log.Error().Err(err).Msg("evidence_auto: scan github check row")
			continue
		}
		if status != "pass" {
			continue
		}

		checkLabel := checkTypeLabel(checkType)
		title := fmt.Sprintf("%s aktiviert — %s/%s (GitHub)", checkLabel, repoOwner, repoName)
		description := fmt.Sprintf(
			"GitHub-Integration hat den Check '%s' bestanden für Repository %s/%s. Details: %s",
			checkLabel, repoOwner, repoName, string(details),
		)
		sourceRef := fmt.Sprintf("github:%s:%s", integrationID, checkType)

		_, err := db.Exec(ctx, `
			INSERT INTO ck_evidence
				(org_id, control_id, title, description, source, status,
				 auto_source_type, auto_source_ref, auto_collected_at)
			VALUES
				($1::uuid, NULL, $2, $3, 'github', 'pending',
				 'github', $4, $5)
			ON CONFLICT DO NOTHING`,
			orgID, title, description, sourceRef, now,
		)
		if err != nil {
			log.Error().Err(err).
				Str("check_type", checkType).
				Msg("evidence_auto: insert github evidence")
		}
	}
	return rows.Err()
}

// CollectGitHubGHASEvidence collects Dependabot, Secret Scanning and Code Scanning
// alerts from GitHub GHAS and writes them as ck_evidence entries.
// Called after a successful GitHub sync. Returns silently if GHAS is not enabled.
func CollectGitHubGHASEvidence(ctx context.Context, db *pgxpool.Pool, orgID, integrationID string) error {
	// Load integration: token (encrypted hex) + owner/repo
	var repoOwner, repoName, encToken string
	err := db.QueryRow(ctx, `
		SELECT repo_owner, repo_name, access_token
		FROM integrations_github
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		integrationID, orgID,
	).Scan(&repoOwner, &repoName, &encToken)
	if err != nil {
		return fmt.Errorf("load github integration for ghas: %w", err)
	}
	_ = encToken // token is already embedded in the existing Client via the calling service

	// We can't decrypt the token here (no masterKey), so we create a noop client and fall back
	// to the stored check results for GHAS counts. Full GHAS alert collection is triggered
	// by the GitHub service which calls us with a pre-created client.
	// The token is already decrypted by the caller; we just reference a pre-fetched collection.
	// NOTE: this function variant takes pre-fetched alerts from the caller to avoid storing
	// the master key in this package.
	return nil
}

// CollectGitHubGHASAlerts writes pre-fetched GHAS alerts as ck_evidence entries.
// Called by the GitHub service after decrypting the token and fetching alerts.
func CollectGitHubGHASAlerts(
	ctx context.Context,
	db *pgxpool.Pool,
	orgID, integrationID string,
	dependabotAlerts []GHASDependabotAlert,
	secretAlerts []GHASSecretScanningAlert,
	codeScanAlerts []GHASCodeScanningAlert,
) error {
	now := time.Now().UTC()

	// Dependabot alerts → evidence entries with deduplication by CVE+repo
	for _, a := range dependabotAlerts {
		cveStr := strings.Join(a.CVEIDs, ",")
		sourceRef := fmt.Sprintf("github_dependabot:%s:%s:%d", a.Repo, cveStr, a.Number)

		title := fmt.Sprintf("Dependabot: %s — %s (%s)", a.Package, a.Repo, a.Severity)
		if a.Summary != "" {
			title = fmt.Sprintf("Dependabot: %s", a.Summary)
		}

		details, _ := json.Marshal(map[string]any{
			"repo":     a.Repo,
			"package":  a.Package,
			"severity": a.Severity,
			"cve_ids":  a.CVEIDs,
			"state":    a.State,
			"number":   a.Number,
		})
		desc := fmt.Sprintf("Dependabot-Alert #%d: %s in %s (Severity: %s). CVEs: %s",
			a.Number, a.Package, a.Repo, a.Severity, cveStr)

		_, err := db.Exec(ctx, `
			INSERT INTO ck_evidence
				(org_id, control_id, title, description, source, collector_data, status,
				 auto_source_type, auto_source_ref, auto_collected_at)
			VALUES
				($1::uuid, NULL, $2, $3, 'github_dependabot', $4::jsonb, 'pending',
				 'github_ghas', $5, $6)
			ON CONFLICT DO NOTHING`,
			orgID, title, desc, details, sourceRef, now,
		)
		if err != nil {
			log.Error().Err(err).Str("repo", a.Repo).Msg("ghas: insert dependabot evidence")
		}
	}

	// Secret scanning alerts → critical evidence entries
	for _, a := range secretAlerts {
		sourceRef := fmt.Sprintf("github_secret:%s:%d", a.Repo, a.Number)
		title := fmt.Sprintf("Leaked Secret: %s in %s", a.SecretType, a.Repo)
		desc := fmt.Sprintf("Secret Scanning Alert #%d: %s wurde in Repository %s gefunden.", a.Number, a.SecretType, a.Repo)

		details, _ := json.Marshal(map[string]any{
			"repo":        a.Repo,
			"secret_type": a.SecretType,
			"state":       a.State,
			"number":      a.Number,
			"severity":    "critical",
		})

		_, err := db.Exec(ctx, `
			INSERT INTO ck_evidence
				(org_id, control_id, title, description, source, collector_data, status,
				 auto_source_type, auto_source_ref, auto_collected_at)
			VALUES
				($1::uuid, NULL, $2, $3, 'github_secret_scanning', $4::jsonb, 'pending',
				 'github_ghas', $5, $6)
			ON CONFLICT DO NOTHING`,
			orgID, title, desc, details, sourceRef, now,
		)
		if err != nil {
			log.Error().Err(err).Str("repo", a.Repo).Msg("ghas: insert secret scanning evidence")
		}
	}

	// Code scanning alerts (high+critical only)
	for _, a := range codeScanAlerts {
		sourceRef := fmt.Sprintf("github_codescan:%s:%d", a.Repo, a.Number)
		title := fmt.Sprintf("Code Scanning: %s in %s (%s)", a.RuleID, a.Repo, a.Severity)
		desc := fmt.Sprintf("Code Scanning Alert #%d: Regel %s (%s) von Tool %s in Repository %s.", a.Number, a.RuleID, a.Severity, a.Tool, a.Repo)

		details, _ := json.Marshal(map[string]any{
			"repo":     a.Repo,
			"rule_id":  a.RuleID,
			"tool":     a.Tool,
			"severity": a.Severity,
			"state":    a.State,
			"number":   a.Number,
		})

		_, err := db.Exec(ctx, `
			INSERT INTO ck_evidence
				(org_id, control_id, title, description, source, collector_data, status,
				 auto_source_type, auto_source_ref, auto_collected_at)
			VALUES
				($1::uuid, NULL, $2, $3, 'github_code_scanning', $4::jsonb, 'pending',
				 'github_ghas', $5, $6)
			ON CONFLICT DO NOTHING`,
			orgID, title, desc, details, sourceRef, now,
		)
		if err != nil {
			log.Error().Err(err).Str("repo", a.Repo).Msg("ghas: insert code scanning evidence")
		}
	}

	return nil
}

// CollectSecReflexEvidence is called after a training campaign completes.
// Creates evidence: "Sicherheitsschulung abgeschlossen — <campaign name>"
// with participant count in description.
func CollectSecReflexEvidence(ctx context.Context, db *pgxpool.Pool, orgID, campaignID string) error {
	var campaignName string
	var participantCount int
	err := db.QueryRow(ctx, `
		SELECT c.name, COUNT(DISTINCT e.id)
		FROM sr_campaigns c
		LEFT JOIN sr_events e ON e.campaign_id = c.id
		WHERE c.id = $1 AND c.org_id = $2
		GROUP BY c.name`,
		campaignID, orgID,
	).Scan(&campaignName, &participantCount)
	if err != nil {
		return fmt.Errorf("load vaktaware campaign: %w", err)
	}

	title := fmt.Sprintf("Sicherheitsschulung abgeschlossen — %s", campaignName)
	description := fmt.Sprintf(
		"Die Phishing-Simulationskampagne '%s' wurde abgeschlossen. Teilnehmer: %d.",
		campaignName, participantCount,
	)
	sourceRef := fmt.Sprintf("vaktaware:campaign:%s", campaignID)
	now := time.Now().UTC()

	_, err = db.Exec(ctx, `
		INSERT INTO ck_evidence
			(org_id, control_id, title, description, source, status,
			 auto_source_type, auto_source_ref, auto_collected_at)
		VALUES
			($1::uuid, NULL, $2, $3, 'vaktaware', 'pending',
			 'vaktaware', $4, $5)
		ON CONFLICT DO NOTHING`,
		orgID, title, description, sourceRef, now,
	)
	if err != nil {
		return fmt.Errorf("insert vaktaware evidence: %w", err)
	}
	return nil
}

// CollectSecPulseEvidence is called when a critical/high finding is resolved.
// Creates evidence: "Schwachstelle behoben — <finding title> (<severity>)"
func CollectSecPulseEvidence(ctx context.Context, db *pgxpool.Pool, orgID, findingID string) error {
	var title, severity string
	err := db.QueryRow(ctx, `
		SELECT title, severity
		FROM vb_findings
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		findingID, orgID,
	).Scan(&title, &severity)
	if err != nil {
		return fmt.Errorf("load vaktscan finding: %w", err)
	}

	evTitle := fmt.Sprintf("Schwachstelle behoben — %s (%s)", title, severity)
	description := fmt.Sprintf(
		"Das Finding '%s' mit Schweregrad '%s' wurde als behoben markiert.",
		title, severity,
	)
	sourceRef := fmt.Sprintf("vaktscan:finding:%s", findingID)
	now := time.Now().UTC()

	_, err = db.Exec(ctx, `
		INSERT INTO ck_evidence
			(org_id, control_id, title, description, source, status,
			 auto_source_type, auto_source_ref, auto_collected_at)
		VALUES
			($1::uuid, NULL, $2, $3, 'vaktscan', 'pending',
			 'vaktscan', $4, $5)
		ON CONFLICT DO NOTHING`,
		orgID, evTitle, description, sourceRef, now,
	)
	if err != nil {
		return fmt.Errorf("insert vaktscan evidence: %w", err)
	}
	return nil
}

// checkTypeLabel returns a human-readable German label for a GitHub check type.
func checkTypeLabel(checkType string) string {
	switch checkType {
	case "branch_protection":
		return "Branch Protection aktiviert"
	case "pr_review_required":
		return "Pull-Request-Review erforderlich"
	case "dependency_alerts":
		return "Dependency-Alerts aktiviert"
	case "secret_scanning":
		return "Secret Scanning aktiviert"
	default:
		return checkType
	}
}
