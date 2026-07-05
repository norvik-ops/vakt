package admin

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	sharedcrypto "github.com/matharnica/vakt/internal/shared/crypto"
	platformldap "github.com/matharnica/vakt/internal/shared/platform/ldap"
)

// GetOrgSMTPSettings handles GET /api/v1/admin/org/smtp.
// Returns the per-org SMTP configuration. The password is never exposed in plaintext;
// HasPass signals whether an encrypted password is currently stored.
func (h *Handler) GetOrgSMTPSettings(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	s, err := h.service.repo.GetOrgSMTPSettings(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("get org smtp settings failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve SMTP settings",
			"code":  "ADMIN_SMTP_SETTINGS_ERROR",
		})
	}
	return c.JSON(http.StatusOK, s)
}

// UpdateOrgSMTPSettingsInput is the request body for PUT /api/v1/admin/org/smtp.
type UpdateOrgSMTPSettingsInput struct {
	Host string `json:"host"`
	Port string `json:"port"`
	User string `json:"user"`
	Pass string `json:"pass"` // empty = keep existing password
	From string `json:"from"`
	TLS  bool   `json:"tls"`
}

// UpdateOrgSMTPSettings handles PUT /api/v1/admin/org/smtp.
// If Pass is non-empty it is encrypted with the master key and stored.
// If Pass is empty the existing encrypted password is kept unchanged.
func (h *Handler) UpdateOrgSMTPSettings(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	var in UpdateOrgSMTPSettingsInput
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid input",
			"code":  "ADMIN_BAD_REQUEST",
		})
	}

	var passEnc []byte
	if in.Pass != "" {
		if len(h.service.masterKey) == 0 {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "master key not configured",
				"code":  "ADMIN_NO_MASTER_KEY",
			})
		}
		var encErr error
		passEnc, encErr = sharedcrypto.Encrypt(h.service.masterKey, []byte(in.Pass))
		if encErr != nil {
			log.Error().Err(encErr).Str("org_id", orgID).Msg("smtp password encryption failed")
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to encrypt SMTP password",
				"code":  "ADMIN_SMTP_ENCRYPT_ERROR",
			})
		}
	}

	if err := h.service.repo.SetOrgSMTPSettings(c.Request().Context(), orgID, in.Host, in.Port, in.User, in.From, in.TLS, passEnc); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("update org smtp settings failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to update SMTP settings",
			"code":  "ADMIN_SMTP_SETTINGS_UPDATE_ERROR",
		})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// ─── Backup Configuration (Migration 230) ────────────────────────────────────

// GetOrgBackupConfig handles GET /api/v1/admin/org/backup-config.
// Returns the per-org backup configuration. Encrypted secrets are never exposed;
// HasPassphrase and HasNotifyWebhook signal whether values are stored.
func (h *Handler) GetOrgBackupConfig(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	cfg, _, _, err := h.service.repo.GetOrgBackupConfig(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("get org backup config failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve backup configuration",
			"code":  "ADMIN_BACKUP_CONFIG_ERROR",
		})
	}
	return c.JSON(http.StatusOK, cfg)
}

// UpdateOrgBackupConfigInput is the request body for PUT /api/v1/admin/org/backup-config.
type UpdateOrgBackupConfigInput struct {
	Schedule      string `json:"schedule"`       // cron expression; "" = use env default
	RetentionDays int    `json:"retention_days"` // 0 = use env default
	Passphrase    string `json:"passphrase"`     // empty = keep existing
	NotifyWebhook string `json:"notify_webhook"` // empty = keep existing
	OffsiteCmd    string `json:"offsite_cmd"`
	NotifyCmd     string `json:"notify_cmd"`
}

