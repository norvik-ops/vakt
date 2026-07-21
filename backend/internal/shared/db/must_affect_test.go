package db

import (
	"errors"
	"testing"

	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestMustAffect(t *testing.T) {
	// A real error propagates unchanged — a schema error must NOT be masked as
	// not-found (S121-F3). This is the whole reason tag and err stay separate.
	schemaErr := errors.New(`column "updated_at" does not exist (SQLSTATE 42703)`)
	if got := MustAffect(pgconn.CommandTag{}, schemaErr); got != schemaErr {
		t.Errorf("real error must propagate unchanged, got %v", got)
	}

	// Zero rows on a clean write → pgx.ErrNoRows (→ 404 via handler mapping).
	if got := MustAffect(pgconn.NewCommandTag("UPDATE 0"), nil); !errors.Is(got, pgx.ErrNoRows) {
		t.Errorf("zero rows must return pgx.ErrNoRows, got %v", got)
	}

	// A write that hit a row is a success.
	if got := MustAffect(pgconn.NewCommandTag("UPDATE 1"), nil); got != nil {
		t.Errorf("affected rows must return nil, got %v", got)
	}
	if got := MustAffect(pgconn.NewCommandTag("DELETE 3"), nil); got != nil {
		t.Errorf("affected rows must return nil, got %v", got)
	}
}
