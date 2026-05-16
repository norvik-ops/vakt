package github

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	sharedcrypto "github.com/sechealth-app/sechealth/internal/shared/crypto"
	"github.com/sechealth-app/sechealth/internal/shared/evidence_auto"
)

// Service handles GitHub integration business logic.
type Service struct {
	db        *pgxpool.Pool
	masterKey []byte
}

// NewService creates a new GitHub integration service.
func NewService(db *pgxpool.Pool, masterKey []byte) *Service {
	return &Service{
		db:        db,
		masterKey: masterKey,
	}
}

// AddIntegration creates a new GitHub repository integration, encrypting the access token.
func (s *Service) AddIntegration(ctx context.Context, orgID string, in AddIntegrationInput) (*Integration, error) {
	encrypted, err := sharedcrypto.Encrypt(s.masterKey, []byte(in.AccessToken))
	if err != nil {
		return nil, fmt.Errorf("encrypt access token: %w", err)
	}
	encryptedHex := hex.EncodeToString(encrypted)

	var ig Integration
	err = s.db.QueryRow(ctx, `
		INSERT INTO integrations_github (org_id, repo_owner, repo_name, access_token)
		VALUES ($1::uuid, $2, $3, $4)
		RETURNING id::text, org_id::text, repo_owner, repo_name, last_synced_at, sync_status, COALESCE(sync_error,''), created_at, updated_at`,
		orgID, in.RepoOwner, in.RepoName, encryptedHex,
	).Scan(&ig.ID, &ig.OrgID, &ig.RepoOwner, &ig.RepoName, &ig.LastSyncedAt, &ig.SyncStatus, &ig.SyncError, &ig.CreatedAt, &ig.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert integration: %w", err)
	}
	return &ig, nil
}

// ListIntegrations returns all GitHub integrations for an organisation.
func (s *Service) ListIntegrations(ctx context.Context, orgID string) ([]Integration, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id::text, org_id::text, repo_owner, repo_name, last_synced_at, sync_status, COALESCE(sync_error,''), created_at, updated_at
		FROM integrations_github
		WHERE org_id = $1::uuid
		ORDER BY created_at DESC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list integrations: %w", err)
	}
	defer rows.Close()

	var result []Integration
	for rows.Next() {
		var ig Integration
		if err := rows.Scan(&ig.ID, &ig.OrgID, &ig.RepoOwner, &ig.RepoName, &ig.LastSyncedAt, &ig.SyncStatus, &ig.SyncError, &ig.CreatedAt, &ig.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan integration: %w", err)
		}
		result = append(result, ig)
	}
	return result, rows.Err()
}

// DeleteIntegration removes a GitHub integration (and cascades to checks).
func (s *Service) DeleteIntegration(ctx context.Context, orgID, id string) error {
	tag, err := s.db.Exec(ctx, `
		DELETE FROM integrations_github WHERE id = $1::uuid AND org_id = $2::uuid`,
		id, orgID,
	)
	if err != nil {
		return fmt.Errorf("delete integration: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("integration not found")
	}
	return nil
}

// SyncIntegration runs all checks for a GitHub integration, stores results, and creates evidence.
func (s *Service) SyncIntegration(ctx context.Context, orgID, id string) error {
	// Load integration including encrypted token
	var ig Integration
	var encryptedHex string
	err := s.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, repo_owner, repo_name, access_token
		FROM integrations_github
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		id, orgID,
	).Scan(&ig.ID, &ig.OrgID, &ig.RepoOwner, &ig.RepoName, &encryptedHex)
	if err != nil {
		return fmt.Errorf("load integration: %w", err)
	}

	// Decrypt token
	encryptedBytes, err := hex.DecodeString(encryptedHex)
	if err != nil {
		return fmt.Errorf("decode encrypted token: %w", err)
	}
	tokenBytes, err := sharedcrypto.Decrypt(s.masterKey, encryptedBytes)
	if err != nil {
		return fmt.Errorf("decrypt access token: %w", err)
	}

	// Run checks
	client := NewClient(string(tokenBytes))
	checkResults, runErr := RunAllChecks(ctx, client, ig.RepoOwner, ig.RepoName)

	now := time.Now().UTC()

	if runErr != nil {
		// Mark as error but continue with whatever partial results we have
		_, _ = s.db.Exec(ctx, `
			UPDATE integrations_github
			SET sync_status = 'error', sync_error = $1, last_synced_at = $2, updated_at = $2
			WHERE id = $3::uuid`,
			runErr.Error(), now, id,
		)
		return fmt.Errorf("run checks: %w", runErr)
	}

	// Persist check results
	for _, cr := range checkResults {
		detailsJSON, _ := json.Marshal(cr.Details)
		_, err := s.db.Exec(ctx, `
			INSERT INTO integrations_github_checks (integration_id, check_type, status, details, checked_at)
			VALUES ($1::uuid, $2, $3, $4::jsonb, $5)`,
			id, cr.Type, cr.Status, detailsJSON, now,
		)
		if err != nil {
			log.Error().Err(err).Str("check_type", cr.Type).Msg("failed to persist github check result")
		}
	}

	// Update sync status
	_, err = s.db.Exec(ctx, `
		UPDATE integrations_github
		SET sync_status = 'ok', sync_error = NULL, last_synced_at = $1, updated_at = $1
		WHERE id = $2::uuid`,
		now, id,
	)
	if err != nil {
		log.Error().Err(err).Msg("failed to update integration sync status")
	}

	// Write evidence into ck_evidence (best-effort — never fail sync for this)
	s.writeEvidence(ctx, orgID, ig.RepoOwner, ig.RepoName, checkResults)

	// Collect auto-evidence into the unassigned inbox (best-effort).
	if autoErr := evidence_auto.CollectGitHubEvidence(ctx, s.db, orgID, id); autoErr != nil {
		log.Error().Err(autoErr).Str("integration_id", id).Msg("evidence_auto: github collection failed")
	}

	return nil
}