// UpdateOrgBackupConfig handles PUT /api/v1/admin/org/backup-config.
// Non-empty Passphrase and NotifyWebhook are encrypted with the master key.
// Empty values leave existing encrypted data unchanged.
func (h *Handler) UpdateOrgBackupConfig(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	var in UpdateOrgBackupConfigInput
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid input",
			"code":  "ADMIN_BAD_REQUEST",
		})
	}

	encrypt := func(plaintext string) ([]byte, error) {
		if len(h.service.masterKey) == 0 {
			return nil, fmt.Errorf("master key not configured")
		}
		return sharedcrypto.Encrypt(h.service.masterKey, []byte(plaintext))
	}

	var passphraseEnc []byte
	if in.Passphrase != "" {
		var encErr error
		passphraseEnc, encErr = encrypt(in.Passphrase)
		if encErr != nil {
			log.Error().Err(encErr).Str("org_id", orgID).Msg("backup passphrase encryption failed")
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to encrypt backup passphrase",
				"code":  "ADMIN_BACKUP_ENCRYPT_ERROR",
			})
		}
	}

	var webhookEnc []byte
	if in.NotifyWebhook != "" {
		var encErr error
		webhookEnc, encErr = encrypt(in.NotifyWebhook)
		if encErr != nil {
			log.Error().Err(encErr).Str("org_id", orgID).Msg("backup notify webhook encryption failed")
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to encrypt backup notify webhook",
				"code":  "ADMIN_BACKUP_ENCRYPT_ERROR",
			})
		}
	}

	if err := h.service.repo.SetOrgBackupConfig(
		c.Request().Context(), orgID,
		in.Schedule, in.RetentionDays,
		passphraseEnc, webhookEnc,
		in.OffsiteCmd, in.NotifyCmd,
	); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("update org backup config failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to update backup configuration",
			"code":  "ADMIN_BACKUP_CONFIG_UPDATE_ERROR",
		})
	}
	// Audit log: shell commands execute in the backup container as operator.
	// Log hash+presence only — never the content (may contain credentials).
	if in.OffsiteCmd != "" || in.NotifyCmd != "" {
		h256 := func(s string) string {
			if s == "" {
				return ""
			}
			sum := sha256.Sum256([]byte(s))
			return hex.EncodeToString(sum[:8]) // first 8 bytes sufficient for change-detection
		}
		log.Warn().
			Str("org_id", orgID).
			Bool("offsite_cmd_set", in.OffsiteCmd != "").
			Bool("notify_cmd_set", in.NotifyCmd != "").
			Str("offsite_cmd_sha256_prefix", h256(in.OffsiteCmd)).
			Str("notify_cmd_sha256_prefix", h256(in.NotifyCmd)).
			Msg("backup shell commands updated by admin — review if unexpected")
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// InternalBackupConfigResponse is the plaintext response for the backup script endpoint.
type InternalBackupConfigResponse struct {
	Schedule      string `json:"schedule"`
	RetentionDays int    `json:"retention_days"`
	Passphrase    string `json:"passphrase"`     // decrypted plaintext
	NotifyWebhook string `json:"notify_webhook"` // decrypted plaintext
	OffsiteCmd    string `json:"offsite_cmd"`
	NotifyCmd     string `json:"notify_cmd"`
	// Guided backup destination (migration 232)
	BackupDestType       string `json:"backup_dest_type"`
	BackupDestURL        string `json:"backup_dest_url"`
	BackupDestUser       string `json:"backup_dest_user"`
	BackupDestPass       string `json:"backup_dest_pass"`
	BackupDestRemotePath string `json:"backup_dest_remote_path"`
	BackupDestEndpoint   string `json:"backup_dest_endpoint"`
	BackupDestBucket     string `json:"backup_dest_bucket"`
	BackupDestPrefix     string `json:"backup_dest_prefix"`
	BackupDestAccessKey  string `json:"backup_dest_access_key"`
	BackupDestSecretKey  string `json:"backup_dest_secret_key"`
	BackupDestHost       string `json:"backup_dest_host"`
	BackupDestPort       int    `json:"backup_dest_port"`
	BackupDestCmd        string `json:"backup_dest_cmd"`
}

// GetInternalBackupConfig handles GET /api/v1/internal/backup-config.
// Auth: "Authorization: Bearer <VAKT_SECRET_KEY>" (hex-encoded master key).
// Returns plaintext backup configuration so the backup script can use it directly.
// This endpoint has no JWT middleware — it uses the master key as the Bearer token.
func (h *Handler) GetInternalBackupConfig(c echo.Context) error {
	// Validate Bearer token against hex-encoded master key.
	authHeader := c.Request().Header.Get("Authorization")
	const prefix = "Bearer "
	if len(authHeader) <= len(prefix) || authHeader[:len(prefix)] != prefix {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "unauthorized",
			"code":  "UNAUTHORIZED",
		})
	}
	token := authHeader[len(prefix):]
	expected := hex.EncodeToString(h.service.masterKey)
	if len(h.service.masterKey) == 0 || subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "unauthorized",
			"code":  "UNAUTHORIZED",
		})
	}

	ctx := c.Request().Context()

	// Single-tenant: query the first org.
	var orgID string
	if err := h.service.db.QueryRow(ctx, `SELECT id::text FROM organizations LIMIT 1`).Scan(&orgID); err != nil {
		log.Error().Err(err).Msg("internal backup config: no org found")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "no organization found",
			"code":  "INTERNAL_BACKUP_NO_ORG",
		})
	}

	cfg, passphraseEnc, webhookEnc, err := h.service.repo.GetOrgBackupConfig(ctx, orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("internal backup config: get config failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve backup configuration",
			"code":  "INTERNAL_BACKUP_CONFIG_ERROR",
		})
	}

	resp := InternalBackupConfigResponse{
		Schedule:      cfg.Schedule,
		RetentionDays: cfg.RetentionDays,
		OffsiteCmd:    cfg.OffsiteCmd,
		NotifyCmd:     cfg.NotifyCmd,
	}

	if len(passphraseEnc) > 0 {
		plain, decErr := sharedcrypto.Decrypt(h.service.masterKey, passphraseEnc)
		if decErr != nil {
			log.Error().Err(decErr).Str("org_id", orgID).Msg("internal backup config: passphrase decrypt failed")
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to decrypt backup passphrase",
				"code":  "INTERNAL_BACKUP_DECRYPT_ERROR",
			})
		}
		resp.Passphrase = string(plain)
	}

	if len(webhookEnc) > 0 {
		plain, decErr := sharedcrypto.Decrypt(h.service.masterKey, webhookEnc)
		if decErr != nil {
			log.Error().Err(decErr).Str("org_id", orgID).Msg("internal backup config: notify webhook decrypt failed")
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to decrypt backup notify webhook",
				"code":  "INTERNAL_BACKUP_DECRYPT_ERROR",
			})
		}
		resp.NotifyWebhook = string(plain)
	}

	// Load backup dest config
	dest, destConfigEnc, err := h.service.repo.GetOrgBackupDest(ctx, orgID)
	if err == nil && len(destConfigEnc) > 0 {
		if plain, decErr := sharedcrypto.Decrypt(h.service.masterKey, destConfigEnc); decErr == nil {
			var destCfg BackupDestConfig
			if json.Unmarshal(plain, &destCfg) == nil {
				resp.BackupDestType = dest.Type
				resp.BackupDestURL = destCfg.URL
				resp.BackupDestUser = destCfg.User
				resp.BackupDestPass = destCfg.Pass
				resp.BackupDestRemotePath = destCfg.RemotePath
				resp.BackupDestEndpoint = destCfg.Endpoint
				resp.BackupDestBucket = destCfg.Bucket
				resp.BackupDestPrefix = destCfg.Prefix
				resp.BackupDestAccessKey = destCfg.AccessKey
				resp.BackupDestSecretKey = destCfg.SecretKey
				resp.BackupDestHost = destCfg.Host
				resp.BackupDestPort = destCfg.Port
				resp.BackupDestCmd = destCfg.Cmd
			}
		}
	} else if dest != nil {
		resp.BackupDestType = dest.Type
	}

	return c.JSON(http.StatusOK, resp)
}

