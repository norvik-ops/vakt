package evidence_auto

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

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
		return fmt.Errorf("load secreflex campaign: %w", err)
	}

	title := fmt.Sprintf("Sicherheitsschulung abgeschlossen — %s", campaignName)
	description := fmt.Sprintf(
		"Die Phishing-Simulationskampagne '%s' wurde abgeschlossen. Teilnehmer: %d.",
		campaignName, participantCount,
	)
	sourceRef := fmt.Sprintf("secreflex:campaign:%s", campaignID)
	now := time.Now().UTC()

	_, err = db.Exec(ctx, `
		INSERT INTO ck_evidence
			(org_id, control_id, title, description, source, status,
			 auto_source_type, auto_source_ref, auto_collected_at)
		VALUES
			($1::uuid, NULL, $2, $3, 'secreflex', 'pending',
			 'secreflex', $4, $5)
		ON CONFLICT DO NOTHING`,
		orgID, title, description, sourceRef, now,
	)
	if err != nil {
		return fmt.Errorf("insert secreflex evidence: %w", err)
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
		return fmt.Errorf("load secpulse finding: %w", err)
	}

	evTitle := fmt.Sprintf("Schwachstelle behoben — %s (%s)", title, severity)
	description := fmt.Sprintf(
		"Das Finding '%s' mit Schweregrad '%s' wurde als behoben markiert.",
		title, severity,
	)
	sourceRef := fmt.Sprintf("secpulse:finding:%s", findingID)
	now := time.Now().UTC()

	_, err = db.Exec(ctx, `
		INSERT INTO ck_evidence
			(org_id, control_id, title, description, source, status,
			 auto_source_type, auto_source_ref, auto_collected_at)
		VALUES
			($1::uuid, NULL, $2, $3, 'secpulse', 'pending',
			 'secpulse', $4, $5)
		ON CONFLICT DO NOTHING`,
		orgID, evTitle, description, sourceRef, now,
	)
	if err != nil {
		return fmt.Errorf("insert secpulse evidence: %w", err)
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
