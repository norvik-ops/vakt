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
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

// ErrTokenInvalid is returned when the reset token is not found, already used,
// or expired.
var ErrTokenInvalid = errors.New("token invalid or expired")

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

	// Send email — non-fatal if SMTP fails (log and return nil).
	resetLink := frontendURL + "/auth/reset-password?token=" + rawHex
	if err := sendPasswordResetEmail(smtpHost, smtpPort, smtpUser, smtpPass, smtpFrom, email, resetLink); err != nil {
		log.Error().Err(err).Str("email", email).Msg("password reset: send email failed")
	}
	return nil
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

	return tx.Commit(ctx)
}

// sendPasswordResetEmail delivers a plain-text reset email via stdlib net/smtp.
func sendPasswordResetEmail(host, port, user, pass, from, to, resetLink string) error {
	if host == "" {
		return fmt.Errorf("SMTP host not configured")
	}
	if from == "" {
		from = "noreply@vakt.local"
	}
	if port == "" {
		port = "25"
	}

	subject := "Passwort zurücksetzen — Vakt"
	body := "Hallo,\r\n\r\n" +
		"Sie haben das Zurücksetzen Ihres Passworts angefordert.\r\n\r\n" +
		"Klicken Sie auf den folgenden Link, um ein neues Passwort festzulegen:\r\n\r\n" +
		resetLink + "\r\n\r\n" +
		"Dieser Link ist 1 Stunde lang gültig.\r\n\r\n" +
		"Falls Sie diese Anfrage nicht gestellt haben, können Sie diese E-Mail ignorieren.\r\n\r\n" +
		"Ihr Vakt-Team\r\n"

	headers := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n",
		from, to, subject,
	)
	msg := []byte(headers + body)
	addr := host + ":" + port

	if user != "" && pass != "" {
		auth := smtp.PlainAuth("", user, pass, host)
		return smtp.SendMail(addr, auth, from, []string{to}, msg)
	}
	return smtp.SendMail(addr, nil, from, []string{to}, msg)
}
