package db

import (
	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// MustAffect converts a write whose zero-rows result is a caller error — the target
// row did not exist — into pgx.ErrNoRows, the codebase's canonical not-found signal
// that handler error-mapping turns into HTTP 404.
//
// It is an OPT-IN helper (S131-A1, ADR-0075), deliberately NOT a blanket default:
//   - A real DB/schema error is returned unchanged, so a 42703 (missing column) still
//     surfaces as 500 and is never masked as 404 — the tag/err split preserves the
//     S121-F3 invariant.
//   - Use it ONLY where zero rows genuinely means "not found": UPDATE/DELETE ... WHERE
//     id = $1 that back a single-resource handler. Do NOT use it for idempotent writes,
//     `ON CONFLICT ... DO NOTHING` dedup, or cleanup/retention jobs, where zero rows is
//     a valid, expected outcome — there a false ErrNoRows would create a 404 bug where
//     none existed.
//
// Typical call site in a repository method:
//
//	tag, err := r.db.Exec(ctx, `UPDATE ... WHERE id = $1 AND org_id = $2`, id, orgID)
//	return MustAffect(tag, err)
func MustAffect(tag pgconn.CommandTag, err error) error {
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
