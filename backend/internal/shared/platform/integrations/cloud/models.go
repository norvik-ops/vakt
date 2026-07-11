// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package cloud provides automated evidence collection from cloud providers.
package cloud

import "time"

// Provider constants for cloud_integrations.provider.
const (
	ProviderAWS        = "aws"
	ProviderAzure      = "azure"
	ProviderHetzner    = "hetzner"
	ProviderIONOS      = "ionos"
	ProviderWazuh      = "wazuh"
	ProviderPrometheus = "prometheus"
	ProviderEntraID    = "entra_id"
	ProviderKeycloak   = "keycloak"
	ProviderLDAP       = "ldap"
	ProviderGitLab     = "gitlab"
	ProviderSonarQube  = "sonarqube"
	ProviderPersonio   = "personio"
	ProviderIntune     = "intune"
)

// AWSConfig holds AWS credentials and region for evidence collection.
// SecretAccessKey is stored encrypted (AES-256-GCM) inside cloud_integrations.config JSONB.
type AWSConfig struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"` // stored encrypted
	Region          string `json:"region"`
	AccountID       string `json:"account_id"`
}

// AzureConfig holds Azure service principal credentials for evidence collection.
// ClientSecret is stored encrypted (AES-256-GCM) inside cloud_integrations.config JSONB.
type AzureConfig struct {
	TenantID       string `json:"tenant_id"`
	ClientID       string `json:"client_id"`
	ClientSecret   string `json:"client_secret"` // stored encrypted
	SubscriptionID string `json:"subscription_id"`
}

// CloudIntegration represents a row from the cloud_integrations table.
type CloudIntegration struct {
	ID             string     `json:"id"`
	OrgID          string     `json:"org_id"`
	Provider       string     `json:"provider"` // "aws" | "azure"
	Enabled        bool       `json:"enabled"`
	LastSyncAt     *time.Time `json:"last_sync_at,omitempty"`
	LastSyncStatus *string    `json:"last_sync_status,omitempty"`
	LastSyncError  *string    `json:"last_sync_error,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// SaveAWSConfigInput is the validated HTTP input for saving AWS config.
type SaveAWSConfigInput struct {
	AccessKeyID     string `json:"access_key_id"     validate:"required"`
	SecretAccessKey string `json:"secret_access_key" validate:"required"`
	Region          string `json:"region"            validate:"required"`
	AccountID       string `json:"account_id"`
}

// SaveAzureConfigInput is the validated HTTP input for saving Azure config.
type SaveAzureConfigInput struct {
	TenantID       string `json:"tenant_id"       validate:"required"`
	ClientID       string `json:"client_id"       validate:"required"`
	ClientSecret   string `json:"client_secret"   validate:"required"`
	SubscriptionID string `json:"subscription_id" validate:"required"`
}

// AWSConfigResponse is returned from GET /config (secrets masked).
type AWSConfigResponse struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"` // "****" if set
	Region          string `json:"region"`
	AccountID       string `json:"account_id"`
	IsConfigured    bool   `json:"is_configured"`
}

// AzureConfigResponse is returned from GET /config (secrets masked).
type AzureConfigResponse struct {
	TenantID       string `json:"tenant_id"`
	ClientID       string `json:"client_id"`
	ClientSecret   string `json:"client_secret"` // "****" if set
	SubscriptionID string `json:"subscription_id"`
	IsConfigured   bool   `json:"is_configured"`
}

// SyncStatus is returned from GET /status.
type SyncStatus struct {
	Provider       string     `json:"provider"`
	Enabled        bool       `json:"enabled"`
	LastSyncAt     *time.Time `json:"last_sync_at,omitempty"`
	LastSyncStatus *string    `json:"last_sync_status,omitempty"`
	LastSyncError  *string    `json:"last_sync_error,omitempty"`
	EvidenceCount  int        `json:"evidence_count"`
}

// --- Hetzner ---

// HetznerConfig holds Hetzner API token for evidence collection.
type HetznerConfig struct {
	APIToken string `json:"api_token"` // stored encrypted
	Location string `json:"location"`  // optional location filter: "nbg1","fsn1","hel1"
}

// SaveHetznerConfigInput is the validated HTTP input for saving Hetzner config.
type SaveHetznerConfigInput struct {
	APIToken string `json:"api_token" validate:"required"`
	Location string `json:"location"`
}

// HetznerConfigResponse is returned from GET /hetzner/config (secret masked).
type HetznerConfigResponse struct {
	APIToken     string `json:"api_token"` // "****" if set
	Location     string `json:"location"`
	IsConfigured bool   `json:"is_configured"`
}

// HetznerStatus extends SyncStatus with server count.
type HetznerStatus struct {
	SyncStatus
	ServerCount int `json:"server_count"`
}

// --- IONOS ---

