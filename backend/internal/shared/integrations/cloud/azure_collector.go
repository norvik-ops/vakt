// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/sechealth-app/sechealth/internal/modules/secvitals"
)

const azureSource = "azure-collector"

// AzureCollector collects compliance evidence from an Azure subscription via REST API.
type AzureCollector struct {
	db         *pgxpool.Pool
	svRepo     *secvitals.Repository
	httpClient *http.Client
}

// NewAzureCollector creates a new AzureCollector.
func NewAzureCollector(db *pgxpool.Pool) *AzureCollector {
	return &AzureCollector{
		db:     db,
		svRepo: secvitals.NewRepository(db),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Collect runs all Azure evidence collectors for the given org and config.
// Returns the number of evidence items created.
func (c *AzureCollector) Collect(ctx context.Context, orgID string, cfg AzureConfig) (int, error) {
	token, err := c.getAccessToken(ctx, cfg)
	if err != nil {
		return 0, fmt.Errorf("azure auth: %w", err)
	}

	securityControls, err := c.svRepo.FindControlsByKeywords(ctx, orgID, []string{"security", "cloud", "monitoring", "azure"})
	if err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("azure_collector: no security controls found")
	}

	policyControls, err := c.svRepo.FindControlsByKeywords(ctx, orgID, []string{"policy", "compliance", "configuration"})
	if err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("azure_collector: no policy controls found")
	}

	total := 0

	if n, err := c.collectSecureScore(ctx, orgID, cfg.SubscriptionID, token, securityControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("azure_collector: secure score collection failed")
	} else {
		total += n
	}

	if n, err := c.collectSecurityAssessments(ctx, orgID, cfg.SubscriptionID, token, securityControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("azure_collector: security assessments collection failed")
	} else {
		total += n
	}

	if n, err := c.collectPolicyCompliance(ctx, orgID, cfg.SubscriptionID, token, policyControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("azure_collector: policy compliance collection failed")
	} else {
		total += n
	}

	return total, nil
}

// getAccessToken obtains a Bearer token via the client_credentials OAuth flow.
func (c *AzureCollector) getAccessToken(ctx context.Context, cfg AzureConfig) (string, error) {
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", cfg.TenantID)

	body := url.Values{}
	body.Set("grant_type", "client_credentials")
	body.Set("client_id", cfg.ClientID)
	body.Set("client_secret", cfg.ClientSecret)
	body.Set("scope", "https://management.azure.com/.default")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(body.Encode()))
	if err != nil {
		return "", fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(raw))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.Unmarshal(raw, &tokenResp); err != nil {
		return "", fmt.Errorf("parse token response: %w", err)
	}
	if tokenResp.Error != "" {
		return "", fmt.Errorf("token error: %s — %s", tokenResp.Error, tokenResp.ErrorDesc)
	}
	return tokenResp.AccessToken, nil
}

// azureGet performs an authenticated GET to the Azure Management API.
func (c *AzureCollector) azureGet(ctx context.Context, token, apiURL string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get %s: %w", apiURL, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("azure api %s returned %d: %s", apiURL, resp.StatusCode, string(raw))
	}

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return result, nil
}

func (c *AzureCollector) addEvidence(ctx context.Context, orgID, controlID, title string, details map[string]any) error {
	data, _ := json.Marshal(details)
	if controlID == "" {
		log.Debug().Str("org_id", orgID).Str("title", title).Msg("azure_collector: no matching control, skipping evidence")
		return nil
	}
	_, err := c.svRepo.AddCollectorEvidence(ctx, orgID, controlID, "", azureSource, title, data)
	return err
}

