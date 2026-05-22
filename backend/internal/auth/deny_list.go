package auth

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// denyListFallback provides a PostgreSQL-backed deny list used when Redis is unavailable.
// It is used by RevokeToken (write) and IsTokenRevoked (read).
type denyListFallback struct {
	db *pgxpool.Pool
}

// revokeInFallback writes a token hash to the PostgreSQL fallback table.
// Called by RevokeToken when Redis.Set fails.
func (f *denyListFallback) revokeInFallback(ctx context.Context, tokenHash string, expiresAt time.Time) {
	if f == nil || f.db == nil {
		return
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
	}
}

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
