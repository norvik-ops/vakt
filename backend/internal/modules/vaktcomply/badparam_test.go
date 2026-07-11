// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktcomply/bsi"
)

// TestIsBadParam_ClassifiesInputErrors is the S121-F3 (P4) unit check: the
// classifier must recognise the BSI unknown-report-type sentinel and Postgres
// 22P02/22003 (which is what a malformed UUID/number in a path param produces),
// while leaving genuine failures to fall through to 500.
func TestIsBadParam_ClassifiesInputErrors(t *testing.T) {
	assert.True(t, isBadParam(fmt.Errorf("wrap: %w", bsi.ErrUnknownReportType)),
		"unknown BSI report type is a client mistake")
	assert.True(t, isBadParam(&pgconn.PgError{Code: "22P02"}), "malformed UUID → 22P02 → bad param")
	assert.True(t, isBadParam(&pgconn.PgError{Code: "22003"}), "numeric out of range → bad param")

	assert.False(t, isBadParam(nil))
	assert.False(t, isBadParam(errors.New("some other failure")))
	assert.False(t, isBadParam(pgx.ErrNoRows), "no-rows is not-found (404), not bad-param (400)")
	assert.False(t, isNotFound(&pgconn.PgError{Code: "22P02"}), "bad-param must not be misread as not-found")
}

// TestIsBadParam_RealMalformedUUID drives an actual query with a malformed UUID
// against Postgres and confirms the error it produces is classified as bad-param
// — i.e. that 22P02 really is the SQLSTATE and isBadParam really catches it. This
// is the non-vacuous half: it fails if a driver/version ever changes the code.
func TestIsBadParam_RealMalformedUUID(t *testing.T) {
	dbURL := os.Getenv("VAKT_DB_URL")
	if dbURL == "" {
		t.Skip("VAKT_DB_URL not set — needs a real Postgres (set in CI)")
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)
	defer pool.Close()

	// A malformed UUID cast — exactly what a bad :id path param triggers deeper down.
	var out string
	err = pool.QueryRow(context.Background(),
		`SELECT $1::uuid::text`, "not-a-valid-uuid").Scan(&out)
	require.Error(t, err)
	assert.True(t, isBadParam(err),
		"a malformed UUID must be classified as bad-param (400), got: %v", err)
	assert.False(t, isNotFound(err))
}