// ─── Migration 231: LDAP/AD Configuration ────────────────────────────────────

// GetOrgLDAPConfig handles GET /api/v1/admin/org/ldap.
// Returns the per-org LDAP configuration. The bind password is never exposed
// in plaintext; HasBindPass signals whether an encrypted password is stored.
func (h *Handler) GetOrgLDAPConfig(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	cfg, _, err := h.service.repo.GetOrgLDAPConfig(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("get org ldap config failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve LDAP configuration",
			"code":  "ADMIN_LDAP_CONFIG_ERROR",
		})
	}
	return c.JSON(http.StatusOK, cfg)
}

// UpdateOrgLDAPConfigInput is the request body for PUT /api/v1/admin/org/ldap.
type UpdateOrgLDAPConfigInput struct {
	URL         string `json:"url"`
	BindDN      string `json:"bind_dn"`
	BindPass    string `json:"bind_pass"` // empty = keep existing password
	BaseDN      string `json:"base_dn"`
	UserFilter  string `json:"user_filter"`
	GroupFilter string `json:"group_filter"`
	TLS         bool   `json:"tls"`
}

// UpdateOrgLDAPConfig handles PUT /api/v1/admin/org/ldap.
// If BindPass is non-empty it is encrypted with the master key and stored.
// If BindPass is empty the existing encrypted password is kept unchanged.
func (h *Handler) UpdateOrgLDAPConfig(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	var in UpdateOrgLDAPConfigInput
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid input",
			"code":  "ADMIN_BAD_REQUEST",
		})
	}

	var bindPassEnc []byte
	if in.BindPass != "" {
		if len(h.service.masterKey) == 0 {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "master key not configured",
				"code":  "ADMIN_NO_MASTER_KEY",
			})
		}
		var encErr error
		bindPassEnc, encErr = sharedcrypto.Encrypt(h.service.masterKey, []byte(in.BindPass))
		if encErr != nil {
			log.Error().Err(encErr).Str("org_id", orgID).Msg("ldap bind password encryption failed")
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to encrypt LDAP bind password",
				"code":  "ADMIN_LDAP_ENCRYPT_ERROR",
			})
		}
	}

	if err := h.service.repo.SetOrgLDAPConfig(
		c.Request().Context(), orgID,
		in.URL, in.BindDN, in.BaseDN, in.UserFilter, in.GroupFilter,
		in.TLS, bindPassEnc,
	); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("update org ldap config failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to update LDAP configuration",
			"code":  "ADMIN_LDAP_CONFIG_UPDATE_ERROR",
		})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// buildLDAPSyncer retrieves the org's LDAP config from the DB, decrypts the
