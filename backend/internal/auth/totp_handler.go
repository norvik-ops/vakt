package auth

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	sharedcrypto "github.com/matharnica/vakt/internal/shared/crypto"
)

// TotpHandler holds handler state for the 2FA/TOTP endpoints.
type TotpHandler struct {
	db        *pgxpool.Pool
	masterKey []byte
	svc       *Service // used by recovery-code login to issue token pairs
}

// NewTotpHandler constructs a TotpHandler.
func NewTotpHandler(db *pgxpool.Pool, masterKey []byte, svc *Service) *TotpHandler {
	return &TotpHandler{db: db, masterKey: masterKey, svc: svc}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func (h *TotpHandler) encryptSecret(plaintext string) (string, error) {
	ct, err := sharedcrypto.Encrypt(h.masterKey, []byte(plaintext))
	if err != nil {
		return "", fmt.Errorf("encrypt totp secret: %w", err)
	}
	return hex.EncodeToString(ct), nil
}

func (h *TotpHandler) decryptSecret(cipherhex string) (string, error) {
	ct, err := hex.DecodeString(cipherhex)
	if err != nil {
		return "", fmt.Errorf("decode cipherhex: %w", err)
	}
	plain, err := sharedcrypto.Decrypt(h.masterKey, ct)
	if err != nil {
		return "", fmt.Errorf("decrypt totp secret: %w", err)
	}
	return string(plain), nil
}

func requireUserID(c echo.Context) (string, bool) {
	userID, ok := c.Get("user_id").(string)
	return userID, ok && userID != ""
}

// ─── Status ───────────────────────────────────────────────────────────────────

// Status handles GET /auth/2fa/status.
// Returns {"enabled": true/false} for the current user.
func (h *TotpHandler) Status(c echo.Context) error {
	userID, ok := requireUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	var enabled bool
	err := h.db.QueryRow(
		c.Request().Context(),
		`SELECT enabled FROM totp_secrets WHERE user_id = $1::uuid`,
		userID,
	).Scan(&enabled)
	if err != nil {
		// No row means 2FA not set up → enabled = false.
		return c.JSON(http.StatusOK, map[string]bool{"enabled": false})
	}
	return c.JSON(http.StatusOK, map[string]bool{"enabled": enabled})
}

// ─── Setup ────────────────────────────────────────────────────────────────────

// Setup handles POST /auth/2fa/setup.
// Generates a TOTP secret, stores it (encrypted, not yet confirmed), returns secret + URI.
func (h *TotpHandler) Setup(c echo.Context) error {
	userID, ok := requireUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	ctx := c.Request().Context()

	// Check if 2FA is already enabled.
	var enabled bool
	_ = h.db.QueryRow(ctx,
		`SELECT enabled FROM totp_secrets WHERE user_id = $1::uuid`, userID,
	).Scan(&enabled)
	if enabled {
		return c.JSON(http.StatusConflict, map[string]string{
			"error": "2FA already enabled",
			"code":  "TOTP_ALREADY_ENABLED",
		})
	}

	// Fetch the user's email for the TOTP label.
	var email string
	if err := h.db.QueryRow(ctx,
		`SELECT email FROM users WHERE id = $1::uuid`, userID,
	).Scan(&email); err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("totp setup: user lookup failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "user not found",
			"code":  "TOTP_USER_NOT_FOUND",
		})
	}

	secret, uri, err := GenerateTOTPSecret(email, totpIssuer)
	if err != nil {
		log.Error().Err(err).Msg("totp setup: generate secret failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to generate 2FA secret",
			"code":  "TOTP_GENERATE_FAILED",
		})
	}

	encryptedSecret, err := h.encryptSecret(secret)
	if err != nil {
		log.Error().Err(err).Msg("totp setup: encrypt secret failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to encrypt 2FA secret",
			"code":  "TOTP_ENCRYPT_FAILED",
		})
	}

	// Upsert with enabled=false (pending confirmation).
	_, err = h.db.Exec(ctx, `
		INSERT INTO totp_secrets (user_id, secret, enabled)
		VALUES ($1::uuid, $2, false)
		ON CONFLICT (user_id)
		DO UPDATE SET secret = EXCLUDED.secret, enabled = false, updated_at = now()
	`, userID, encryptedSecret)
	if err != nil {
		log.Error().Err(err).Msg("totp setup: db upsert failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to store 2FA secret",
			"code":  "TOTP_STORE_FAILED",
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"secret": secret,
		"uri":    uri,
	})
}

