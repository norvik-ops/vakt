package config

import (
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	DBUrl          string
	RedisUrl       string
	SecretKey      string
	APIPort        string
	InternalPort   string
	ModulesEnabled string
	AutoMigrate    bool
	DemoSeed       bool
	Version        string
	SMTPHost       string
	SMTPPort       string
	SMTPUser       string
	SMTPPass       string
	SMTPFrom       string
	// AI reports — OpenAI-compatible provider (default: local Ollama).
	// Provider "openai" works with any OpenAI-compatible endpoint (Ollama, Mistral, etc.).
	// Set VAKT_AI_PROVIDER=disabled to disable. Default: "ollama" (local Ollama via OpenAI-compatible API).
	AIProvider string // "ollama" | "openai" | "disabled"
	AIBaseURL  string // e.g. "https://api.mistral.ai/v1" or "http://ollama:11434/v1"
	AIAPIKey   string // optional — leave empty for local providers (Ollama, LM Studio)
	AIModel    string // e.g. "mistral-small-latest", "gpt-4o-mini", "llama3.2"
	// Sprint 15: AI-Härtung.
	// AIRateLimitRPM     — max AI-Calls pro Minute pro Org (Token-Bucket, Redis-backed). 0 = aus.
	// AIDailyTokenLimit  — pro Org pro Kalendertag (UTC). 0 = aus.
	// AICacheTTLSeconds  — Response-Cache-TTL (sha256(model+prompt) → cached body). 0 = aus.
	// AICostPerMTokenIn/Out (in Mikro-EUR pro 1M Tokens) — für Kosten-Tracking. Lokales Ollama = 0.
	AIRateLimitRPM         int
	AIDailyTokenLimit      int
	AICacheTTLSeconds      int
	AIReportTimeoutSeconds int   // HTTP client timeout for AI report generation (default 120s)
	AICostPerMTokenIn      int64 // micro-EUR per 1M input tokens
	AICostPerMTokenOut     int64 // micro-EUR per 1M output tokens
	CasdoorURL             string
	CasdoorClientID        string
	CasdoorClientSecret    string
	FrontendURL            string
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
	// LicenseToken enables automatic license renewal. When set, the instance polls
	// api.norvikops.de/api/v1/billing/license/:token every 24h and activates the
	// returned key silently — no admin action required on renewal.
	// Set VAKT_LICENSE_TOKEN to the renewal token from your purchase email.
	LicenseToken string
	// LicenseRefreshURL overrides the default refresh endpoint (https://api.norvikops.de).
	// Only useful for testing.
	LicenseRefreshURL string

	// LicenseAutoRenew (VAKT_LICENSE_AUTORENEW, default true).
	//
	// When on, a Pro instance fetches its next licence key by itself — but ONLY in the
	// last quarter of the current key's life. A yearly customer's instance therefore
	// contacts api.norvikops.de roughly once a year, a monthly one a few times per
	// renewal, and never otherwise. It sends the renewal token carried inside the
	// signed key, and nothing else.
	//
	// Set to false and the instance never contacts us at all; we mail the key instead.
	// That path is supported, not a punishment — an air-gapped ISMS is a legitimate
	// thing to run, and it is half of why people buy this.
	//
	// The Community Edition has no key and never calls, regardless of this setting.
	LicenseAutoRenew bool
	// ECDSA private key PEM for signing license keys on purchase (VAKT_LICENSE_PRIVATE_KEY).
	LicensePrivateKey string

	// ── Direct sale via Lexware Office (billing instance only) ──────────────
	// A customer's self-hosted Vakt leaves all three empty and the direct-sale
	// routes stay dark. Only api.norvikops.de sets them.

	// SMTPReplyTo (VAKT_SMTP_REPLY_TO) — the address a customer reaches when they
	// hit "Reply" on an invoice or license mail. The From address is bound to the
	// SMTP login (Proton only sends as addresses that exist in the account), so
	// Reply-To is the reliable way to be reachable without a new mailbox.
	SMTPReplyTo string

	// Lexware Office API key (VAKT_LEXWARE_API_KEY). Treat as a credential:
	// it can read contacts and create invoices. Expires after 24 months, and
	// rotating it DELETES all event subscriptions — they are re-created at boot.
	LexwareAPIKey string
	// Public base URL of this billing API (VAKT_BILLING_BASE_URL), e.g.
	// https://api.norvikops.de. Used to build the approval link and the webhook
	// callback URL that Lexware calls back on.
	BillingBaseURL string
	// Where "new quote request, approve?" mails go (VAKT_BILLING_NOTIFY_EMAIL).
	BillingNotifyEmail string

	// BillingSmallBusiness (VAKT_BILLING_SMALL_BUSINESS, default true) spiegelt
	// § 19 UStG. true = jede Rechnung geht als "vatfree" raus, ohne Fallunterscheidung
	// nach Land. false = das Land des Kunden entscheidet (Inland 19 %, EU-Ausland
	// Reverse Charge, Drittland nicht steuerbar) — siehe internal/billing/lexware/tax.go.
	//
	// Der Wert MUSS zu dem passen, wie Lexware den Mandanten führt. Live geprüft
	// (2026-07-19): Ein als Kleinunternehmer geführter Mandant lehnt JEDEN anderen
	// taxType mit HTTP 406 ab — ein einseitiges Umlegen erzeugt also keine falschen
	// Rechnungen, sondern gar keine. VerifyTaxStatus() prüft das beim Start.
	BillingSmallBusiness bool

	// BillingVATID (VAKT_BILLING_VAT_ID) ist die EIGENE USt-IdNr.
	//
	// Zwei Dinge hängen daran, und beide erst ab der Regelbesteuerung: Sie gehört als
	// Pflichtangabe auf jede Reverse-Charge-Rechnung, und sie ist Voraussetzung, um bei
	// VIES QUALIFIZIERT anfragen zu dürfen (mit Name/Anschrift — nur das trägt als
	// Nachweis). Leer = nur die einfache Gültigkeitsprüfung ist möglich.
	BillingVATID string

	// PortalBaseURL (VAKT_PORTAL_BASE_URL) is where a customer's licence portal lives.
	//
	// A separate host from the API on purpose: a HUMAN opens this link, and
	// "api.norvikops.de/api/v1/billing/portal/<64 hex chars>" is not a link you send a
	// customer. It is also deliberately NOT called msp.* — that name belongs to the
	// MSP dashboard, a different product, and the whole point of ADR-0071 is that a
	// licence portal is not that. Single-seat customers use this too.
	PortalBaseURL string
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
	// (comma-separated). Defaults to ["http://localhost", "http://localhost:5173"] when VAKT_CORS_ORIGINS is not set.
	CORSOrigins []string
	// MetricsEnabled controls whether the /metrics endpoint is registered.
	// Set VAKT_METRICS_ENABLED=true to expose Prometheus metrics (still IP-allowlisted).
	MetricsEnabled bool
	// EPSSEnabled controls whether findings are enriched with EPSS scores from
	// api.first.org. Disabled by default because enrichment sends CVE IDs to an
	// external third-party service, which contradicts the self-hosted data-privacy
	// promise. Set VAKT_EPSS_ENABLED=true to opt in.
	EPSSEnabled bool
	// BSIFeedEnabled controls whether Vakt fetches the daily BSI CERT-Bund RSS feed
	// (https://www.bsi.bund.de/…/RSSNewsfeed_WarnMeldungen.xml). Enabled by default.
	// Set VAKT_BSI_FEED_ENABLED=false to disable in air-gapped environments or
	// when the outbound connection to bsi.bund.de is not permitted.
	BSIFeedEnabled bool

	// EOLCheckEnabled controls whether Vakt asks endoflife.date whether a component
	// from an SBOM scan is past its end of life.
	//
	// This is the only outbound call that carries anything ABOUT the customer: the
	// name of a software component they run (e.g. "openssl", "postgresql"). Not their
	// compliance data, but not nothing either — it says something about their stack.
	// It was undeclared until 2026-07-12, while SECURITY.md called its table of
	// outbound connections "complete". For a product sold on data sovereignty, a
	// falsifiable claim is more dangerous than the connection itself: it dies in the
	// first customer's firewall review.
	//
	// Default on (the feature is useless without it), opt-out for air-gapped setups —
	// exactly like the BSI feed.
	EOLCheckEnabled bool
	// ForceSecureCookies forces the Secure attribute on all session/CSRF cookies
	// regardless of the request's TLS state or X-Forwarded-Proto header. Default
	// false (Secure is inferred from TLS/XFP). Set VAKT_FORCE_SECURE_COOKIES=true
	// in production behind a TLS-terminating proxy as a hard safety net against a
	// misconfigured proxy that drops X-Forwarded-Proto (S87-5, F-07, CWE-614).
	ForceSecureCookies bool
}