// bind password, and returns a ready-to-use Syncer. Returns a descriptive
// HTTP error response via c.JSON on failure (callers must return immediately).
func (h *Handler) buildLDAPSyncer(c echo.Context, orgID string) (*platformldap.Syncer, error) {
	cfg, bindPassEnc, err := h.service.repo.GetOrgLDAPConfig(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("ldap test: get config failed")
		_ = c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve LDAP configuration",
			"code":  "ADMIN_LDAP_CONFIG_ERROR",
		})
		return nil, err
	}

	var bindPass string
	if len(bindPassEnc) > 0 {
		if len(h.service.masterKey) == 0 {
			_ = c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "master key not configured",
				"code":  "ADMIN_NO_MASTER_KEY",
			})
			return nil, fmt.Errorf("master key not configured")
		}
		plain, decErr := sharedcrypto.Decrypt(h.service.masterKey, bindPassEnc)
		if decErr != nil {
			log.Error().Err(decErr).Str("org_id", orgID).Msg("ldap test: bind password decrypt failed")
			_ = c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to decrypt LDAP bind password",
				"code":  "ADMIN_LDAP_DECRYPT_ERROR",
			})
			return nil, decErr
		}
		bindPass = string(plain)
	}

	ldapCfg := platformldap.Config{
		URL:         cfg.URL,
		BindDN:      cfg.BindDN,
		BindPass:    bindPass,
		BaseDN:      cfg.BaseDN,
		UserFilter:  cfg.UserFilter,
		GroupFilter: cfg.GroupFilter,
		TLS:         cfg.TLS,
	}
	return platformldap.NewSyncer(ldapCfg), nil
}

// TestOrgLDAPConnection handles POST /api/v1/admin/org/ldap/test.
// Connects to the configured LDAP server, lists users, and returns the count.
func (h *Handler) TestOrgLDAPConnection(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)

	syncer, err := h.buildLDAPSyncer(c, orgID)
	if err != nil {
		return nil // response already written by buildLDAPSyncer
	}

	users, err := syncer.ListUsers(c.Request().Context())
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("ldap test: list users failed")
		return c.JSON(http.StatusBadGateway, map[string]any{
			"ok":    false,
			"error": err.Error(),
			"code":  "ADMIN_LDAP_TEST_FAILED",
		})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"ok":          true,
		"users_found": len(users),
	})
}

// SyncOrgLDAP handles POST /api/v1/admin/org/ldap/sync.
// Connects to the configured LDAP server, retrieves all users, and returns them.
func (h *Handler) SyncOrgLDAP(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)

	syncer, err := h.buildLDAPSyncer(c, orgID)
	if err != nil {
		return nil // response already written by buildLDAPSyncer
	}

	users, err := syncer.ListUsers(c.Request().Context())
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("ldap sync: list users failed")
		return c.JSON(http.StatusBadGateway, map[string]any{
			"error": err.Error(),
			"code":  "ADMIN_LDAP_SYNC_FAILED",
		})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"synced": len(users),
		"users":  users,
	})
}

// ─── Migration 232: Guided Backup Destination ─────────────────────────────────