// ─── Confirm ─────────────────────────────────────────────────────────────────

// Confirm handles POST /auth/2fa/confirm.
// Validates the first TOTP code, activates 2FA, and returns backup codes.
func (h *TotpHandler) Confirm(c echo.Context) error {
	userID, ok := requireUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	var body struct {
		Code string `json:"code"`
	}
	if err := c.Bind(&body); err != nil || body.Code == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "code is required",
			"code":  "TOTP_BAD_REQUEST",
		})
	}

	ctx := c.Request().Context()

	var encryptedSecret string
	var alreadyEnabled bool
	err := h.db.QueryRow(ctx,
		`SELECT secret, enabled FROM totp_secrets WHERE user_id = $1::uuid`, userID,
	).Scan(&encryptedSecret, &alreadyEnabled)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "2FA setup not started — call /auth/2fa/setup first",
			"code":  "TOTP_NOT_SETUP",
		})
	}
	if alreadyEnabled {
		return c.JSON(http.StatusConflict, map[string]string{
			"error": "2FA already enabled",
			"code":  "TOTP_ALREADY_ENABLED",
		})
	}

	secret, err := h.decryptSecret(encryptedSecret)
	if err != nil {
		log.Error().Err(err).Msg("totp confirm: decrypt failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to decrypt 2FA secret",
			"code":  "TOTP_DECRYPT_FAILED",
		})
	}

	if !ValidateTOTP(secret, body.Code) {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": "invalid TOTP code",
			"code":  "TOTP_INVALID_CODE",
		})
	}
	if err := h.checkAndMarkTOTPCode(ctx, userID, body.Code); err != nil {
		// An outage is not a user error: telling someone with a valid
		// authenticator "already used" sends them chasing clock drift while the
		// logs stay silent (ADR-0044).
		if errors.Is(err, ErrTOTPReplayCheckUnavailable) {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"error": "Zwei-Faktor-Prüfung vorübergehend nicht verfügbar. Bitte in Kürze erneut versuchen.",
				"code":  "TOTP_REPLAY_CHECK_UNAVAILABLE",
			})
		}
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": "TOTP code already used",
			"code":  "TOTP_CODE_REPLAYED",
		})
	}

	plainCodes, hashedCodes, err := GenerateBackupCodes()
	if err != nil {
		log.Error().Err(err).Msg("totp confirm: backup code generation failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to generate backup codes",
			"code":  "TOTP_BACKUP_FAILED",
		})
	}

	_, err = h.db.Exec(ctx, `
		UPDATE totp_secrets
		SET enabled = true, backup_codes = $2, updated_at = now()
		WHERE user_id = $1::uuid
	`, userID, hashedCodes)
	if err != nil {
		log.Error().Err(err).Msg("totp confirm: db update failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to activate 2FA",
			"code":  "TOTP_ACTIVATE_FAILED",
		})
	}

	// Generate recovery codes and persist them in auth_recovery_codes.
	plainRecovery, hashedRecovery, err := generateRecoveryCodes()
	if err != nil {
		log.Error().Err(err).Msg("totp confirm: recovery code generation failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to generate recovery codes",
			"code":  "TOTP_BACKUP_FAILED",
		})
	}
	if err := h.StoreRecoveryCodes(ctx, userID, hashedRecovery); err != nil {
		log.Error().Err(err).Msg("totp confirm: store recovery codes failed")
		// Non-fatal: 2FA is already activated; log and continue without codes.
		return c.JSON(http.StatusOK, map[string]any{
			"backup_codes":   plainCodes,
			"recovery_codes": []string{},
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"backup_codes":   plainCodes,
		"recovery_codes": plainRecovery,
	})
}

