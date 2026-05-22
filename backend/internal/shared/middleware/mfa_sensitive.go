// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package middleware

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// RequireMFASensitive returns middleware that enforces TOTP validation for sensitive
// endpoints when the org has require_mfa_sensitive_calls = true.
//
// The caller must pass `validateTOTP func(secret, code string) bool` — this avoids
// importing the auth package from shared/middleware (would create an import cycle).
//
// When MFA is not configured for the user or the org setting is off, the request
// passes through without any TOTP check.
func RequireMFASensitive(db *pgxpool.Pool, validateTOTP func(secret, code string) bool) echo.MiddlewareFunc {
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

			// Load user's TOTP secret.
			secret := loadUserTOTPSecret(c.Request().Context(), db, userID)
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
	_ = db.QueryRow(ctx,
		`SELECT require_mfa_sensitive_calls FROM organizations WHERE id = $1::uuid`, orgID,
	).Scan(&required)
	return required
}

func loadUserTOTPSecret(ctx context.Context, db *pgxpool.Pool, userID string) string {
	if db == nil {
		return ""
	}
	var secret string
	_ = db.QueryRow(ctx,
		`SELECT secret FROM totp_secrets WHERE user_id = $1::uuid AND enabled = true`, userID,
	).Scan(&secret)
	return secret
}