// Validate checks that all required environment variables are present and
// well-formed. Call this immediately after Load() in cmd/* entrypoints.
// Returns a descriptive error so operators know exactly which variable to fix.
func (c *Config) Validate() error {
	if c.DBUrl == "" {
		return fmt.Errorf("VAKT_DB_URL is required but not set — see .env.example")
	}
	if c.RedisUrl == "" {
		return fmt.Errorf("VAKT_REDIS_URL is required but not set — see .env.example")
	}
	if c.SecretKey == "" {
		return fmt.Errorf("VAKT_SECRET_KEY is required but not set — generate with: openssl rand -hex 32")
	}
	// Minimum length: 32 bytes = 64 hex characters.
	// hex.DecodeString already validated this in Load() if the key is set,
	// but we defend-in-depth here in case Validate() is called independently.
	keyBytes, err := hex.DecodeString(c.SecretKey)
	if err != nil {
		return fmt.Errorf("VAKT_SECRET_KEY is not valid hex: %w", err)
	}
	if len(keyBytes) < 32 {
		return fmt.Errorf("VAKT_SECRET_KEY must be at least 32 bytes (64 hex chars), got %d bytes — regenerate with: openssl rand -hex 32", len(keyBytes))
	}
	allSame := true
	for _, b := range keyBytes[1:] {
		if b != keyBytes[0] {
			allSame = false
			break
		}
	}
	if allSame {
		return fmt.Errorf("VAKT_SECRET_KEY is cryptographically weak: all %d bytes are identical (0x%02x) — regenerate with: openssl rand -hex 32", len(keyBytes), keyBytes[0])
	}
	distinctBytes := make(map[byte]struct{})
	for _, b := range keyBytes {
		distinctBytes[b] = struct{}{}
	}
	if len(distinctBytes) < 16 {
		return fmt.Errorf("VAKT_SECRET_KEY has insufficient entropy (< 16 distinct bytes) — use a cryptographically random key")
	}
	return nil
}

