// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvitals

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// collectorHTTPClient is a shared HTTP client for all collector outbound requests.
// A 30-second timeout prevents unbounded hangs against slow or unresponsive APIs.
var collectorHTTPClient = &http.Client{Timeout: 30 * time.Second}

// Collector is the interface that automated evidence sources must implement.
type Collector interface {
	// Name returns the unique source identifier (e.g. "github", "aws").
	Name() string
	// Collect gathers evidence data and returns it as a JSON-encoded byte slice.
	Collect(ctx context.Context, cfg CollectorConfig) ([]byte, error)
}

// CollectorConfig carries the runtime parameters for a collector run.
type CollectorConfig struct {
	// Type identifies the collector: "github", "aws", "azure", "ad".
	Type string
	// Params holds connector-specific values such as tokens, URLs, account IDs.
	Params map[string]string
}

// registry maps source names to Collector implementations.
var registry = map[string]Collector{
	"github": &GitHubCollector{},
	"aws":    &AWSCollector{},
	"azure":  &AzureCollector{},
}

// GetCollector returns the registered Collector for the given source name, or an error
// if the source is unknown.
func GetCollector(source string) (Collector, error) {
	c, ok := registry[source]
	if !ok {
		return nil, fmt.Errorf("unknown collector source: %s", source)
	}
	return c, nil
}

// --- GitHubCollector ---

// GitHubCollector collects evidence from the GitHub REST API.
// It checks: branch protection rules, required PR reviews, and vulnerability alerts.
type GitHubCollector struct{}

// Name returns the collector's source identifier.
func (c *GitHubCollector) Name() string { return "github" }

// githubResult is the JSON-serialisable output produced by GitHubCollector.Collect.
type githubResult struct {
	CollectedAt               time.Time `json:"collected_at"`
	Owner                     string    `json:"owner"`
	Repo                      string    `json:"repo"`
	DefaultBranch             string    `json:"default_branch"`
	BranchProtectionEnabled   bool      `json:"branch_protection_enabled"`
	RequiredPRReviews         bool      `json:"required_pr_reviews"`
	VulnerabilityAlertsEnabled bool     `json:"vulnerability_alerts_enabled"`
	Error                     string    `json:"error,omitempty"`
}

// Collect calls the GitHub REST API and returns a JSON summary of security controls.
// Required params: "token" (PAT), "owner", "repo".
func (c *GitHubCollector) Collect(ctx context.Context, cfg CollectorConfig) ([]byte, error) {
	token := cfg.Params["token"]
	owner := cfg.Params["owner"]
	repo := cfg.Params["repo"]

	if token == "" || owner == "" || repo == "" {
		return nil, fmt.Errorf("github collector requires params: token, owner, repo")
	}

	result := githubResult{
		CollectedAt: time.Now().UTC(),
		Owner:       owner,
		Repo:        repo,
	}

	// Helper: perform authenticated GitHub API request.
	doRequest := func(urlPath string) (map[string]interface{}, int, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			"https://api.github.com"+urlPath, nil)
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("Authorization", "token "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		resp, err := collectorHTTPClient.Do(req)
		if err != nil {
			return nil, 0, err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, resp.StatusCode, err
		}
		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			return nil, resp.StatusCode, nil
		}
		return data, resp.StatusCode, nil
	}

	// 1. Fetch repo info to get default branch.
	repoData, status, err := doRequest(fmt.Sprintf("/repos/%s/%s", owner, repo))
	if err != nil || status != http.StatusOK {
		result.Error = fmt.Sprintf("repo fetch failed (status %d): %v", status, err)
		return json.Marshal(result)
	}
	if db, ok := repoData["default_branch"].(string); ok {
		result.DefaultBranch = db
	}

	// 2. Check branch protection on the default branch.
	if result.DefaultBranch != "" {
		bpData, bpStatus, _ := doRequest(fmt.Sprintf("/repos/%s/%s/branches/%s/protection",
			owner, repo, result.DefaultBranch))
		if bpStatus == http.StatusOK && bpData != nil {
			result.BranchProtectionEnabled = true
			if rr, ok := bpData["required_pull_request_reviews"].(map[string]interface{}); ok {
				if cnt, ok := rr["required_approving_review_count"].(float64); ok && cnt > 0 {
					result.RequiredPRReviews = true
				}
			}
		}
	}

	// 3. Check vulnerability alerts (requires special Accept header; returns 204 if enabled).
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("https://api.github.com/repos/%s/%s/vulnerability-alerts", owner, repo), nil)
	if err == nil {
		req.Header.Set("Authorization", "token "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		if resp, err := collectorHTTPClient.Do(req); err == nil {
			defer resp.Body.Close()
			result.VulnerabilityAlertsEnabled = resp.StatusCode == http.StatusNoContent
		}
	}

	return json.Marshal(result)
}