// IONOSConfig holds IONOS credentials for evidence collection.
type IONOSConfig struct {
	Username string `json:"username"` // stored encrypted
	Password string `json:"password"` // stored encrypted
	Token    string `json:"token"`    // alternative: API Token (optional, stored encrypted)
}

// SaveIONOSConfigInput is the validated HTTP input for saving IONOS config.
type SaveIONOSConfigInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Token    string `json:"token"`
}

// IONOSConfigResponse is returned from GET /ionos/config (secrets masked).
type IONOSConfigResponse struct {
	Username     string `json:"username"`
	Password     string `json:"password"` // "****" if set
	Token        string `json:"token"`    // "****" if set
	IsConfigured bool   `json:"is_configured"`
}

// IONOSStatus extends SyncStatus with server and datacenter counts.
type IONOSStatus struct {
	SyncStatus
	ServerCount     int `json:"server_count"`
	DatacenterCount int `json:"datacenter_count"`
}

// --- Wazuh ---

// WazuhConfig holds Wazuh REST API credentials.
type WazuhConfig struct {
	BaseURL   string `json:"base_url"`   // e.g. "https://wazuh-manager:55000"
	Username  string `json:"username"`   // stored encrypted
	Password  string `json:"password"`   // stored encrypted
	VerifyTLS bool   `json:"verify_tls"` // false = accept self-signed
	// AllowPrivateTarget is persisted (S121-F4 / F1-Inj) so the dial-time SSRF
	// guard can honour the admin's opt-in at request time, not just at save time.
	AllowPrivateTarget bool `json:"allow_private_target"`
}

// SaveWazuhConfigInput is the validated HTTP input for saving Wazuh config.
type SaveWazuhConfigInput struct {
	BaseURL            string `json:"base_url"            validate:"required"`
	Username           string `json:"username"            validate:"required"`
	Password           string `json:"password"            validate:"required"`
	VerifyTLS          bool   `json:"verify_tls"`
	AllowPrivateTarget bool   `json:"allow_private_target"` // allow RFC1918 targets (on-premises Wazuh)
}

// WazuhConfigResponse is returned from GET /wazuh/config (secrets masked).
type WazuhConfigResponse struct {
	BaseURL      string `json:"base_url"`
	Username     string `json:"username"`
	Password     string `json:"password"` // "****" if set
	VerifyTLS    bool   `json:"verify_tls"`
	IsConfigured bool   `json:"is_configured"`
}

// WazuhStatus extends SyncStatus with agent counts.
type WazuhStatus struct {
	SyncStatus
	AgentCount    int `json:"agent_count"`
	AgentsOffline int `json:"agents_offline"`
}

// --- Prometheus ---

// PrometheusConfig holds Prometheus/Alertmanager connection details.
type PrometheusConfig struct {
	PrometheusURL   string `json:"prometheus_url"`   // e.g. "http://prometheus:9090"
	AlertmanagerURL string `json:"alertmanager_url"` // optional
	Token           string `json:"token"`            // optional Bearer Token (stored encrypted)
	// AllowPrivateTarget is persisted (S121-F4 / F1-Inj) so the dial-time SSRF
	// guard can honour the admin's opt-in at request time, not just at save time.
	AllowPrivateTarget bool `json:"allow_private_target"`
}

// SavePrometheusConfigInput is the validated HTTP input for saving Prometheus config.
type SavePrometheusConfigInput struct {
	PrometheusURL      string `json:"prometheus_url"       validate:"required"`
	AlertmanagerURL    string `json:"alertmanager_url"`
	Token              string `json:"token"`
	AllowPrivateTarget bool   `json:"allow_private_target"` // allow RFC1918 targets (on-premises Prometheus)
}

// PrometheusConfigResponse is returned from GET /prometheus/config (secret masked).
type PrometheusConfigResponse struct {
	PrometheusURL   string `json:"prometheus_url"`
	AlertmanagerURL string `json:"alertmanager_url"`
	Token           string `json:"token"` // "****" if set
	IsConfigured    bool   `json:"is_configured"`
}

// PrometheusStatus extends SyncStatus with target and alert counts.
type PrometheusStatus struct {
	SyncStatus
	TargetCount      int `json:"target_count"`
	ActiveAlertCount int `json:"active_alert_count"`
}

// --- Entra ID ---

// EntraIDConfig holds Microsoft Entra ID (Azure AD) OAuth2 client credentials.
type EntraIDConfig struct {
	TenantID     string `json:"tenant_id"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"` // stored encrypted
}

// SaveEntraIDConfigInput is the validated HTTP input for saving Entra ID config.
type SaveEntraIDConfigInput struct {
	TenantID     string `json:"tenant_id"     validate:"required"`
	ClientID     string `json:"client_id"     validate:"required"`
	ClientSecret string `json:"client_secret" validate:"required"`
}

