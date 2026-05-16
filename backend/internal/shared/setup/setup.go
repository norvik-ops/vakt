// Package setup provides first-run detection and the one-time setup endpoint.
package setup

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// IsSetupComplete reports whether at least one organization exists in the
// database.  When the database is unreachable it returns false so the
// setup flow can be shown rather than a cryptic error page.
func IsSetupComplete(ctx context.Context, db *pgxpool.Pool) (bool, error) {
	var count int
	err := db.QueryRow(ctx, `SELECT COUNT(*) FROM organizations`).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check setup: %w", err)
	}
	return count > 0, nil
}