// --- AWSCollector ---

// AWSCollector collects security posture evidence from AWS:
// CloudTrail logging status, S3 default encryption, and IAM account password policy.
// Required params: "region", "access_key_id", "secret_access_key".
type AWSCollector struct{}

func (c *AWSCollector) Name() string { return "aws" }

type awsResult struct {
	CollectedAt          time.Time `json:"collected_at"`
	Region               string    `json:"region"`
	CloudTrailEnabled    bool      `json:"cloudtrail_enabled"`
	S3DefaultEncryption  bool      `json:"s3_default_encryption"`
	IAMPasswordPolicySet bool      `json:"iam_password_policy_set"`
	Error                string    `json:"error,omitempty"`
}

func (c *AWSCollector) Collect(ctx context.Context, cfg CollectorConfig) ([]byte, error) {
	region := cfg.Params["region"]
	if region == "" {
		region = "us-east-1"
	}
	keyID := cfg.Params["access_key_id"]
	secret := cfg.Params["secret_access_key"]

	result := awsResult{
		CollectedAt: time.Now().UTC(),
		Region:      region,
	}

	if keyID == "" || secret == "" {
		result.Error = "missing params: access_key_id and secret_access_key are required"
		return json.Marshal(result)
	}

	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(keyID, secret, ""),
		),
	)
	if err != nil {
		result.Error = fmt.Sprintf("aws config: %v", err)
		return json.Marshal(result)
	}

	// 1. CloudTrail — check if at least one trail is logging.
	ctClient := cloudtrail.NewFromConfig(awsCfg)
	trails, err := ctClient.DescribeTrails(ctx, &cloudtrail.DescribeTrailsInput{
		IncludeShadowTrails: aws.Bool(false),
	})
	if err != nil {
		result.Error = fmt.Sprintf("cloudtrail: %v", err)
		return json.Marshal(result)
	}
	for _, trail := range trails.TrailList {
		if trail.TrailARN == nil {
			continue
		}
		status, err := ctClient.GetTrailStatus(ctx, &cloudtrail.GetTrailStatusInput{
			Name: trail.TrailARN,
		})
		if err == nil && status.IsLogging != nil && *status.IsLogging {
			result.CloudTrailEnabled = true
			break
		}
	}

	// 2. S3 default encryption — check the first bucket found.
	s3Client := s3.NewFromConfig(awsCfg)
	buckets, err := s3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err == nil && len(buckets.Buckets) > 0 {
		firstBucket := aws.ToString(buckets.Buckets[0].Name)
		enc, err := s3Client.GetBucketEncryption(ctx, &s3.GetBucketEncryptionInput{
			Bucket: aws.String(firstBucket),
		})
		if err == nil && enc.ServerSideEncryptionConfiguration != nil {
			result.S3DefaultEncryption = len(enc.ServerSideEncryptionConfiguration.Rules) > 0
		}
	}

	// 3. IAM account password policy.
	iamClient := iam.NewFromConfig(awsCfg)
	_, err = iamClient.GetAccountPasswordPolicy(ctx, &iam.GetAccountPasswordPolicyInput{})
	result.IAMPasswordPolicySet = err == nil

	return json.Marshal(result)
}

// --- AzureCollector ---