// ─── Disable ─────────────────────────────────────────────────────────────────

// Disable handles POST /auth/2fa/disable.
// Requires a valid TOTP code, then deletes the totp_secrets row.
func (h *TotpHandler) Disable(c echo.Context) error {
	userID, ok := requireUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	var body struct {
		Code string `json:"code"`
	}
	if err := c.Bind(&body); err != nil || body.Code == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "code is required",
			"code":  "TOTP_BAD_REQUEST",
		})
	}

	ctx := c.Request().Context()

	var encryptedSecret string
	var enabled bool
	err := h.db.QueryRow(ctx,
		`SELECT secret, enabled FROM totp_secrets WHERE user_id = $1::uuid`, userID,
	).Scan(&encryptedSecret, &enabled)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "2FA is not set up",
			"code":  "TOTP_NOT_SETUP",
		})
	}
	if !enabled {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "2FA is not enabled",
			"code":  "TOTP_NOT_ENABLED",
		})
	}

	secret, err := h.decryptSecret(encryptedSecret)
	if err != nil {
		log.Error().Err(err).Msg("totp disable: decrypt failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to decrypt 2FA secret",
			"code":  "TOTP_DECRYPT_FAILED",
		})
	}

	if !ValidateTOTP(secret, body.Code) {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": "invalid TOTP code",
			"code":  "TOTP_INVALID_CODE",
		})
	}
	if err := h.checkAndMarkTOTPCode(ctx, userID, body.Code); err != nil {
		// An outage is not a user error: telling someone with a valid
		// authenticator "already used" sends them chasing clock drift while the
		// logs stay silent (ADR-0044).
		if errors.Is(err, ErrTOTPReplayCheckUnavailable) {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"error": "Zwei-Faktor-Prüfung vorübergehend nicht verfügbar. Bitte in Kürze erneut versuchen.",
				"code":  "TOTP_REPLAY_CHECK_UNAVAILABLE",
			})
		}
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": "TOTP code already used",
			"code":  "TOTP_CODE_REPLAYED",
		})
	}

	_, err = h.db.Exec(ctx,
		`DELETE FROM totp_secrets WHERE user_id = $1::uuid`, userID,
	)
	if err != nil {
		log.Error().Err(err).Msg("totp disable: db delete failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to disable 2FA",
			"code":  "TOTP_DISABLE_FAILED",
		})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "2FA disabled"})
}

// ─── Verify ───────────────────────────────────────────────────────────────────

