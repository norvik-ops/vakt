package vaktcomply

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCollectorInterface verifies that all registered collectors implement the Collector interface.
func TestCollectorInterface(t *testing.T) {
	var _ Collector = &GitHubCollector{}
	var _ Collector = &AWSCollector{}
	var _ Collector = &AzureCollector{}
}

// TestCollectorNames verifies each collector reports the correct source name.
func TestCollectorNames(t *testing.T) {
	assert.Equal(t, "github", (&GitHubCollector{}).Name())
	assert.Equal(t, "aws", (&AWSCollector{}).Name())
	assert.Equal(t, "azure", (&AzureCollector{}).Name())
}

// TestGetCollector_Known verifies registered sources are resolved correctly.
func TestGetCollector_Known(t *testing.T) {
	for _, source := range []string{"github", "aws", "azure"} {
		c, err := GetCollector(source)
		require.NoError(t, err, "source: %s", source)
		assert.NotNil(t, c)
		assert.Equal(t, source, c.Name())
	}
}

// TestGetCollector_Unknown verifies an error is returned for unknown sources.
func TestGetCollector_Unknown(t *testing.T) {
	_, err := GetCollector("somerandominvalidcollector")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown collector source")
}

// TestGitHubCollector_MissingParams verifies validation of required params.
func TestGitHubCollector_MissingParams(t *testing.T) {
	c := &GitHubCollector{}
	cfg := CollectorConfig{
		Type:   "github",
		Params: map[string]string{}, // missing token, owner, repo
	}
	_, err := c.Collect(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token")
}

// TestAWSCollector_DefaultRegion verifies AWSCollector falls back to a default region.
func TestAWSCollector_DefaultRegion(t *testing.T) {
	c := &AWSCollector{}
	cfg := CollectorConfig{
		Type:   "aws",
		Params: map[string]string{}, // no region provided
	}
	data, err := c.Collect(context.Background(), cfg)
	require.NoError(t, err)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &result))
	assert.Equal(t, "us-east-1", result["region"])
}

// TestAWSCollector_OutputSchema verifies the JSON output contains expected fields.
func TestAWSCollector_OutputSchema(t *testing.T) {
	c := &AWSCollector{}
	cfg := CollectorConfig{
		Type:   "aws",
		Params: map[string]string{"region": "eu-west-1"},
	}
	data, err := c.Collect(context.Background(), cfg)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &result))

	assert.Contains(t, result, "collected_at")
	assert.Contains(t, result, "region")
	assert.Contains(t, result, "cloudtrail_enabled")
	assert.Contains(t, result, "s3_default_encryption")
	assert.Contains(t, result, "iam_password_policy_set")
	assert.Equal(t, "eu-west-1", result["region"])
}

// TestAzureCollector_MissingTenantID verifies validation of required params.
func TestAzureCollector_MissingTenantID(t *testing.T) {
	c := &AzureCollector{}
	cfg := CollectorConfig{
		Type:   "azure",
		Params: map[string]string{}, // missing tenant_id
	}
	_, err := c.Collect(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant_id")
}

// TestAzureCollector_OutputSchema verifies the JSON output contains expected fields.
func TestAzureCollector_OutputSchema(t *testing.T) {
	c := &AzureCollector{}
	cfg := CollectorConfig{
		Type:   "azure",
		Params: map[string]string{"tenant_id": "my-tenant-id"},
	}
	data, err := c.Collect(context.Background(), cfg)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &result))

	assert.Contains(t, result, "collected_at")
	assert.Contains(t, result, "tenant_id")
	assert.Contains(t, result, "security_center_enabled")
	assert.Contains(t, result, "mfa_policy_enforced")
	assert.Equal(t, "my-tenant-id", result["tenant_id"])
}

// TestCollectorConfig_ParamsMap verifies that CollectorConfig holds arbitrary params.
func TestCollectorConfig_ParamsMap(t *testing.T) {
	cfg := CollectorConfig{
		Type: "github",
		Params: map[string]string{
			"token": "ghp_test",
			"owner": "myorg",
			"repo":  "myrepo",
		},
	}
	assert.Equal(t, "ghp_test", cfg.Params["token"])
	assert.Equal(t, "myorg", cfg.Params["owner"])
	assert.Equal(t, "myrepo", cfg.Params["repo"])
}