// EntraIDConfigResponse is returned from GET /entra-id/config (secrets masked).
type EntraIDConfigResponse struct {
	TenantID     string `json:"tenant_id"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"` // "****" if set
	IsConfigured bool   `json:"is_configured"`
}

// EntraIDStatus extends SyncStatus with identity metrics.
type EntraIDStatus struct {
	SyncStatus
	MFAEnrollmentPct  float64 `json:"mfa_enrollment_pct"`
	RiskyUserCount    int     `json:"risky_user_count"`
	InactiveUserCount int     `json:"inactive_user_count"`
}

// --- Intune (S88-7) ---

// IntuneConfig holds Graph API client-credentials for the customer's tenant.
type IntuneConfig struct {
	TenantID     string `json:"tenant_id"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"` // stored encrypted
}

// SaveIntuneConfigInput is the validated HTTP input for saving Intune config.
type SaveIntuneConfigInput struct {
	TenantID     string `json:"tenant_id"     validate:"required"`
	ClientID     string `json:"client_id"     validate:"required"`
	ClientSecret string `json:"client_secret" validate:"required"`
}

// IntuneConfigResponse is returned from GET /intune/config (secret masked).
type IntuneConfigResponse struct {
	TenantID     string `json:"tenant_id"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"` // "****" if set
	IsConfigured bool   `json:"is_configured"`
}

// IntuneStatus extends SyncStatus with device-posture metrics.
type IntuneStatus struct {
	SyncStatus
	DeviceCompliancePct float64 `json:"device_compliance_pct"`
}

// --- Keycloak ---

// KeycloakConfig holds Keycloak service-account credentials.
type KeycloakConfig struct {
	KeycloakURL  string `json:"keycloak_url"` // e.g. "https://keycloak.example.com"
	Realm        string `json:"realm"`        // e.g. "master" or "company"
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"` // stored encrypted
	// AllowPrivateTarget is persisted (S121-F4 / F1-Inj) so the dial-time SSRF
	// guard can honour the admin's opt-in at request time, not just at save time.
	AllowPrivateTarget bool `json:"allow_private_target"`
}

// SaveKeycloakConfigInput is the validated HTTP input for saving Keycloak config.
type SaveKeycloakConfigInput struct {
	KeycloakURL        string `json:"keycloak_url"        validate:"required"`
	Realm              string `json:"realm"               validate:"required"`
	ClientID           string `json:"client_id"           validate:"required"`
	ClientSecret       string `json:"client_secret"       validate:"required"`
	AllowPrivateTarget bool   `json:"allow_private_target"` // allow RFC1918 targets (on-premises Keycloak)
}

// KeycloakConfigResponse is returned from GET /keycloak/config (secrets masked).
type KeycloakConfigResponse struct {
	KeycloakURL  string `json:"keycloak_url"`
	Realm        string `json:"realm"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"` // "****" if set
	IsConfigured bool   `json:"is_configured"`
}

// KeycloakStatus extends SyncStatus with user metrics.
type KeycloakStatus struct {
	SyncStatus
	UserCount        int     `json:"user_count"`
	MFAEnrollmentPct float64 `json:"mfa_enrollment_pct"`
}

// --- LDAP ---

// LDAPConfig holds LDAP/Active-Directory connection parameters.
type LDAPConfig struct {
	Host              string   `json:"host"`
	Port              int      `json:"port"` // 389 or 636
	BindDN            string   `json:"bind_dn"`
	BindPassword      string   `json:"bind_password"` // stored encrypted
	BaseDN            string   `json:"base_dn"`
	UseTLS            bool     `json:"use_tls"`
	IsActiveDirectory bool     `json:"is_active_directory"` // affects timestamp parsing
	PrivilegedGroups  []string `json:"privileged_groups"`   // default: ["Domain Admins","Administrators"]
}

// SaveLDAPConfigInput is the validated HTTP input for saving LDAP config.
type SaveLDAPConfigInput struct {
	Host              string   `json:"host"          validate:"required"`
	Port              int      `json:"port"          validate:"required"`
	BindDN            string   `json:"bind_dn"       validate:"required"`
	BindPassword      string   `json:"bind_password" validate:"required"`
	BaseDN            string   `json:"base_dn"       validate:"required"`
	UseTLS            bool     `json:"use_tls"`
	IsActiveDirectory bool     `json:"is_active_directory"`
	PrivilegedGroups  []string `json:"privileged_groups"`
}

// LDAPConfigResponse is returned from GET /ldap/config (secrets masked).
type LDAPConfigResponse struct {
	Host              string   `json:"host"`
	Port              int      `json:"port"`
	BindDN            string   `json:"bind_dn"`
	BindPassword      string   `json:"bind_password"` // "****" if set
	BaseDN            string   `json:"base_dn"`
	UseTLS            bool     `json:"use_tls"`
	IsActiveDirectory bool     `json:"is_active_directory"`
	PrivilegedGroups  []string `json:"privileged_groups"`
	IsConfigured      bool     `json:"is_configured"`
}