// LoginVerify handles POST /auth/2fa/login-verify — the SECOND stage of the
// two-stage login (S124-1/SA14-01). It is a PUBLIC route: the caller has no
// session yet, only the short-lived mfa_pending token returned by /auth/login.
// It validates that token plus a TOTP or backup code, then issues a full,
// mfa=true token pair and sets the session cookies — exactly like /auth/login.
func (h *TotpHandler) LoginVerify(c echo.Context) error {
	var body struct {
		MFAToken   string `json:"mfa_token"`
		Code       string `json:"code"`
		BackupCode string `json:"backup_code"`
	}
	if err := c.Bind(&body); err != nil || body.MFAToken == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "mfa_token is required", "code": "AUTH_BAD_REQUEST",
		})
	}
	if body.Code == "" && body.BackupCode == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "code or backup_code is required", "code": "TOTP_BAD_REQUEST",
		})
	}
	if h.svc == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "token issuance not configured", "code": "AUTH_INTERNAL_ERROR",
		})
	}

	userID, orgID, err := h.svc.ParseMFAPending(body.MFAToken)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "invalid or expired MFA session — please log in again",
			"code":  "AUTH_MFA_PENDING_INVALID",
		})
	}

	ctx := c.Request().Context()
	if err := h.validateSecondFactor(ctx, userID, body.Code, body.BackupCode); err != nil {
		// This is the path every MFA login goes through. Reporting a Redis outage
		// as "invalid code" here is the worst of the four sites: the user blames
		// their authenticator's clock and retries forever, while the operator sees
		// only an invalid-code spike and nothing in the logs (ADR-0044).
		if errors.Is(err, ErrTOTPReplayCheckUnavailable) {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"error": "Zwei-Faktor-Prüfung vorübergehend nicht verfügbar. Bitte in Kürze erneut versuchen.",
				"code":  "TOTP_REPLAY_CHECK_UNAVAILABLE",
			})
		}
		if errors.Is(err, ErrBackupCodeNotConsumed) {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"error": "Der Backup-Code konnte nicht entwertet werden. Bitte in Kürze erneut versuchen.",
				"code":  "TOTP_BACKUP_CODE_NOT_CONSUMED",
			})
		}
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": "invalid code", "code": "TOTP_INVALID_CODE",
		})
	}

	deviceHint := c.Request().Header.Get("User-Agent")
	if len(deviceHint) > 120 {
		deviceHint = deviceHint[:120]
	}
	resp, err := h.svc.CompleteMFALogin(ctx, userID, orgID, deviceHint)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("mfa login-verify: token issuance failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to issue tokens", "code": "AUTH_INTERNAL_ERROR",
		})
	}

	// Set session cookies exactly like the primary login handler.
	secure := CookieSecure(c)
	c.SetCookie(&http.Cookie{ // nosemgrep: cookie-missing-secure -- Secure set via variable
		Name: "access_token", Value: resp.AccessToken, HttpOnly: true, Secure: secure,
		SameSite: http.SameSiteStrictMode, Path: "/api/v1", MaxAge: 3600,
	})
	csrfToken := GenerateCSRFToken()
	SetCSRFCookie(c, csrfToken)
	resp.CSRFToken = csrfToken
	return c.JSON(http.StatusOK, resp)
}

// validateSecondFactor checks a TOTP code (with replay protection) or a backup
// code for the user, consuming the backup code on success. Shared by LoginVerify.
func (h *TotpHandler) validateSecondFactor(ctx context.Context, userID, code, backupCode string) error {
	var encryptedSecret string
	var enabled bool
	var backupCodes []string
	err := h.db.QueryRow(ctx, `
		SELECT secret, enabled, backup_codes FROM totp_secrets WHERE user_id = $1::uuid
	`, userID).Scan(&encryptedSecret, &enabled, &backupCodes)
	if err != nil || !enabled {
		return fmt.Errorf("2FA not enabled")
	}

	if backupCode != "" {
		idx := CheckBackupCode(backupCode, backupCodes)
		if idx < 0 {
			return fmt.Errorf("invalid backup code")
		}
		newCodes := removeIndex(backupCodes, idx)
		if uerr := h.updateBackupCodes(ctx, userID, newCodes); uerr != nil {
			// R1-W7A-N3 (class sweep): this used to log and `return nil`, i.e. the
			// login passed while the code stayed in totp_secrets.backup_codes —
			// a SINGLE-USE second factor silently turned into a permanent one.
			// The TOTP path immediately below already had this right: it preserves
			// the error class so the caller can answer 503 for an outage instead
			// of pretending. Same rule here — a factor that could not be consumed
			// has not been spent, so it has not been accepted.
			log.Error().Err(uerr).Msg("login-verify: failed to consume backup code")
			return fmt.Errorf("%w: %v", ErrBackupCodeNotConsumed, uerr)
		}
		return nil
	}

	secret, err := h.decryptSecret(encryptedSecret)
	if err != nil {
		return fmt.Errorf("decrypt secret: %w", err)
	}
	if !ValidateTOTP(secret, code) {
		return fmt.Errorf("invalid TOTP code")
	}
	if err := h.checkAndMarkTOTPCode(ctx, userID, code); err != nil {
		// Preserve the class so LoginVerify can tell an outage (503) from a real
		// replay (422) — flattening it here is what made both look identical.
		return err
	}
	return nil
}