// IsModuleEnabled reports whether the named module (e.g. "vaktscan") appears in
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

// readEnvOrFile reads a secret from a file path (fileKey) or falls back to
// the plain env var (envKey). Use VAKT_SECRET_KEY_FILE / VAKT_DB_URL_FILE in
// production to keep plaintext secrets out of `docker inspect` output.
func readEnvOrFile(envKey, fileKey string) (string, error) {
	if f := os.Getenv(fileKey); f != "" {
		if !strings.HasPrefix(f, "/") {
			return "", fmt.Errorf("%s must be an absolute path, got %q", fileKey, f)
		}
		b, err := os.ReadFile(f) // #nosec G703 — operator-controlled path, not user input
		if err != nil {
			return "", fmt.Errorf("cannot read %s=%q: %w", fileKey, f, err)
		}
		return strings.TrimSpace(string(b)), nil
	}
	return os.Getenv(envKey), nil
}

// getEnvInt parst eine Integer-Env-Var; bei Fehler oder leerem Wert wird der
// Default zurueckgegeben. Sprint 15 (S15-1/2/3) nutzt das fuer numerische
// Rate-/Quota-/Cache-Konfiguration.
func getEnvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func getEnvInt64(key string, def int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}

// Load reads configuration from environment variables with explicit validation.
func Load() (*Config, error) {
	dbURL, err := readEnvOrFile("VAKT_DB_URL", "VAKT_DB_URL_FILE")
	if err != nil {
		return nil, err
	}
	secretKey, err := readEnvOrFile("VAKT_SECRET_KEY", "VAKT_SECRET_KEY_FILE")
	if err != nil {
		return nil, err
	}
	licenseKey, err := readEnvOrFile("VAKT_LICENSE_KEY", "VAKT_LICENSE_KEY_FILE")
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		DBUrl:                  dbURL,
		RedisUrl:               getEnv("VAKT_REDIS_URL", ""),
		SecretKey:              secretKey,
		APIPort:                getEnv("VAKT_API_PORT", "8080"),
		InternalPort:           getEnv("VAKT_INTERNAL_PORT", "8081"),
		ModulesEnabled:         getEnv("VAKT_MODULES_ENABLED", "vaktscan,vaktcomply,vaktvault,vaktaware,vaktprivacy,vakthr"),
		AutoMigrate:            getEnv("AUTO_MIGRATE", "false") == "true",
		DemoSeed:               getEnv("VAKT_DEMO", "false") == "true",
		Version:                getEnv("APP_VERSION", "0.1.0"),
		SMTPHost:               getEnv("VAKT_SMTP_HOST", "localhost"),
		SMTPPort:               getEnv("VAKT_SMTP_PORT", "1025"),
		SMTPUser:               getEnv("VAKT_SMTP_USER", ""),
		SMTPPass:               getEnv("VAKT_SMTP_PASS", ""),
		SMTPFrom:               getEnv("VAKT_SMTP_FROM", "noreply@vakt.local"),
		AIProvider:             getEnv("VAKT_AI_PROVIDER", "ollama"),
		AIBaseURL:              getEnv("VAKT_AI_BASE_URL", "http://ollama:11434/v1"),
		AIAPIKey:               getEnv("VAKT_AI_API_KEY", ""),
		AIModel:                getEnv("VAKT_AI_MODEL", "qwen2.5:7b"),
		AIRateLimitRPM:         getEnvInt("VAKT_AI_RATE_LIMIT_RPM", 30),
		AIDailyTokenLimit:      getEnvInt("VAKT_AI_DAILY_TOKEN_LIMIT_PER_ORG", 0),
		AICacheTTLSeconds:      getEnvInt("VAKT_AI_CACHE_TTL_SECONDS", 3600),
		AIReportTimeoutSeconds: getEnvInt("VAKT_AI_REPORT_TIMEOUT", 120),
		AICostPerMTokenIn:      getEnvInt64("VAKT_AI_COST_PER_MTOKEN_IN_MICRO_EUR", 0),
		AICostPerMTokenOut:     getEnvInt64("VAKT_AI_COST_PER_MTOKEN_OUT_MICRO_EUR", 0),
		CasdoorURL:             getEnv("CASDOOR_URL", ""),
		CasdoorClientID:        getEnv("CASDOOR_CLIENT_ID", ""),
		CasdoorClientSecret:    getEnv("CASDOOR_CLIENT_SECRET", ""),
		FrontendURL:            getEnv("VAKT_FRONTEND_URL", "http://localhost:5173"),
		LDAPUrl:                getEnv("VAKT_LDAP_URL", ""),
		LDAPBindDN:             getEnv("VAKT_LDAP_BIND_DN", ""),
		LDAPBindPass:           getEnv("VAKT_LDAP_BIND_PASS", ""),
		LDAPBaseDN:             getEnv("VAKT_LDAP_BASE_DN", ""),
		LDAPUserFilter:         getEnv("VAKT_LDAP_USER_FILTER", "(objectClass=person)"),
		LDAPGroupFilter:        getEnv("VAKT_LDAP_GROUP_FILTER", "(objectClass=group)"),
		LDAPTLS:                getEnv("VAKT_LDAP_TLS", "false") == "true",
		// Absolute, not "./data/uploads": the distroless final image sets no WORKDIR
		// (CWD is "/"), so a relative default resolved to /data/uploads while the
		// named volume was mounted at /app/data/uploads — uploads went to the
		// ephemeral image layer and vanished on every container recreation/upgrade
		// (R-C04/S131-E1). The image chowns /data to the nonroot UID; the volume now
		// mounts at /data/uploads to match.
		UploadDir:          getEnv("VAKT_UPLOAD_DIR", "/data/uploads"),
		LicenseKey:         licenseKey,
		LicenseToken:       getEnv("VAKT_LICENSE_TOKEN", ""),
		LicenseRefreshURL:  getEnv("VAKT_LICENSE_REFRESH_URL", ""),
		LicenseAutoRenew:   getEnv("VAKT_LICENSE_AUTORENEW", "true") == "true",
		LicensePrivateKey:  getEnv("VAKT_LICENSE_PRIVATE_KEY", ""),
		LexwareAPIKey:      getEnv("VAKT_LEXWARE_API_KEY", ""),
		SMTPReplyTo:        getEnv("VAKT_SMTP_REPLY_TO", ""),
		BillingBaseURL:     getEnv("VAKT_BILLING_BASE_URL", ""),
		BillingNotifyEmail: getEnv("VAKT_BILLING_NOTIFY_EMAIL", ""),
		// Default "true" mit Absicht: Ein fehlendes oder vertipptes Flag darf NIE zur
		// Regelbesteuerung führen. Der teure Fehler liegt in der anderen Richtung.
		BillingSmallBusiness: getEnv("VAKT_BILLING_SMALL_BUSINESS", "true") == "true",
		BillingVATID:         getEnv("VAKT_BILLING_VAT_ID", ""),
		PortalBaseURL:        getEnv("VAKT_PORTAL_BASE_URL", "https://lizenz.norvikops.de"),
		UpdateCheck:          getEnv("VAKT_UPDATE_CHECK", "false") == "true",
		Staging:              getEnv("VAKT_STAGING", "false") == "true",
		PromoteURL:           getEnv("VAKT_PROMOTE_URL", "http://host.docker.internal:9099/promote"),
		PromoteSecret:        getEnv("VAKT_PROMOTE_SECRET", ""),
		// Sprint 15 S15-11: Prometheus-Metrics default-on. Vorher war
		// VAKT_METRICS_ENABLED=false der Default — Operatoren mussten erst
		// einen Schalter umlegen. Jetzt ist der Endpoint immer aktiv (IP-
		// allowlisted auf Loopback + Docker-Netz), opt-out via
		// VAKT_METRICS_DISABLED=true wenn jemand das explizit nicht will.
		MetricsEnabled:     getEnv("VAKT_METRICS_DISABLED", "false") != "true",
		EPSSEnabled:        getEnv("VAKT_EPSS_ENABLED", "false") == "true",
		BSIFeedEnabled:     getEnv("VAKT_BSI_FEED_ENABLED", "true") == "true",
		EOLCheckEnabled:    getEnv("VAKT_EOL_CHECK_ENABLED", "true") == "true",
		ForceSecureCookies: getEnv("VAKT_FORCE_SECURE_COOKIES", "false") == "true",
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
		cfg.CORSOrigins = []string{"http://localhost", "http://localhost:5173"}
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

	// S13-1 SSRF-Guard fuer VAKT_AI_BASE_URL.
	// Nur wenn AI aktiviert ist — disabled darf alles bleiben.
	if cfg.AIProvider != "" && cfg.AIProvider != "disabled" {
		if err := validateAIBaseURL(cfg.AIBaseURL); err != nil {
			return nil, fmt.Errorf("VAKT_AI_BASE_URL rejected: %w", err)
		}
	}

	return cfg, nil
}

