package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	recoveryCodeCount = 8
)

// recoveryCodeBcryptCost is the bcrypt cost factor used for recovery-code hashes.
// var (not const) so tests can override it to bcrypt.MinCost for fast runs —
// the format/uniqueness/length assertions don't depend on cost.
var recoveryCodeBcryptCost = 12

// generateRecoveryCodes creates 8 single-use recovery codes in the format
// XXXX-XXXX-XXXX (12 hex chars, grouped in threes).
// Returns the plaintext codes (for display) and their bcrypt hashes (for storage).
func generateRecoveryCodes() (plainCodes []string, hashedCodes []string, err error) {
	plainCodes = make([]string, recoveryCodeCount)
	hashedCodes = make([]string, recoveryCodeCount)

	for i := range plainCodes {
		raw := make([]byte, 6)
		if _, e := rand.Read(raw); e != nil {
			return nil, nil, fmt.Errorf("generate recovery code random bytes: %w", e)
		}
		encoded := strings.ToUpper(hex.EncodeToString(raw)) // 12 hex chars
		plain := encoded[0:4] + "-" + encoded[4:8] + "-" + encoded[8:12]
		plainCodes[i] = plain

		hash, e := bcrypt.GenerateFromPassword([]byte(plain), recoveryCodeBcryptCost)
		if e != nil {
			return nil, nil, fmt.Errorf("hash recovery code: %w", e)
		}
		hashedCodes[i] = string(hash)
	}
	return plainCodes, hashedCodes, nil
}

// StoreRecoveryCodes deletes all existing recovery codes for the user and
// inserts fresh ones from hashedCodes.
func (h *TotpHandler) StoreRecoveryCodes(ctx context.Context, userID string, hashedCodes []string) error {
	tx, err := h.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	_, err = tx.Exec(ctx,
		`DELETE FROM auth_recovery_codes WHERE user_id = $1::uuid`, userID,
	)
	if err != nil {
		return fmt.Errorf("delete old recovery codes: %w", err)
	}

	for _, hash := range hashedCodes {
		_, err = tx.Exec(ctx,
			`INSERT INTO auth_recovery_codes (user_id, code_hash) VALUES ($1::uuid, $2)`,
			userID, hash,
		)
		if err != nil {
			return fmt.Errorf("insert recovery code: %w", err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit recovery codes: %w", err)
	}
	return nil
}

// VerifyRecoveryCode finds an unused recovery code matching candidate for the
// given user, marks it as used, and returns nil on success.
// Returns a typed error with code "AUTH_INVALID_RECOVERY_CODE" if no match is found.
func (h *TotpHandler) VerifyRecoveryCode(ctx context.Context, userID, candidate string) error {
	// Fetch all unused hashes for this user.
	rows, err := h.db.Query(ctx,
		`SELECT id::text, code_hash FROM auth_recovery_codes
		 WHERE user_id = $1::uuid AND used_at IS NULL`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("query recovery codes: %w", err)
	}
	defer rows.Close()

	type row struct {
		id   string
		hash string
	}
	var codes []row
	for rows.Next() {
		var r row
		if e := rows.Scan(&r.id, &r.hash); e != nil {
			return fmt.Errorf("scan recovery code: %w", e)
		}
		codes = append(codes, r)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate recovery codes: %w", err)
	}

	// Try each hash.
	for _, c := range codes {
		if bcrypt.CompareHashAndPassword([]byte(c.hash), []byte(candidate)) == nil {
			// Mark as used.
			_, err = h.db.Exec(ctx,
				`UPDATE auth_recovery_codes SET used_at = NOW() WHERE id = $1::uuid`,
				c.id,
			)
			if err != nil {
				return fmt.Errorf("mark recovery code used: %w", err)
			}
			return nil
		}
	}

	return &recoveryCodeError{}
}

// recoveryCodeError is returned when no matching recovery code is found.
type recoveryCodeError struct{}

func (e *recoveryCodeError) Error() string { return "AUTH_INVALID_RECOVERY_CODE" }
func (e *recoveryCodeError) Code() string  { return "AUTH_INVALID_RECOVERY_CODE" }
