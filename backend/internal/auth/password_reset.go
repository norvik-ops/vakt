// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/smtp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/matharnica/vakt/internal/shared/logsafe"
	"github.com/matharnica/vakt/internal/shared/mailhdr"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

// ErrTokenInvalid is returned when the reset token is not found, already used,
// or expired.
var ErrTokenInvalid = errors.New("token invalid or expired")

// resetThrottleMax is the maximum number of password reset emails per address
// within resetThrottleTTL. Chosen to allow legitimate use (lost password,
// typo in new password) while preventing inbox-spam abuse.
const (
	resetThrottleMax = 3
	resetThrottleTTL = time.Hour
)

// RequestPasswordReset generates a reset token for the given email address and
// sends it via SMTP. If the email is not found in the DB the function returns
// nil without error — callers must not distinguish found/not-found.
func (s *Service) RequestPasswordReset(ctx context.Context, email, frontendURL, smtpHost, smtpPort, smtpUser, smtpPass, smtpFrom string) error {
	// Look up user — no error if not found (avoid enumeration).
	var userID string
	err := s.db.QueryRow(ctx,
		`SELECT id::text FROM users WHERE email = $1 AND is_active = TRUE`, email,
	).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("lookup user: %w", err)
	}

	// Per-email throttle: suppress emails silently once the limit is reached.
	// Fail-open: if Redis is unavailable the request proceeds normally.
	throttleKey := "reset_req:" + email
	if cnt, incrErr := s.redis.Incr(ctx, throttleKey).Result(); incrErr == nil {
		if cnt == 1 {
			_ = s.redis.Expire(ctx, throttleKey, resetThrottleTTL).Err()
		}
		if cnt > resetThrottleMax {
			log.Warn().Str("email_redacted", logsafe.RedactEmail(email)).Msg("password reset: per-email rate limit reached, suppressing")
			return nil
		}
	} else {
		log.Warn().Err(incrErr).Msg("password reset: redis throttle unavailable, failing open")
	}

	// Generate 32 cryptographically random bytes as the raw token.
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Errorf("generate reset token: %w", err)
	}
	rawHex := hex.EncodeToString(raw)

	// Store the SHA-256 hash (never the raw token).
	hash := sha256.Sum256([]byte(rawHex))
	tokenHash := hex.EncodeToString(hash[:])

	_, err = s.db.Exec(ctx, `
		INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
		VALUES ($1::uuid, $2, now() + interval '1 hour')`,
		userID, tokenHash,
	)
	if err != nil {
		return fmt.Errorf("insert reset token: %w", err)
	}

	// Look up the user's preferred language for the email.
	var lang string
	_ = s.db.QueryRow(ctx, `SELECT preferred_language FROM users WHERE id = $1::uuid`, userID).Scan(&lang)
	if lang == "" {
		lang = "de"
	}

	// Send email — non-fatal if SMTP fails (log and return nil).
	resetLink := frontendURL + "/auth/reset-password?token=" + rawHex
	if err := sendPasswordResetEmail(smtpHost, smtpPort, smtpUser, smtpPass, smtpFrom, email, resetLink, lang); err != nil {
		log.Error().Err(err).Str("email_redacted", logsafe.RedactEmail(email)).Msg("password reset: send email failed")
	}
	return nil
}

// GeneratePasswordResetLink creates a reset token for the given email and returns the full reset link.
// Unlike RequestPasswordReset it does not send email — the caller is responsible for delivery.
// Returns ("", nil) if the email address is not found.
func (s *Service) GeneratePasswordResetLink(ctx context.Context, email, frontendURL string) (string, error) {
	var userID string
	err := s.db.QueryRow(ctx,
		`SELECT id::text FROM users WHERE email = $1 AND is_active = TRUE`, email,
	).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("lookup user: %w", err)
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate reset token: %w", err)
	}
	rawHex := hex.EncodeToString(raw)

	hash := sha256.Sum256([]byte(rawHex))
	tokenHash := hex.EncodeToString(hash[:])

	_, err = s.db.Exec(ctx, `
		INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
		VALUES ($1::uuid, $2, now() + interval '1 hour')`,
		userID, tokenHash,
	)
	if err != nil {
		return "", fmt.Errorf("insert reset token: %w", err)
	}

	resetLink := frontendURL + "/auth/reset-password?token=" + rawHex
	return resetLink, nil
}