// GetOrgBackupDest handles GET /api/v1/admin/org/backup-dest.
func (h *Handler) GetOrgBackupDest(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	dest, configEnc, err := h.service.repo.GetOrgBackupDest(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("get org backup dest failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve backup destination",
			"code":  "ADMIN_BACKUP_DEST_ERROR",
		})
	}
	// Decrypt config to populate non-secret fields + has_* booleans.
	if len(configEnc) > 0 && len(h.service.masterKey) > 0 {
		plain, err := sharedcrypto.Decrypt(h.service.masterKey, configEnc)
		if err == nil {
			var cfg BackupDestConfig
			if json.Unmarshal(plain, &cfg) == nil {
				dest.URL = cfg.URL
				dest.User = cfg.User
				dest.RemotePath = cfg.RemotePath
				dest.HasPass = cfg.Pass != ""
				dest.Endpoint = cfg.Endpoint
				dest.Bucket = cfg.Bucket
				dest.Prefix = cfg.Prefix
				dest.AccessKey = cfg.AccessKey
				dest.HasSecretKey = cfg.SecretKey != ""
				dest.Host = cfg.Host
				dest.Port = cfg.Port
				dest.Cmd = cfg.Cmd
			}
		}
	}
	return c.JSON(http.StatusOK, dest)
}

// UpdateOrgBackupDestInput is the request body for PUT /api/v1/admin/org/backup-dest.
// Empty secret fields (Pass, SecretKey) = keep existing encrypted value.
type UpdateOrgBackupDestInput struct {
	Type       string `json:"type"`
	URL        string `json:"url"`
	User       string `json:"user"`
	Pass       string `json:"pass"` // empty = keep existing
	RemotePath string `json:"remote_path"`
	Endpoint   string `json:"endpoint"`
	Bucket     string `json:"bucket"`
	Prefix     string `json:"prefix"`
	AccessKey  string `json:"access_key"`
	SecretKey  string `json:"secret_key"` // empty = keep existing
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Cmd        string `json:"cmd"`
}

// UpdateOrgBackupDest handles PUT /api/v1/admin/org/backup-dest.
func (h *Handler) UpdateOrgBackupDest(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	var in UpdateOrgBackupDestInput
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid input",
			"code":  "ADMIN_BAD_REQUEST",
		})
	}

	// Load existing config to preserve secrets when empty fields are submitted.
	var existing BackupDestConfig
	_, existingEnc, err := h.service.repo.GetOrgBackupDest(c.Request().Context(), orgID)
	if err == nil && len(existingEnc) > 0 && len(h.service.masterKey) > 0 {
		if plain, err := sharedcrypto.Decrypt(h.service.masterKey, existingEnc); err == nil {
			_ = json.Unmarshal(plain, &existing)
		}
	}

	cfg := BackupDestConfig{
		URL:        in.URL,
		User:       in.User,
		Pass:       existing.Pass, // keep existing by default
		RemotePath: in.RemotePath,
		Endpoint:   in.Endpoint,
		Bucket:     in.Bucket,
		Prefix:     in.Prefix,
		AccessKey:  in.AccessKey,
		SecretKey:  existing.SecretKey, // keep existing by default
		Host:       in.Host,
		Port:       in.Port,
		Cmd:        in.Cmd,
	}
	if in.Pass != "" {
		cfg.Pass = in.Pass
	}
	if in.SecretKey != "" {
		cfg.SecretKey = in.SecretKey
	}

	plain, err := json.Marshal(cfg)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to marshal config",
			"code":  "ADMIN_BACKUP_DEST_MARSHAL_ERROR",
		})
	}

	var configEnc []byte
	if in.Type != "none" && in.Type != "" {
		if len(h.service.masterKey) == 0 {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "master key not configured",
				"code":  "ADMIN_NO_MASTER_KEY",
			})
		}
		configEnc, err = sharedcrypto.Encrypt(h.service.masterKey, plain)
		if err != nil {
			log.Error().Err(err).Str("org_id", orgID).Msg("backup dest encryption failed")
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to encrypt backup destination config",
				"code":  "ADMIN_BACKUP_DEST_ENCRYPT_ERROR",
			})
		}
	}

	if err := h.service.repo.SetOrgBackupDest(c.Request().Context(), orgID, in.Type, configEnc); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("update org backup dest failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to update backup destination",
			"code":  "ADMIN_BACKUP_DEST_UPDATE_ERROR",
		})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
