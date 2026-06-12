package vaktcomply

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Additional sentinel errors for vaktcomply service layer. Handlers use
// errors.Is to map these to HTTP status codes without fragile string matching.
// ErrNotFound and ErrDORANotEnabled are declared in service.go.
var (
	ErrAlreadySubmitted      = errors.New("already submitted")
	ErrNotConfigured         = errors.New("nicht konfiguriert")
	ErrInvalidMaturityScore  = errors.New("maturity_score must be between 0 and 3")
	ErrInvalidProtection     = errors.New("invalid protection_level")
	ErrInvalidAssessment     = errors.New("invalid assessment_level")
	ErrInvalidOptions        = errors.New("multiple_choice question requires non-empty options")
	ErrConflict              = errors.New("resource already exists")
	ErrCycle                 = errors.New("adding this dependency would create a cycle")
	ErrOverrideReasonMissing = errors.New("override_reason is required when setting an override")
)

// isNotFound returns true for any "resource does not exist" error — either the
// service-layer ErrNotFound sentinel or a raw pgx.ErrNoRows from the repository.
func isNotFound(err error) bool {
	return errors.Is(err, ErrNotFound) || errors.Is(err, pgx.ErrNoRows)
}

// isUniqueViolation returns true for PostgreSQL SQLSTATE 23505 (unique_violation).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
