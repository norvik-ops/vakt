//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/auth"
)

// R1-W7A-N3, class sweep — a second factor that could not be spent has not been
// spent.
//
// Both backup-code paths (TotpHandler.Verify and the shared
// validateSecondFactor behind /2fa/login-verify) matched the code, removed it
// from the slice in memory, and then treated the UPDATE that writes the shortened
// list back as best-effort: log the error, answer "verified". If that write
// failed, the code stayed in totp_secrets.backup_codes and remained valid — a
// SINGLE-USE recovery credential quietly became a permanent one, and the login it
// let through looked exactly like a correct one.
//
// This needs a real Postgres for the reason the defect survived: the failure is
// in the write, not in the logic. A fake store returns whatever it is told, so
// the interesting question — is the code still in the table afterwards? — has no
// meaningful answer unless there is a table.
//
// The write is broken with a BEFORE UPDATE trigger rather than by tearing down
// the pool: it fails exactly one statement on exactly one table, so a green
// baseline in the same test proves the harness works and the red case is not a
// side effect of a broken connection.
func TestBackupCodeThatCannotBeConsumedIsNotAccepted(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	pool, rdb, orgID := bootLogoutStack(ctx, t)
	key := mustKeyIntegration(t)
	svc := auth.NewService(pool, rdb, key)
	handler := auth.NewTotpHandler(pool, make([]byte, 32), svc)

	email, _ := seedLoginUser(ctx, t, pool, orgID, "backup-codes")
	var userID string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id::text FROM users WHERE email = $1`, email).Scan(&userID))

	plain, hashed, err := auth.GenerateBackupCodes()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(plain), 2, "need two codes: one for the baseline, one for the failure")

	// The backup-code path never decrypts the TOTP secret, so any non-empty
	// placeholder is enough here — the test is about the code list.
	_, err = pool.Exec(ctx, `
		INSERT INTO totp_secrets (user_id, secret, enabled, backup_codes)
		VALUES ($1::uuid, 'placeholder-not-used-on-the-backup-path', TRUE, $2)`,
		userID, hashed)
	require.NoError(t, err)

	verify := func(backupCode string) *httptest.ResponseRecorder {
		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/2fa/verify",
			strings.NewReader(`{"backup_code":"`+backupCode+`"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("user_id", userID) // what the auth middleware supplies on this route
		require.NoError(t, handler.Verify(c))
		return rec
	}

	// ── Baseline: a healthy store consumes the code and accepts it ───────────
	rec := verify(plain[0])
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	assert.Equal(t, len(hashed)-1, countBackupCodes(ctx, t, pool, userID),
		"baseline: a verified code must be struck off the list")

	// ── Break exactly one write ─────────────────────────────────────────────
	_, err = pool.Exec(ctx, `
		CREATE FUNCTION refuse_totp_update() RETURNS trigger AS $$
		BEGIN RAISE EXCEPTION 'simulated write failure'; END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER refuse_totp_update BEFORE UPDATE ON totp_secrets
		FOR EACH ROW EXECUTE FUNCTION refuse_totp_update();`)
	require.NoError(t, err)

	remaining := countBackupCodes(ctx, t, pool, userID)
	rec = verify(plain[1])

	require.NotEqual(t, http.StatusOK, rec.Code,
		"a backup code that could not be struck off the list is still spendable — "+
			"answering 'verified' hands out an unlimited second factor; body: %s", rec.Body.String())
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code,
		"nothing was wrong with the code, the store was — 503, not 422")

	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "TOTP_BACKUP_CODE_NOT_CONSUMED", body["code"])

	assert.Equal(t, remaining, countBackupCodes(ctx, t, pool, userID),
		"the code is demonstrably still on the list — which is exactly why the "+
			"old answer of 200 was a lie")
}

func countBackupCodes(ctx context.Context, t *testing.T, pool *pgxpool.Pool, userID string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT cardinality(backup_codes) FROM totp_secrets WHERE user_id = $1::uuid`,
		userID).Scan(&n))
	return n
}
