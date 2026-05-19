package config

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	DBUrl                string
	RedisUrl             string
	SecretKey            string
	APIPort              string
	ModulesEnabled       string
	AutoMigrate          bool
	DemoSeed             bool
	Version              string
	SMTPHost             string
	SMTPPort             string
	SMTPUser             string
	SMTPPass             string
	SMTPFrom             string
	// AI reports — OpenAI-compatible provider (disabled by default).
	// Provider "openai" works with OpenAI, Mistral, Groq, Ollama (/v1), LM Studio, vLLM, etc.
	AIProvider string // "disabled" | "openai"
	AIBaseURL  string // e.g. "https://api.mistral.ai/v1" or "http://ollama:11434/v1"
	AIAPIKey   string // optional — leave empty for local providers (Ollama, LM Studio)
	AIModel    string // e.g. "mistral-small-latest", "gpt-4o-mini", "llama3.2"
	CasdoorURL           string
	CasdoorClientID      string
	CasdoorClientSecret  string
	FrontendURL          string
	// LDAP/AD sync
	LDAPUrl         string
	LDAPBindDN      string
	LDAPBindPass    string
	LDAPBaseDN      string
	LDAPUserFilter  string
	LDAPGroupFilter string
	LDAPTLS         bool
	// Upload directory for user-uploaded files (evidence attachments, etc.)
	UploadDir string
	// License key (base64url payload + "." + base64url signature).
	// Leave empty for Community Edition. Set VAKT_DEMO=true to enable all features without a key.
	LicenseKey string
	// LemonSqueezy webhook signing secret (VAKT_LS_WEBHOOK_SECRET).
	LSWebhookSecret string
	// ECDSA private key PEM for signing license keys on purchase (VAKT_LICENSE_PRIVATE_KEY).
	LicensePrivateKey string
	// UpdateCheck — opt-in check against GitHub releases API once per day.
	// Set VAKT_UPDATE_CHECK=true to enable. No data is sent; only a GET request to the public GitHub API.
	UpdateCheck bool
	// Staging mode — set VAKT_STAGING=true on the staging instance only.
	// Enables the "Promote to Demo" UI and API endpoint.
	Staging bool
	// PromoteURL is the local webhook URL that triggers staging → demo promotion.
	// Defaults to http://host.docker.internal:9099/promote (set via VAKT_PROMOTE_URL).
	PromoteURL string
	// PromoteSecret is the shared secret sent in X-Promote-Secret header.
	PromoteSecret string
	// CORSOrigins is the list of allowed CORS origins loaded from VAKT_CORS_ORIGINS
	// (comma-separated). Defaults to ["*"] when not set, preserving dev behaviour.
	CORSOrigins []string
	// MetricsEnabled controls whether the /metrics endpoint is registered.
	// Set VAKT_METRICS_ENABLED=true to expose Prometheus metrics (still IP-allowlisted).
	MetricsEnabled bool
}