// Verify handles POST /auth/2fa/verify.
// Accepts {"code": "123456"} or {"backup_code": "XXXX-XXXX"}.
// Used as a second factor after primary login succeeds.
func (h *TotpHandler) Verify(c echo.Context) error {
	userID, ok := requireUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	var body struct {
		Code       string `json:"code"`
		BackupCode string `json:"backup_code"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "TOTP_BAD_REQUEST",
		})
	}
	if body.Code == "" && body.BackupCode == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "code or backup_code is required",
			"code":  "TOTP_BAD_REQUEST",
		})
	}

	ctx := c.Request().Context()

	var encryptedSecret string
	var enabled bool
	var backupCodes []string
	err := h.db.QueryRow(ctx, `
		SELECT secret, enabled, backup_codes
		FROM totp_secrets
		WHERE user_id = $1::uuid
	`, userID).Scan(&encryptedSecret, &enabled, &backupCodes)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "2FA is not configured for this user",
			"code":  "TOTP_NOT_SETUP",
		})
	}
	if !enabled {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "2FA is not enabled",
			"code":  "TOTP_NOT_ENABLED",
		})
	}

	// Backup code path.
	if body.BackupCode != "" {
		idx := CheckBackupCode(body.BackupCode, backupCodes)
		if idx < 0 {
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{
				"error": "invalid backup code",
				"code":  "TOTP_INVALID_CODE",
			})
		}
		// Remove the used backup code (replace with empty string sentinel or shrink slice).
		newCodes := removeIndex(backupCodes, idx)
		if err := h.updateBackupCodes(ctx, userID, newCodes); err != nil {
			// Twin of the site in validateSecondFactor — see the note there. A
			// backup code that could not be struck off the list is still on it,
			// so answering "verified" hands out an unlimited second factor.
			log.Error().Err(err).Msg("totp verify: failed to consume backup code")
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"error": "Der Backup-Code konnte nicht entwertet werden. Bitte in Kürze erneut versuchen.",
				"code":  "TOTP_BACKUP_CODE_NOT_CONSUMED",
			})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "verified"})
	}

	// TOTP code path.
	secret, err := h.decryptSecret(encryptedSecret)
	if err != nil {
		log.Error().Err(err).Msg("totp verify: decrypt failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to decrypt 2FA secret",
			"code":  "TOTP_DECRYPT_FAILED",
		})
	}

	if !ValidateTOTP(secret, body.Code) {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": "invalid TOTP code",
			"code":  "TOTP_INVALID_CODE",
		})
	}
	if err := h.checkAndMarkTOTPCode(ctx, userID, body.Code); err != nil {
		// An outage is not a user error: telling someone with a valid
		// authenticator "already used" sends them chasing clock drift while the
		// logs stay silent (ADR-0044).
		if errors.Is(err, ErrTOTPReplayCheckUnavailable) {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"error": "Zwei-Faktor-Prüfung vorübergehend nicht verfügbar. Bitte in Kürze erneut versuchen.",
				"code":  "TOTP_REPLAY_CHECK_UNAVAILABLE",
			})
		}
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": "TOTP code already used",
			"code":  "TOTP_CODE_REPLAYED",
		})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "verified"})
}

// ErrTOTPReplayCheckUnavailable signals that replay protection could not be
// consulted because Redis is unreachable — an INFRASTRUCTURE outage, not a bad
// code from the user.
//
// It exists because the two conditions must not share a channel. Folding them
// together tells a user with a perfectly good authenticator "invalid code",
// which they can only read as clock drift; they retry forever, and the operator
// sees a replay/invalid-code spike — a plausible but wrong signal — with nothing
// in the logs. Callers map this to 503 + a retry hint, mirroring the lockout
// path (ADR-0044, ErrLockoutCheckUnavailable).
var ErrTOTPReplayCheckUnavailable = errors.New("auth: totp replay check unavailable (redis outage)")

// ErrBackupCodeNotConsumed signals that a backup code matched but could not be
// struck off the user's list. The code is therefore still spendable, so the
// second factor must NOT count as passed — the caller answers 503, not 422:
// nothing about the code was wrong, the store was.
var ErrBackupCodeNotConsumed = errors.New("auth: backup code could not be consumed")

// ErrTOTPCodeReplayed signals that this code was already spent inside its window.
var ErrTOTPCodeReplayed = errors.New("auth: TOTP code already used")

// checkAndMarkTOTPCode uses Redis SetNX to ensure a TOTP code cannot be replayed
// within its valid window (90 seconds).
//
// Fails CLOSED on a Redis outage — without replay-protection storage the
// single-use guarantee cannot be enforced, so the code is refused — UNLESS the
// operator has explicitly opted into fail-open via
// VAKT_AUTH_FAIL_OPEN_ON_REDIS_OUTAGE. That switch is documented as governing
// auth behaviour during a Redis outage; ignoring it here would mean the
// documented opt-out silently does not apply to the one path users hit on every
// MFA login.
func (h *TotpHandler) checkAndMarkTOTPCode(ctx context.Context, userID, code string) error {
	if h.svc == nil || h.svc.redis == nil {
		if h.svc != nil && h.svc.failOpenOnRedisOutage {
			log.Warn().Str("user_id", userID).Bool("fail_open", true).
				Msg("totp replay protection unavailable (no redis client) — allowing per VAKT_AUTH_FAIL_OPEN_ON_REDIS_OUTAGE")
			return nil
		}
		return ErrTOTPReplayCheckUnavailable
	}
	key := "totp_used:" + userID + ":" + code
	set, err := h.svc.redis.SetNX(ctx, key, "1", 90*time.Second).Result()
	if err != nil {
		if h.svc.failOpenOnRedisOutage {
			log.Warn().Err(err).Str("user_id", userID).Bool("fail_open", true).
				Msg("totp replay protection check failed — allowing per VAKT_AUTH_FAIL_OPEN_ON_REDIS_OUTAGE")
			return nil
		}
		log.Error().Err(err).Str("user_id", userID).Bool("fail_open", false).
			Msg("totp replay protection check failed — refusing code (fail-closed)")
		return fmt.Errorf("%w: %v", ErrTOTPReplayCheckUnavailable, err)
	}
	if !set {
		return ErrTOTPCodeReplayed
	}
	return nil
}

func (h *TotpHandler) updateBackupCodes(ctx context.Context, userID string, codes []string) error {
	_, err := h.db.Exec(ctx,
		`UPDATE totp_secrets SET backup_codes = $2, updated_at = now() WHERE user_id = $1::uuid`,
		userID, codes,
	)
	return err
}

func removeIndex(s []string, i int) []string {
	out := make([]string, 0, len(s)-1)
	out = append(out, s[:i]...)
	out = append(out, s[i+1:]...)
	return out
}

// ─── Recovery Code Login ──────────────────────────────────────────────────────

// RecoveryLogin handles POST /auth/2fa/recovery.
// Accepts {"code": "XXXX-XXXX-XXXX"}, verifies a recovery code, marks it used,
// and issues a new token pair — the same shape as a regular login response.
// Requires an authenticated user (e.g. a partial-auth token or existing session).
func (h *TotpHandler) RecoveryLogin(c echo.Context) error {
	userID, ok := requireUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	var body struct {
		Code string `json:"code"`
	}
	if err := c.Bind(&body); err != nil || body.Code == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "code is required",
			"code":  "AUTH_BAD_REQUEST",
		})
	}

	ctx := c.Request().Context()

	if err := h.VerifyRecoveryCode(ctx, userID, body.Code); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": "invalid or already-used recovery code",
			"code":  "AUTH_INVALID_RECOVERY_CODE",
		})
	}

	if h.svc == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "token issuance not configured",
			"code":  "AUTH_INTERNAL_ERROR",
		})
	}

	// Fetch the user's primary org membership to issue a proper token pair.
	var orgID, roleName string
	err := h.db.QueryRow(ctx, `
		SELECT om.org_id::text, r.name
		FROM org_members om
		JOIN roles r ON r.id = om.role_id
		WHERE om.user_id = $1::uuid
		ORDER BY om.joined_at ASC
		LIMIT 1`,
		userID,
	).Scan(&orgID, &roleName)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("recovery login: org lookup failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to fetch user membership",
			"code":  "AUTH_INTERNAL_ERROR",
		})
	}

	resp, err := h.svc.issueTokenPair(ctx, userID, orgID, []string{roleName}, "", true /* recovery code = 2nd factor */)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("recovery login: token issuance failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to issue tokens",
			"code":  "AUTH_INTERNAL_ERROR",
		})
	}

	return c.JSON(http.StatusOK, resp)
}

// ─── Regenerate Recovery Codes ────────────────────────────────────────────────

// RegenerateRecoveryCodes handles POST /auth/2fa/recovery-codes/regenerate.
// Requires an authenticated user with 2FA already enabled AND the current
// TOTP code (S132-S11/D24-2) — otherwise a hijacked session (or a CSRF-less
// request, before this sprint's fix) could invalidate every existing recovery
// code without ever proving possession of the authenticator.
func (h *TotpHandler) RegenerateRecoveryCodes(c echo.Context) error {
	userID, ok := requireUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	var body struct {
		Code string `json:"code"`
	}
	if err := c.Bind(&body); err != nil || body.Code == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "code is required",
			"code":  "TOTP_BAD_REQUEST",
		})
	}

	ctx := c.Request().Context()

	// Verify that 2FA is enabled for this user.
	var encryptedSecret string
	var enabled bool
	err := h.db.QueryRow(ctx,
		`SELECT secret, enabled FROM totp_secrets WHERE user_id = $1::uuid`, userID,
	).Scan(&encryptedSecret, &enabled)
	if err != nil || !enabled {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "2FA is not enabled",
			"code":  "TOTP_NOT_ENABLED",
		})
	}

	secret, err := h.decryptSecret(encryptedSecret)
	if err != nil {
		log.Error().Err(err).Msg("regenerate recovery codes: decrypt failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to decrypt 2FA secret",
			"code":  "TOTP_DECRYPT_FAILED",
		})
	}

	if !ValidateTOTP(secret, body.Code) {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": "invalid TOTP code",
			"code":  "TOTP_INVALID_CODE",
		})
	}
	if err := h.checkAndMarkTOTPCode(ctx, userID, body.Code); err != nil {
		// Same split as the other call sites (ADR-0044): a Redis outage is not a
		// user error. This site arrived with S11 while S12 fixed the other four,
		// so the two branches were each green and their merge was not — git had
		// no reason to flag it, the edits are in different parts of the file.
		if errors.Is(err, ErrTOTPReplayCheckUnavailable) {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"error": "Zwei-Faktor-Prüfung vorübergehend nicht verfügbar. Bitte in Kürze erneut versuchen.",
				"code":  "TOTP_REPLAY_CHECK_UNAVAILABLE",
			})
		}
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": "TOTP code already used",
			"code":  "TOTP_CODE_REPLAYED",
		})
	}

	plainCodes, hashedCodes, err := generateRecoveryCodes()
	if err != nil {
		log.Error().Err(err).Msg("regenerate recovery codes: generation failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to generate recovery codes",
			"code":  "TOTP_BACKUP_FAILED",
		})
	}

	if err := h.StoreRecoveryCodes(ctx, userID, hashedCodes); err != nil {
		log.Error().Err(err).Msg("regenerate recovery codes: store failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to store recovery codes",
			"code":  "TOTP_BACKUP_FAILED",
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"recovery_codes": plainCodes,
	})
}
