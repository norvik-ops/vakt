// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package middleware

import (
	"context"
	"encoding/hex"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	sharedcrypto "github.com/matharnica/vakt/internal/shared/crypto"
)

// RequireMFASensitive returns middleware that enforces TOTP validation for sensitive
// endpoints when the org has require_mfa_sensitive_calls = true.
//
// The caller must pass:
//   - db: database pool for looking up org settings and encrypted TOTP secrets
//   - masterKey: platform master key ([]byte) used to decrypt the stored TOTP secret.
//     TOTP secrets are stored AES-256-GCM encrypted; passing the wrong or nil key
//     will cause the middleware to block all requests as if MFA is not configured.
//   - validateTOTP: func(plaintextSecret, code string) bool — avoids importing
//     the auth package from shared/middleware (would create an import cycle).
//
// When MFA is not configured for the user or the org setting is off, the request
// passes through without any TOTP check.
func RequireMFASensitive(db *pgxpool.Pool, masterKey []byte, validateTOTP func(secret, code string) bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			orgID, _ := c.Get("org_id").(string)
			userID, _ := c.Get("user_id").(string)
			if orgID == "" || userID == "" {
				return next(c)
			}

			// Check org setting.
			required := isMFARequiredForSensitiveCalls(c.Request().Context(), db, orgID)
			if !required {
				return next(c)
			}

			// Load user's encrypted TOTP secret and decrypt it.
			secret := loadAndDecryptUserTOTPSecret(c.Request().Context(), db, masterKey, userID)
			if secret == "" {
				// User has no MFA configured — block rather than silently allow,
				// since the org policy requires MFA for sensitive calls.
				log.Warn().Str("user_id", userID).Msg("mfa_sensitive: user has no TOTP configured but org requires MFA")
				return c.JSON(http.StatusForbidden, map[string]string{
					"error": "MFA required for this action. Please configure TOTP first.",
					"code":  "MFA_NOT_CONFIGURED",
				})
			}

			code := c.Request().Header.Get("X-MFA-Token")
			if code == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "X-MFA-Token header required for this action",
					"code":  "MFA_TOKEN_REQUIRED",
				})
			}
			if !validateTOTP(secret, code) {
				log.Warn().Str("user_id", userID).Msg("mfa_sensitive: invalid TOTP code")
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "Invalid or expired MFA token",
					"code":  "MFA_TOKEN_INVALID",
				})
			}
			return next(c)
		}
	}
}

func isMFARequiredForSensitiveCalls(ctx context.Context, db *pgxpool.Pool, orgID string) bool {
	if db == nil {
		return false
	}
	var required bool
	if err := db.QueryRow(ctx,
		`SELECT require_mfa_sensitive_calls FROM organizations WHERE id = $1::uuid`, orgID,
	).Scan(&required); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("mfa_sensitive: could not load MFA requirement — defaulting to false")
	}
	return required
}

// loadAndDecryptUserTOTPSecret reads the AES-256-GCM encrypted TOTP secret from
// the database and decrypts it using masterKey. Returns the plaintext TOTP secret,
// or an empty string if the user has no TOTP configured or decryption fails.
func loadAndDecryptUserTOTPSecret(ctx context.Context, db *pgxpool.Pool, masterKey []byte, userID string) string {
	if db == nil {
		return ""
	}
	var cipherhex string
	if err := db.QueryRow(ctx,
		`SELECT secret FROM totp_secrets WHERE user_id = $1::uuid AND enabled = true`, userID,
	).Scan(&cipherhex); err != nil {
		log.Warn().Err(err).Str("user_id", userID).Msg("mfa_sensitive: could not load TOTP secret")
		return ""
	}
	if cipherhex == "" {
		return ""
	}
	ct, err := hex.DecodeString(cipherhex)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("mfa_sensitive: could not hex-decode TOTP secret ciphertext")
		return ""
	}
	plain, err := sharedcrypto.Decrypt(masterKey, ct)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("mfa_sensitive: could not decrypt TOTP secret")
		return ""
	}
	return string(plain)
}