// AzureCollector collects security posture evidence from Azure via the Management REST API:
// Microsoft Defender for Cloud (Security Center) pricing tier and MFA Conditional Access policies.
// Required params: "tenant_id", "client_id", "client_secret", "subscription_id".
type AzureCollector struct{}

func (c *AzureCollector) Name() string { return "azure" }

type azureResult struct {
	CollectedAt           time.Time `json:"collected_at"`
	TenantID              string    `json:"tenant_id"`
	SecurityCenterEnabled bool      `json:"security_center_enabled"`
	MFAPolicyEnforced     bool      `json:"mfa_policy_enforced"`
	Error                 string    `json:"error,omitempty"`
}

func (c *AzureCollector) Collect(ctx context.Context, cfg CollectorConfig) ([]byte, error) {
	tenantID := cfg.Params["tenant_id"]
	clientID := cfg.Params["client_id"]
	clientSecret := cfg.Params["client_secret"]
	subscriptionID := cfg.Params["subscription_id"]

	result := azureResult{
		CollectedAt: time.Now().UTC(),
		TenantID:    tenantID,
	}

	if tenantID == "" {
		return nil, fmt.Errorf("azure collector requires param: tenant_id")
	}
	if clientID == "" || clientSecret == "" {
		result.Error = "missing params: client_id and client_secret are required"
		return json.Marshal(result)
	}

	// Acquire management token (scope: Azure Resource Manager).
	mgmtToken, err := azureOAuth2Token(ctx, tenantID, clientID, clientSecret, "https://management.azure.com/.default")
	if err != nil {
		result.Error = fmt.Sprintf("azure auth: %v", err)
		return json.Marshal(result)
	}

	// 1. Microsoft Defender for Cloud — check if Standard tier is enabled on any resource.
	if subscriptionID != "" {
		apiURL := fmt.Sprintf(
			"https://management.azure.com/subscriptions/%s/providers/Microsoft.Security/pricings?api-version=2023-01-01",
			subscriptionID,
		)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
		req.Header.Set("Authorization", "Bearer "+mgmtToken)
		resp, err := collectorHTTPClient.Do(req)
		if err == nil {
			defer resp.Body.Close()
			var body struct {
				Value []struct {
					Properties struct {
						PricingTier string `json:"pricingTier"`
					} `json:"properties"`
				} `json:"value"`
			}
			if json.NewDecoder(resp.Body).Decode(&body) == nil {
				for _, p := range body.Value {
					if strings.EqualFold(p.Properties.PricingTier, "Standard") {
						result.SecurityCenterEnabled = true
						break
					}
				}
			}
		}
	}

	// 2. MFA via Conditional Access — acquire Graph token and list CA policies.
	graphToken, err := azureOAuth2Token(ctx, tenantID, clientID, clientSecret, "https://graph.microsoft.com/.default")
	if err == nil {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
			"https://graph.microsoft.com/v1.0/identity/conditionalAccess/policies", nil)
		req.Header.Set("Authorization", "Bearer "+graphToken)
		resp, err := collectorHTTPClient.Do(req)
		if err == nil {
			defer resp.Body.Close()
			var body struct {
				Value []struct {
					State           string `json:"state"`
					GrantControls   *struct {
						BuiltInControls []string `json:"builtInControls"`
					} `json:"grantControls"`
				} `json:"value"`
			}
			if json.NewDecoder(resp.Body).Decode(&body) == nil {
				for _, p := range body.Value {
					if p.State != "enabled" || p.GrantControls == nil {
						continue
					}
					for _, ctrl := range p.GrantControls.BuiltInControls {
						if strings.EqualFold(ctrl, "mfa") {
							result.MFAPolicyEnforced = true
							break
						}
					}
				}
			}
		}
	}

	return json.Marshal(result)
}

// azureOAuth2Token acquires an OAuth2 client-credentials token from Azure AD.
func azureOAuth2Token(ctx context.Context, tenantID, clientID, clientSecret, scope string) (string, error) {
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenantID)
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"scope":         {scope},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := collectorHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	var tok struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("azure token error: %s — %s", tok.Error, tok.ErrorDesc)
	}
	return tok.AccessToken, nil
}
