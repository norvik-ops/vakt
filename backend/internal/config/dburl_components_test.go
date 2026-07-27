// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/matharnica/vakt/internal/config"
)

// S13/G-04: with Docker secrets, the DB password arrives as a mounted file
// (VAKT_DB_PASSWORD_FILE) instead of a pre-composed VAKT_DB_URL — Compose
// cannot interpolate one secret into another. Load() must assemble the DSN
// from that file plus the non-secret host/port/user/dbname parts.
func TestLoad_DBURLFromPasswordFile(t *testing.T) {
	secretFile := filepath.Join(t.TempDir(), "postgres_password")
	if err := os.WriteFile(secretFile, []byte("s3cr3t/pass?word\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("VAKT_DB_URL", "")
	t.Setenv("VAKT_DB_URL_FILE", "")
	t.Setenv("VAKT_DB_PASSWORD_FILE", secretFile)
	t.Setenv("VAKT_DB_HOST", "pgbouncer")
	t.Setenv("VAKT_SECRET_KEY", "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	want := "postgres://vakt:s3cr3t%2Fpass%3Fword@pgbouncer:5432/vakt?sslmode=disable"
	if cfg.DBUrl != want {
		t.Fatalf("DBUrl = %q, want %q", cfg.DBUrl, want)
	}
}

// A full VAKT_DB_URL/VAKT_DB_URL_FILE must still win over the component
// path — external/managed Postgres setups that don't use the compose
// secret mount depend on this.
func TestLoad_DBURLFullURLTakesPrecedenceOverComponents(t *testing.T) {
	t.Setenv("VAKT_DB_URL", "postgres://someone:else@external-host:5432/vakt?sslmode=require")
	t.Setenv("VAKT_DB_URL_FILE", "")
	t.Setenv("VAKT_DB_PASSWORD_FILE", "")
	t.Setenv("VAKT_DB_PASSWORD", "should-be-ignored")
	t.Setenv("VAKT_SECRET_KEY", "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	want := "postgres://someone:else@external-host:5432/vakt?sslmode=require"
	if cfg.DBUrl != want {
		t.Fatalf("DBUrl = %q, want %q", cfg.DBUrl, want)
	}
}

// Neither a full URL nor a password component present: DBUrl stays empty
// so the existing Validate() error path ("VAKT_DB_URL is required") fires,
// instead of silently building a DSN with an empty password.
func TestLoad_DBURLEmptyWithoutURLOrPassword(t *testing.T) {
	t.Setenv("VAKT_DB_URL", "")
	t.Setenv("VAKT_DB_URL_FILE", "")
	t.Setenv("VAKT_DB_PASSWORD", "")
	t.Setenv("VAKT_DB_PASSWORD_FILE", "")
	t.Setenv("VAKT_SECRET_KEY", "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.DBUrl != "" {
		t.Fatalf("DBUrl = %q, want empty", cfg.DBUrl)
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected Validate() to reject an empty DBUrl")
	}
}