// ResetPassword validates the raw token, updates the user's password, and marks
// the token as used. Returns ErrTokenInvalid for any invalid/expired/used token.
func (s *Service) ResetPassword(ctx context.Context, rawToken, newPassword string) error {
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])

	var tokenID, userID string
	var expiresAt time.Time
	var usedAt *time.Time

	err := s.db.QueryRow(ctx, `
		SELECT id::text, user_id::text, expires_at, used_at
		FROM password_reset_tokens
		WHERE token_hash = $1`,
		tokenHash,
	).Scan(&tokenID, &userID, &expiresAt, &usedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrTokenInvalid
	}
	if err != nil {
		return fmt.Errorf("lookup reset token: %w", err)
	}
	if usedAt != nil {
		return ErrTokenInvalid
	}
	if time.Now().UTC().After(expiresAt) {
		return ErrTokenInvalid
	}

	// Enforce password complexity on reset.
	if err := validatePasswordStrength(newPassword); err != nil {
		return err
	}

	// Hash the new password.
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	// Update password and mark token as used in a single transaction.
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	_, err = tx.Exec(ctx,
		`UPDATE users SET password_hash = $1 WHERE id = $2::uuid`,
		string(hashed), userID,
	)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	_, err = tx.Exec(ctx,
		`UPDATE password_reset_tokens SET used_at = now() WHERE id = $1::uuid`,
		tokenID,
	)
	if err != nil {
		return fmt.Errorf("mark token used: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return err
	}

	// Invalidate all existing Paseto tokens for this user by incrementing the
	// password version counter (S87-6: PG durable source of truth + Redis mirror;
	// checkPwVersion falls back to PG through a Redis outage). Shared with the
	// offboarding revocation path (S131-C1).
	s.bumpPwVersion(ctx, userID)

	// AUTH-002: Revoke all refresh sessions so that an attacker holding a stolen
	// refresh token cannot call POST /auth/refresh after a password reset and
	// receive a new access token with the current pw_version — bypassing the
	// pw_version invalidation above. Belt-and-suspenders: the password has already
	// been changed and pw_version incremented; any failure here is logged only.
	revokeCtx, revokeCancel := context.WithTimeout(ctx, 3*time.Second)
	defer revokeCancel()
	rows, revokeErr := s.db.Query(revokeCtx,
		`DELETE FROM refresh_sessions WHERE user_id = $1::uuid RETURNING token_hash`,
		userID,
	)
	if revokeErr != nil {
		log.Error().Err(revokeErr).Str("user_id", userID).Msg("password reset: failed to revoke refresh sessions")
	} else {
		defer rows.Close()
		var keys []string
		for rows.Next() {
			var h string
			if scanErr := rows.Scan(&h); scanErr == nil {
				keys = append(keys, "refresh:"+h)
			}
		}
		if s.redis != nil && len(keys) > 0 {
			if delErr := s.redis.Del(revokeCtx, keys...).Err(); delErr != nil {
				log.Error().Err(delErr).Str("user_id", userID).Int("count", len(keys)).
					Msg("password reset: failed to remove refresh tokens from Redis")
			}
		}
	}

	return nil
}

// resetEmailContent returns the localised subject and body (with %s for the reset link) for a
// password-reset email. Falls back to German for unknown language codes.
func resetEmailContent(lang string) (subject, bodyTemplate string) {
	switch lang {
	case "en":
		return "Reset your password — Vakt",
			"Hello,\r\n\r\n" +
				"You requested a password reset.\r\n\r\n" +
				"Click the link below to set a new password:\r\n\r\n" +
				"%s\r\n\r\n" +
				"This link is valid for 1 hour.\r\n\r\n" +
				"If you did not request this, you can ignore this email.\r\n\r\n" +
				"Your Vakt Team\r\n"
	case "fr":
		return "Réinitialiser votre mot de passe — Vakt",
			"Bonjour,\r\n\r\n" +
				"Vous avez demandé une réinitialisation de mot de passe.\r\n\r\n" +
				"Cliquez sur le lien ci-dessous pour définir un nouveau mot de passe :\r\n\r\n" +
				"%s\r\n\r\n" +
				"Ce lien est valable 1 heure.\r\n\r\n" +
				"Si vous n'avez pas effectué cette demande, ignorez cet e-mail.\r\n\r\n" +
				"Votre équipe Vakt\r\n"
	case "nl":
		return "Wachtwoord opnieuw instellen — Vakt",
			"Hallo,\r\n\r\n" +
				"U heeft een wachtwoordreset aangevraagd.\r\n\r\n" +
				"Klik op de onderstaande link om een nieuw wachtwoord in te stellen:\r\n\r\n" +
				"%s\r\n\r\n" +
				"Deze link is 1 uur geldig.\r\n\r\n" +
				"Als u dit niet heeft aangevraagd, kunt u deze e-mail negeren.\r\n\r\n" +
				"Uw Vakt-team\r\n"
	default: // "de"
		return "Passwort zurücksetzen — Vakt",
			"Hallo,\r\n\r\n" +
				"Sie haben das Zurücksetzen Ihres Passworts angefordert.\r\n\r\n" +
				"Klicken Sie auf den folgenden Link, um ein neues Passwort festzulegen:\r\n\r\n" +
				"%s\r\n\r\n" +
				"Dieser Link ist 1 Stunde lang gültig.\r\n\r\n" +
				"Falls Sie diese Anfrage nicht gestellt haben, können Sie diese E-Mail ignorieren.\r\n\r\n" +
				"Ihr Vakt-Team\r\n"
	}
}

// sendPasswordResetEmail delivers a plain-text reset email via stdlib net/smtp.
func sendPasswordResetEmail(host, port, user, pass, from, to, resetLink, lang string) error {
	if host == "" {
		return fmt.Errorf("SMTP host not configured")
	}
	if from == "" {
		from = "noreply@vakt.local"
	}
	if port == "" {
		port = "25"
	}

	subject, bodyTpl := resetEmailContent(lang)
	body := fmt.Sprintf(bodyTpl, resetLink)

	headers := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n",
		mailhdr.Sanitize(from), mailhdr.Sanitize(to), mailhdr.Sanitize(subject),
	)
	msg := []byte(headers + body)
	addr := host + ":" + port

	if user != "" && pass != "" {
		auth := smtp.PlainAuth("", user, pass, host)
		return smtp.SendMail(addr, auth, from, []string{to}, msg)
	}
	return smtp.SendMail(addr, nil, from, []string{to}, msg)
}
