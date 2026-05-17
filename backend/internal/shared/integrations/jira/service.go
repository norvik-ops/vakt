// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package jira

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	sharedcrypto "github.com/sechealth-app/sechealth/internal/shared/crypto"
)

// Service handles Jira integration business logic.
type Service struct {
	db         *pgxpool.Pool
	masterKey  []byte
	httpClient *http.Client
}

// NewService creates a new Jira integration service.
func NewService(db *pgxpool.Pool, masterKey []byte) *Service {
	return &Service{
		db:        db,
		masterKey: masterKey,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// GetConfig returns the Jira config for an organisation. The API token is always masked.
func (s *Service) GetConfig(ctx context.Context, orgID string) (*Config, error) {
	var jiraURL, projectKey, userEmail, apiToken *string
	err := s.db.QueryRow(ctx, `
		SELECT jira_url, jira_project_key, jira_user_email, jira_api_token
		FROM organisations
		WHERE id = $1::uuid`,
		orgID,
	).Scan(&jiraURL, &projectKey, &userEmail, &apiToken)
	if err != nil {
		return nil, fmt.Errorf("get jira config: %w", err)
	}

	cfg := &Config{}
	if jiraURL != nil {
		cfg.JiraURL = *jiraURL
	}
	if projectKey != nil {
		cfg.ProjectKey = *projectKey
	}
	if userEmail != nil {
		cfg.UserEmail = *userEmail
	}
	if apiToken != nil && *apiToken != "" {
		cfg.APITokenMask = "****"
	}
	cfg.IsConfigured = cfg.JiraURL != "" && cfg.ProjectKey != "" && cfg.UserEmail != "" && cfg.APITokenMask == "****"
	return cfg, nil
}

// SaveConfig persists the Jira config for an organisation. The API token is encrypted.
// If the incoming api_token is "****", the existing token is left unchanged.
func (s *Service) SaveConfig(ctx context.Context, orgID string, in SaveConfigInput) error {
	// If caller sends back the mask value, keep the existing token (no-op for that field).
	if in.APIToken == "****" {
		_, err := s.db.Exec(ctx, `
			UPDATE organisations
			SET jira_url = $1, jira_project_key = $2, jira_user_email = $3
			WHERE id = $4::uuid`,
			in.JiraURL, in.ProjectKey, in.UserEmail, orgID,
		)
		return err
	}

	encrypted, err := sharedcrypto.Encrypt(s.masterKey, []byte(in.APIToken))
	if err != nil {
		return fmt.Errorf("encrypt api token: %w", err)
	}
	encryptedHex := hex.EncodeToString(encrypted)

	_, err = s.db.Exec(ctx, `
		UPDATE organisations
		SET jira_url = $1, jira_project_key = $2, jira_user_email = $3, jira_api_token = $4
		WHERE id = $5::uuid`,
		in.JiraURL, in.ProjectKey, in.UserEmail, encryptedHex, orgID,
	)
	return err
}

// TestConnection calls the Jira /myself endpoint to verify credentials.
func (s *Service) TestConnection(ctx context.Context, orgID string) (*TestResult, error) {
	jiraURL, email, token, err := s.loadDecryptedConfig(ctx, orgID)
	if err != nil {
		return &TestResult{Success: false, Error: err.Error()}, nil
	}

	endpoint := strings.TrimRight(jiraURL, "/") + "/rest/api/3/myself"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return &TestResult{Success: false, Error: "invalid jira url"}, nil
	}
	req.Header.Set("Authorization", basicAuth(email, token))
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return &TestResult{Success: false, Error: fmt.Sprintf("connection failed: %v", err)}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return &TestResult{
			Success: false,
			Error:   fmt.Sprintf("jira returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body))),
		}, nil
	}

	var myself struct {
		DisplayName string `json:"displayName"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&myself); err != nil {
		return &TestResult{Success: true}, nil
	}
	return &TestResult{Success: true, DisplayName: myself.DisplayName}, nil
}

// CreateIssueForFinding creates a Jira issue for the given finding and stores the result.
func (s *Service) CreateIssueForFinding(ctx context.Context, orgID, findingID string) (*CreateIssueResult, error) {
	// Check if issue already exists
	var existing JiraIssue
	err := s.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, finding_id::text, jira_issue_key, jira_issue_url, created_at
		FROM jira_issues
		WHERE org_id = $1::uuid AND finding_id = $2::uuid`,
		orgID, findingID,
	).Scan(&existing.ID, &existing.OrgID, &existing.FindingID, &existing.IssueKey, &existing.IssueURL, &existing.CreatedAt)
	if err == nil {
		// Already exists — return the stored result
		return &CreateIssueResult{IssueKey: existing.IssueKey, IssueURL: existing.IssueURL}, nil
	}

	// Load finding
	type findingRow struct {
		Title       string
		Description string
		Severity    string
	}
	var f findingRow
	err = s.db.QueryRow(ctx, `
		SELECT title, COALESCE(description,''), severity
		FROM vb_findings
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		findingID, orgID,
	).Scan(&f.Title, &f.Description, &f.Severity)
	if err != nil {
		return nil, fmt.Errorf("finding not found: %w", err)
	}

	jiraURL, email, token, err := s.loadDecryptedConfig(ctx, orgID)
	if err != nil {
		return nil, err
	}

	var projectKey string
	_ = s.db.QueryRow(ctx, `SELECT COALESCE(jira_project_key,'') FROM organisations WHERE id = $1::uuid`, orgID).Scan(&projectKey)

	// Build issue payload
	priority := mapPriority(f.Severity)
	descContent := buildADFDescription(f.Description)
	payload := map[string]any{
		"fields": map[string]any{
			"project":     map[string]string{"key": projectKey},
			"summary":     f.Title,
			"description": descContent,
			"issuetype":   map[string]string{"name": "Bug"},
			"priority":    map[string]string{"name": priority},
		},
	}

	body, _ := json.Marshal(payload)
	endpoint := strings.TrimRight(jiraURL, "/") + "/rest/api/3/issue"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", basicAuth(email, token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira api call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("jira returned %d: %s", resp.StatusCode, strings.TrimSpace(string(errBody)))
	}

	var created struct {
		ID   string `json:"id"`
		Key  string `json:"key"`
		Self string `json:"self"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return nil, fmt.Errorf("decode jira response: %w", err)
	}

	issueURL := strings.TrimRight(jiraURL, "/") + "/browse/" + created.Key

	// Store tracking record
	_, err = s.db.Exec(ctx, `
		INSERT INTO jira_issues (org_id, finding_id, jira_issue_key, jira_issue_url)
		VALUES ($1::uuid, $2::uuid, $3, $4)
		ON CONFLICT (org_id, finding_id) DO UPDATE
		  SET jira_issue_key = EXCLUDED.jira_issue_key,
		      jira_issue_url = EXCLUDED.jira_issue_url`,
		orgID, findingID, created.Key, issueURL,
	)
	if err != nil {
		// Non-fatal: issue was created in Jira, just couldn't store the ref locally
		return &CreateIssueResult{IssueKey: created.Key, IssueURL: issueURL}, nil
	}

	return &CreateIssueResult{IssueKey: created.Key, IssueURL: issueURL}, nil
}

// GetIssueForFinding returns the Jira issue reference for a finding, or nil if none exists.
func (s *Service) GetIssueForFinding(ctx context.Context, orgID, findingID string) (*JiraIssue, error) {
	var issue JiraIssue
	err := s.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, finding_id::text, jira_issue_key, jira_issue_url, created_at
		FROM jira_issues
		WHERE org_id = $1::uuid AND finding_id = $2::uuid`,
		orgID, findingID,
	).Scan(&issue.ID, &issue.OrgID, &issue.FindingID, &issue.IssueKey, &issue.IssueURL, &issue.CreatedAt)
	if err != nil {
		return nil, nil //nolint:nilerr // not found is ok
	}
	return &issue, nil
}

// GetIssuesForFindings returns a map of findingID → JiraIssue for the given finding IDs.
func (s *Service) GetIssuesForFindings(ctx context.Context, orgID string, findingIDs []string) (map[string]JiraIssue, error) {
	if len(findingIDs) == 0 {
		return map[string]JiraIssue{}, nil
	}

	// Build $1::uuid, $2::uuid, ... placeholders
	args := make([]any, 0, len(findingIDs)+1)
	args = append(args, orgID)
	placeholders := make([]string, len(findingIDs))
	for i, id := range findingIDs {
		args = append(args, id)
		placeholders[i] = fmt.Sprintf("$%d::uuid", i+2)
	}

	rows, err := s.db.Query(ctx, fmt.Sprintf(`
		SELECT id::text, org_id::text, finding_id::text, jira_issue_key, jira_issue_url, created_at
		FROM jira_issues
		WHERE org_id = $1::uuid AND finding_id IN (%s)`,
		strings.Join(placeholders, ", ")),
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("get issues for findings: %w", err)
	}
	defer rows.Close()

	result := make(map[string]JiraIssue)
	for rows.Next() {
		var issue JiraIssue
		if err := rows.Scan(&issue.ID, &issue.OrgID, &issue.FindingID, &issue.IssueKey, &issue.IssueURL, &issue.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan jira issue: %w", err)
		}
		result[issue.FindingID] = issue
	}
	return result, rows.Err()
}

// --- helpers ---

func (s *Service) loadDecryptedConfig(ctx context.Context, orgID string) (jiraURL, email, token string, err error) {
	var encryptedHex *string
	err = s.db.QueryRow(ctx, `
		SELECT COALESCE(jira_url,''), COALESCE(jira_user_email,''), jira_api_token
		FROM organisations WHERE id = $1::uuid`,
		orgID,
	).Scan(&jiraURL, &email, &encryptedHex)
	if err != nil {
		return "", "", "", fmt.Errorf("load jira config: %w", err)
	}
	if jiraURL == "" || email == "" || encryptedHex == nil || *encryptedHex == "" {
		return "", "", "", fmt.Errorf("jira integration not configured")
	}
	encBytes, err := hex.DecodeString(*encryptedHex)
	if err != nil {
		return "", "", "", fmt.Errorf("decode api token: %w", err)
	}
	tokenBytes, err := sharedcrypto.Decrypt(s.masterKey, encBytes)
	if err != nil {
		return "", "", "", fmt.Errorf("decrypt api token: %w", err)
	}
	return jiraURL, email, string(tokenBytes), nil
}

func basicAuth(email, token string) string {
	raw := email + ":" + token
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(raw))
}

func mapPriority(severity string) string {
	switch severity {
	case "critical":
		return "Highest"
	case "high":
		return "High"
	case "medium":
		return "Medium"
	default:
		return "Low"
	}
}

// buildADFDescription builds an Atlassian Document Format description node.
func buildADFDescription(text string) map[string]any {
	return map[string]any{
		"type":    "doc",
		"version": 1,
		"content": []map[string]any{
			{
				"type": "paragraph",
				"content": []map[string]any{
					{
						"type": "text",
						"text": text,
					},
				},
			},
		},
	}
}