// IsModuleEnabled reports whether the named module (e.g. "secpulse") appears in
// the ModulesEnabled CSV list.  Comparison is case-insensitive.
func (c *Config) IsModuleEnabled(name string) bool {
	for _, mod := range strings.Split(c.ModulesEnabled, ",") {
		if strings.EqualFold(strings.TrimSpace(mod), name) {
			return true
		}
	}
	return false
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Load reads configuration from environment variables with explicit validation.
func Load() (*Config, error) {
	cfg := &Config{
		DBUrl:          getEnv("VAKT_DB_URL", ""),
		RedisUrl:       getEnv("VAKT_REDIS_URL", ""),
		SecretKey:      getEnv("VAKT_SECRET_KEY", ""),
		APIPort:        getEnv("VAKT_API_PORT", "8080"),
		ModulesEnabled: getEnv("VAKT_MODULES_ENABLED", "secpulse,secvitals,secvault,secreflex,secprivacy"),
		AutoMigrate:    getEnv("AUTO_MIGRATE", "false") == "true",
		DemoSeed:       getEnv("VAKT_DEMO", "false") == "true",
		Version:        getEnv("APP_VERSION", "0.1.0"),
		SMTPHost:       getEnv("VAKT_SMTP_HOST", "localhost"),
		SMTPPort:       getEnv("VAKT_SMTP_PORT", "1025"),
		SMTPUser:       getEnv("VAKT_SMTP_USER", ""),
		SMTPPass:       getEnv("VAKT_SMTP_PASS", ""),
		SMTPFrom:       getEnv("VAKT_SMTP_FROM", "noreply@vakt.local"),
		AIProvider: getEnv("VAKT_AI_PROVIDER", "disabled"),
		AIBaseURL:  getEnv("VAKT_AI_BASE_URL", "http://ollama:11434/v1"),
		AIAPIKey:   getEnv("VAKT_AI_API_KEY", ""),
		AIModel:    getEnv("VAKT_AI_MODEL", "llama3.2:3b"),
		CasdoorURL:          getEnv("CASDOOR_URL", ""),
		CasdoorClientID:     getEnv("CASDOOR_CLIENT_ID", ""),
		CasdoorClientSecret: getEnv("CASDOOR_CLIENT_SECRET", ""),
		FrontendURL:         getEnv("VAKT_FRONTEND_URL", "http://localhost:5173"),
		LDAPUrl:         getEnv("VAKT_LDAP_URL", ""),
		LDAPBindDN:      getEnv("VAKT_LDAP_BIND_DN", ""),
		LDAPBindPass:    getEnv("VAKT_LDAP_BIND_PASS", ""),
		LDAPBaseDN:      getEnv("VAKT_LDAP_BASE_DN", ""),
		LDAPUserFilter:  getEnv("VAKT_LDAP_USER_FILTER", "(objectClass=person)"),
		LDAPGroupFilter: getEnv("VAKT_LDAP_GROUP_FILTER", "(objectClass=group)"),
		LDAPTLS:         getEnv("VAKT_LDAP_TLS", "false") == "true",
		UploadDir:       getEnv("VAKT_UPLOAD_DIR", "./data/uploads"),
		LicenseKey:        getEnv("VAKT_LICENSE_KEY", ""),
		LSWebhookSecret:   getEnv("VAKT_LS_WEBHOOK_SECRET", ""),
		LicensePrivateKey: getEnv("VAKT_LICENSE_PRIVATE_KEY", ""),
		UpdateCheck:    getEnv("VAKT_UPDATE_CHECK", "false") == "true",
		Staging:        getEnv("VAKT_STAGING", "false") == "true",
		PromoteURL:     getEnv("VAKT_PROMOTE_URL", "http://host.docker.internal:9099/promote"),
		PromoteSecret:  getEnv("VAKT_PROMOTE_SECRET", ""),
		MetricsEnabled: getEnv("VAKT_METRICS_ENABLED", "false") == "true",
	}

	// CORS origins — default to wildcard to preserve dev behaviour.
	if raw := os.Getenv("VAKT_CORS_ORIGINS"); raw != "" {
		var origins []string
		for _, o := range strings.Split(raw, ",") {
			if trimmed := strings.TrimSpace(o); trimmed != "" {
				origins = append(origins, trimmed)
			}
		}
		if len(origins) > 0 {
			cfg.CORSOrigins = origins
		}
	}
	if len(cfg.CORSOrigins) == 0 {
		cfg.CORSOrigins = []string{"*"}
	}

	if cfg.APIPort == "" {
		return nil, fmt.Errorf("VAKT_API_PORT must not be empty")
	}

	if cfg.SecretKey != "" {
		keyBytes, err := hex.DecodeString(cfg.SecretKey)
		if err != nil {
			return nil, fmt.Errorf("VAKT_SECRET_KEY is not valid hex: %w", err)
		}
		if len(keyBytes) != 32 {
			return nil, fmt.Errorf("VAKT_SECRET_KEY must be exactly 32 bytes (64 hex chars), got %d bytes — regenerate with: openssl rand -hex 32", len(keyBytes))
		}
	}

	return cfg, nil
}