// collectSecureScore collects the Azure Secure Score.
func (c *AzureCollector) collectSecureScore(ctx context.Context, orgID, subID, token string, controls []secvitals.Control) (int, error) {
	apiURL := fmt.Sprintf(
		"https://management.azure.com/subscriptions/%s/providers/Microsoft.Security/secureScores?api-version=2020-01-01",
		subID,
	)

	result, err := c.azureGet(ctx, token, apiURL)
	if err != nil {
		return 0, err
	}

	details := map[string]any{
		"collected_at": time.Now().UTC().Format(time.RFC3339),
	}

	// Extract score details from "value" array
	if vals, ok := result["value"].([]any); ok && len(vals) > 0 {
		if first, ok := vals[0].(map[string]any); ok {
			if props, ok := first["properties"].(map[string]any); ok {
				details["display_name"] = first["name"]
				details["score"] = props["score"]
				details["weight"] = props["weight"]
			}
		}
		details["score_count"] = len(vals)
	}

	controlID := firstControlID(controls)
	if err := c.addEvidence(ctx, orgID, controlID, "Azure Secure Score", details); err != nil {
		return 0, err
	}
	return 1, nil
}

// collectSecurityAssessments collects Azure Security Center findings.
func (c *AzureCollector) collectSecurityAssessments(ctx context.Context, orgID, subID, token string, controls []secvitals.Control) (int, error) {
	apiURL := fmt.Sprintf(
		"https://management.azure.com/subscriptions/%s/providers/Microsoft.Security/assessments?api-version=2021-06-01",
		subID,
	)

	result, err := c.azureGet(ctx, token, apiURL)
	if err != nil {
		return 0, err
	}

	healthyCnt, unhealthyCnt, notApplicableCnt := 0, 0, 0
	if vals, ok := result["value"].([]any); ok {
		for _, v := range vals {
			if item, ok := v.(map[string]any); ok {
				if props, ok := item["properties"].(map[string]any); ok {
					if status, ok := props["status"].(map[string]any); ok {
						switch status["code"] {
						case "Healthy":
							healthyCnt++
						case "Unhealthy":
							unhealthyCnt++
						case "NotApplicable":
							notApplicableCnt++
						}
					}
				}
			}
		}
	}

	details := map[string]any{
		"collected_at":    time.Now().UTC().Format(time.RFC3339),
		"healthy":         healthyCnt,
		"unhealthy":       unhealthyCnt,
		"not_applicable":  notApplicableCnt,
	}

	controlID := firstControlID(controls)
	if err := c.addEvidence(ctx, orgID, controlID, "Azure Security Center Findings", details); err != nil {
		return 0, err
	}
	return 1, nil
}

// collectPolicyCompliance collects Azure Policy compliance summary.
func (c *AzureCollector) collectPolicyCompliance(ctx context.Context, orgID, subID, token string, controls []secvitals.Control) (int, error) {
	apiURL := fmt.Sprintf(
		"https://management.azure.com/subscriptions/%s/providers/Microsoft.Authorization/policyStates/latest/summarize?api-version=2019-10-01",
		subID,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, nil)
	if err != nil {
		return 0, fmt.Errorf("build policy compliance request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("policy compliance request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return 0, fmt.Errorf("read policy compliance response: %w", err)
	}

	var result map[string]any
	if parseErr := json.Unmarshal(raw, &result); parseErr != nil {
		return 0, fmt.Errorf("parse policy compliance: %w", parseErr)
	}

	details := map[string]any{
		"collected_at": time.Now().UTC().Format(time.RFC3339),
		"raw_summary":  result,
	}

	// Try to extract compliance counts from the summary
	if vals, ok := result["value"].([]any); ok && len(vals) > 0 {
		if first, ok := vals[0].(map[string]any); ok {
			if results, ok := first["results"].(map[string]any); ok {
				details["non_compliant_resources"] = results["nonCompliantResources"]
				details["compliant_resources"] = results["resourceDetails"]
			}
		}
	}
	delete(details, "raw_summary") // avoid very large payloads

	controlID := firstControlID(controls)
	if err := c.addEvidence(ctx, orgID, controlID, "Azure Policy Compliance", details); err != nil {
		return 0, err
	}
	return 1, nil
}
