package auth

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/sechealth-app/sechealth/internal/modules/secvault"
)

// TotpHandler holds handler state for the 2FA/TOTP endpoints.
type TotpHandler struct {
	db        *pgxpool.Pool
	masterKey []byte
}

// NewTotpHandler constructs a TotpHandler.
func NewTotpHandler(db *pgxpool.Pool, masterKey []byte) *TotpHandler {
	return &TotpHandler{db: db, masterKey: masterKey}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func (h *TotpHandler) encryptSecret(plaintext string) (string, error) {
	ct, err := secvault.Encrypt(h.masterKey, []byte(plaintext))
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
	plain, err := secvault.Decrypt(h.masterKey, ct)
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

	return c.JSON(http.StatusOK, map[string]interface{}{
		"backup_codes": plainCodes,
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
			log.Error().Err(err).Msg("totp verify: failed to consume backup code")
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

	return c.JSON(http.StatusOK, map[string]string{"status": "verified"})
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