// validateAIBaseURL lehnt URLs ab, die auf interne Cloud-Metadata-Endpunkte
// (169.254.169.254 — AWS/GCP/Azure IMDS), Loopback-Adressen oder
// link-local Bereiche zeigen, wenn AI-Provider aktiviert ist. Der Default
// "http://ollama:11434/v1" bleibt erlaubt: der Hostname "ollama" wird
// in einem Container-Netz von Docker/K8s zu einer RFC1918-Adresse
// aufgeloest, das Allowlist-Exception erlaubt diese explizit.
//
// Eingaberegeln:
//   - Schema muss http oder https sein.
//   - Hostname darf KEIN bare IP aus 127.0.0.0/8, 169.254.0.0/16, ::1, fe80::/10 sein.
//   - Hostname "localhost" wird abgelehnt.
//   - Service-Discovery-Hostnames (ollama, ai-llm, llm-proxy) sind explizit
//     erlaubt — sie loesen im Container-Netz typischerweise zu RFC1918 auf.
//   - Andere Hostnamen + Public-IPs werden durchgelassen (Cloud-LLMs wie
//     api.openai.com, api.mistral.ai etc.).
func validateAIBaseURL(raw string) error {
	if raw == "" {
		return fmt.Errorf("empty when AI provider is enabled")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("not a valid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("scheme must be http or https (got %q)", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("missing host")
	}

	// Allowlist: Service-Discovery-Namen, die in Container-Netzen zu
	// internen Adressen aufloesen sollen. Bewusst eng gehalten.
	allowedServiceNames := map[string]bool{
		"ollama":    true,
		"ai-llm":    true,
		"llm-proxy": true,
		"lm-studio": true,
	}
	if allowedServiceNames[strings.ToLower(host)] {
		return nil
	}

	// localhost ist immer ein Konfig-Fehler: das API-Container-Image kann
	// localhost nicht zum Host-Loopback aufloesen.
	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("hostname \"localhost\" not allowed — use the docker service name (e.g. \"ollama\") or a public DNS name")
	}

	// Wenn der Host eine bare IP ist, gegen Block-Liste pruefen.
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return fmt.Errorf("IP address %s is blocked (loopback, link-local, or cloud-metadata range) — set VAKT_AI_BASE_URL to a service name or public DNS instead", host)
		}
	}

	return nil
}

// isBlockedIP gibt true zurueck wenn die IP zu einem Bereich gehoert, der
// nie ein legitimes AI-Backend sein kann (IMDS, Loopback, Link-Local).
func isBlockedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	// AWS/GCP/Azure Instance Metadata Service.
	imds := net.IPv4(169, 254, 169, 254)
	return ip.Equal(imds)
}
