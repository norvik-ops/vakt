package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// denyListFallback provides a PostgreSQL-backed deny list used when Redis is unavailable.
// It is used by RevokeToken (write) and IsTokenRevoked (read).
type denyListFallback struct {
	db *pgxpool.Pool
}

// revokeInFallback writes a token hash to the PostgreSQL fallback table and
// reports whether the revocation actually landed there.
//
// R1-W7A-N3: this used to log its failure and return nothing, which left its
// only caller (RevokeToken) unable to tell a persisted revocation from a lost
// one. "No fallback configured" is reported as an error too — not because it is
// a fault, but because from the caller's angle it is the same fact: this sink
// does not hold the revocation. RevokeToken decides what that means; here we
// only state it.
func (f *denyListFallback) revokeInFallback(ctx context.Context, tokenHash string, expiresAt time.Time) error {
	if f == nil || f.db == nil {
		return errNoDenyListFallback
	}
	ctx2, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := f.db.Exec(ctx2,
		`INSERT INTO token_deny_list_fallback (token_hash, expires_at)
		 VALUES ($1, $2)
		 ON CONFLICT (token_hash) DO NOTHING`,
		tokenHash, expiresAt)
	if err != nil {
		log.Warn().Err(err).Msg("deny-list fallback: write failed")
		return fmt.Errorf("deny-list fallback write: %w", err)
	}
	return nil
}

// errNoDenyListFallback marks the absence of a PostgreSQL fallback (no pool
// wired). Distinct from a write error so a caller can tell "not configured"
// from "configured and broken" if it ever needs to.
var errNoDenyListFallback = errors.New("deny-list fallback not configured")

// isRevokedInFallback checks the PostgreSQL fallback table.
// Returns true if the token is found and not yet expired.
func (f *denyListFallback) isRevokedInFallback(ctx context.Context, tokenHash string) bool {
	if f == nil || f.db == nil {
		return false
	}
	ctx2, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var exists bool
	err := f.db.QueryRow(ctx2,
		`SELECT EXISTS(
			SELECT 1 FROM token_deny_list_fallback
			WHERE token_hash = $1 AND expires_at > NOW()
		)`, tokenHash).Scan(&exists)
	if err != nil {
		log.Warn().Err(err).Msg("deny-list fallback: read failed")
		return false
	}
	return exists
}

// cleanupExpiredFallbackEntries removes expired rows from the fallback table.
// Called periodically by the auth cleanup Asynq job.
func cleanupExpiredFallbackEntries(ctx context.Context, db *pgxpool.Pool) {
	if db == nil {
		return
	}
	res, err := db.Exec(ctx,
		`DELETE FROM token_deny_list_fallback WHERE expires_at <= NOW()`)
	if err != nil {
		log.Warn().Err(err).Msg("deny-list fallback: cleanup failed")
		return
	}
	log.Debug().Int64("deleted", res.RowsAffected()).Msg("deny-list fallback: expired entries cleaned")
}