// writeEvidence inserts compliance evidence into ck_evidence for relevant check results.
// Failures are logged but do not abort the sync.
func (s *Service) writeEvidence(ctx context.Context, orgID, owner, repo string, results []CheckResult) {
	for _, cr := range results {
		if cr.Status == "unknown" {
			continue
		}

		// Build evidence title
		checkLabel := strings.ReplaceAll(cr.Type, "_", " ")
		checkLabel = strings.Title(checkLabel) //nolint:staticcheck // acceptable for display text
		title := fmt.Sprintf("GitHub %s — %s/%s", checkLabel, owner, repo)

		descStatus := "passed"
		if cr.Status == "fail" {
			descStatus = "failed"
		}
		description := fmt.Sprintf("Automated GitHub integration check: %s %s for repository %s/%s.", checkLabel, descStatus, owner, repo)

		detailsJSON, _ := json.Marshal(cr.Details)

		// Find a matching control (fuzzy match on title containing relevant keywords)
		keywords := s.checkTypeKeywords(cr.Type)
		controlID := s.findMatchingControl(ctx, orgID, keywords)
		if controlID == "" {
			// No matching control — skip this evidence entry
			log.Debug().Str("check_type", cr.Type).Msg("no matching control found for github evidence")
			continue
		}

		_, err := s.db.Exec(ctx, `
			INSERT INTO ck_evidence (control_id, org_id, title, description, source, collector_data, status)
			VALUES ($1::uuid, $2::uuid, $3, $4, 'github_integration', $5::jsonb, 'pending')`,
			controlID, orgID, title, description, detailsJSON,
		)
		if err != nil {
			log.Error().Err(err).Str("check_type", cr.Type).Msg("failed to write github evidence")
		}
	}
}

// checkTypeKeywords returns SQL ILIKE patterns for matching control titles.
func (s *Service) checkTypeKeywords(checkType string) []string {
	switch checkType {
	case "branch_protection":
		return []string{"%branch protection%", "%access control%", "%source control%"}
	case "pr_review_required":
		return []string{"%pull request%", "%code review%", "%peer review%", "%access control%"}
	case "dependency_alerts":
		return []string{"%dependency%", "%vulnerability%", "%patch management%"}
	case "secret_scanning":
		return []string{"%secret%", "%credential%", "%access control%"}
	default:
		return []string{"%access control%"}
	}
}

// findMatchingControl searches ck_controls for a control whose title matches one of the keywords.
func (s *Service) findMatchingControl(ctx context.Context, orgID string, keywords []string) string {
	for _, kw := range keywords {
		var controlID string
		err := s.db.QueryRow(ctx, `
			SELECT id::text FROM ck_controls
			WHERE org_id = $1::uuid AND title ILIKE $2
			ORDER BY created_at ASC
			LIMIT 1`,
			orgID, kw,
		).Scan(&controlID)
		if err == nil && controlID != "" {
			return controlID
		}
	}
	return ""
}

// ListCheckResults returns the latest check results for a given integration.
func (s *Service) ListCheckResults(ctx context.Context, integrationID string) ([]StoredCheckResult, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id::text, integration_id::text, check_type, status, details, checked_at
		FROM integrations_github_checks
		WHERE integration_id = $1::uuid
		ORDER BY checked_at DESC
		LIMIT 100`,
		integrationID,
	)
	if err != nil {
		return nil, fmt.Errorf("list check results: %w", err)
	}
	defer rows.Close()

	var results []StoredCheckResult
	for rows.Next() {
		var r StoredCheckResult
		var detailsJSON []byte
		if err := rows.Scan(&r.ID, &r.IntegrationID, &r.CheckType, &r.Status, &detailsJSON, &r.CheckedAt); err != nil {
			return nil, fmt.Errorf("scan check result: %w", err)
		}
		if len(detailsJSON) > 0 {
			_ = json.Unmarshal(detailsJSON, &r.Details)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}
