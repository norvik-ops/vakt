// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
)

// S13/G-04: mirrors internal/config's equivalent test — migrate has its own
// copy of this logic (see readEnvOrFile above) because it deliberately
// doesn't import internal/config.
func TestBuildDBURLFromComponents(t *testing.T) {
	// NOTE for the next negative test: with a Nop logger, log.Fatal() is a
	// no-op — zerolog returns a nil event, Msg() just returns, and os.Exit is
	// never reached. The two guards in readEnvOrFile (relative path, unreadable
	// file) are therefore NOT fatal here; they fall through and yield "".
	// Asserting "this input is rejected" against this logger would pass
	// vacuously. Use a logger with an output writer if you need that.
	log := zerolog.Nop()

	t.Run("builds DSN from password file", func(t *testing.T) {
		secretFile := filepath.Join(t.TempDir(), "postgres_password")
		if err := os.WriteFile(secretFile, []byte("s3cr3t/pass?word\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("VAKT_DB_PASSWORD_FILE", secretFile)
		t.Setenv("VAKT_DB_PASSWORD", "")
		t.Setenv("VAKT_DB_HOST", "postgres")

		got := buildDBURLFromComponents(log)
		want := "postgres://vakt:s3cr3t%2Fpass%3Fword@postgres:5432/vakt?sslmode=disable"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("empty without a password", func(t *testing.T) {
		t.Setenv("VAKT_DB_PASSWORD_FILE", "")
		t.Setenv("VAKT_DB_PASSWORD", "")

		if got := buildDBURLFromComponents(log); got != "" {
			t.Fatalf("got %q, want empty", got)
		}
	})
}