// LDAPStatus extends SyncStatus with directory metrics.
type LDAPStatus struct {
	SyncStatus
	UserCount       int `json:"user_count"`
	InactiveCount   int `json:"inactive_count"`
	PrivilegedCount int `json:"privileged_count"`
}

// --- GitLab ---

// GitLabConfig holds GitLab API credentials for evidence collection.
type GitLabConfig struct {
	GitLabURL   string `json:"gitlab_url"`   // e.g. "https://gitlab.example.com" or "https://gitlab.com"
	AccessToken string `json:"access_token"` // stored encrypted
	GroupID     string `json:"group_id"`     // optional group namespace or numeric ID
	// AllowPrivateTarget is persisted (S121-F4 / F1-Inj) so the dial-time SSRF
	// guard can honour the admin's opt-in at request time, not just at save time.
	AllowPrivateTarget bool `json:"allow_private_target"`
}

// SaveGitLabConfigInput is the validated HTTP input for saving GitLab config.
type SaveGitLabConfigInput struct {
	GitLabURL          string `json:"gitlab_url"          validate:"required"`
	AccessToken        string `json:"access_token"        validate:"required"`
	GroupID            string `json:"group_id"`
	AllowPrivateTarget bool   `json:"allow_private_target"` // allow RFC1918 targets (self-hosted GitLab in private network)
}

// GitLabConfigResponse is returned from GET /gitlab/config (secrets masked).
type GitLabConfigResponse struct {
	GitLabURL    string `json:"gitlab_url"`
	AccessToken  string `json:"access_token"` // "****" if set
	GroupID      string `json:"group_id"`
	IsConfigured bool   `json:"is_configured"`
}

// GitLabStatus extends SyncStatus with project metrics.
type GitLabStatus struct {
	SyncStatus
	ProjectCount             int `json:"project_count"`
	UnprotectedBranchesCount int `json:"unprotected_branches_count"`
}

// --- SonarQube ---

// SonarQubeConfig holds SonarQube connection details for evidence collection.
type SonarQubeConfig struct {
	BaseURL string `json:"base_url"` // e.g. "https://sonarqube.example.com" or "https://sonarcloud.io"
	Token   string `json:"token"`    // stored encrypted
	// AllowPrivateTarget is persisted (S121-F4 / F1-Inj) so the dial-time SSRF
	// guard can honour the admin's opt-in at request time, not just at save time.
	AllowPrivateTarget bool `json:"allow_private_target"`
}

// SaveSonarQubeConfigInput is the validated HTTP input for saving SonarQube config.
type SaveSonarQubeConfigInput struct {
	BaseURL            string `json:"base_url"            validate:"required"`
	Token              string `json:"token"               validate:"required"`
	AllowPrivateTarget bool   `json:"allow_private_target"` // allow RFC1918 targets (self-hosted SonarQube)
}

// SonarQubeConfigResponse is returned from GET /sonarqube/config (secrets masked).
type SonarQubeConfigResponse struct {
	BaseURL      string `json:"base_url"`
	Token        string `json:"token"` // "****" if set
	IsConfigured bool   `json:"is_configured"`
}

// SonarQubeStatus extends SyncStatus with quality gate metrics.
type SonarQubeStatus struct {
	SyncStatus
	ProjectCount           int `json:"project_count"`
	QualityGateFailedCount int `json:"quality_gate_failed_count"`
	HotspotCount           int `json:"hotspot_count"`
}

// --- Personio ---

// PersonioConfig holds Personio webhook secret for HMAC verification.
// Personio is push-only: Vakt does not pull from Personio; Personio pushes employee.departed events.
type PersonioConfig struct {
	WebhookSecret string `json:"webhook_secret"` // stored encrypted
}

// SavePersonioConfigInput is the validated HTTP input for saving Personio config.
type SavePersonioConfigInput struct {
	WebhookSecret string `json:"webhook_secret" validate:"required"`
}

// PersonioConfigResponse is returned from GET /personio/config (secrets masked).
type PersonioConfigResponse struct {
	WebhookSecret string `json:"webhook_secret"` // "****" if set
	IsConfigured  bool   `json:"is_configured"`
}

// PersonioStatus is returned from GET /personio/status.
type PersonioStatus struct {
	SyncStatus
	WebhookURL            string `json:"webhook_url"`
	WebhookConfigured     bool   `json:"webhook_configured"`
	OffboardingsTriggered int    `json:"offboardings_triggered"`
	OffboardingsOnTime    int    `json:"offboardings_completed_on_time"`
}
