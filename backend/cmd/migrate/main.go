// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

// Command migrate runs all pending database migrations and exits.
// Usage: VAKT_DB_URL=postgres://... go run ./cmd/migrate
package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	shareddb "github.com/matharnica/vakt/internal/shared/db"
	"github.com/rs/zerolog"
)

func main() {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	dbURL := readEnvOrFile("VAKT_DB_URL", "VAKT_DB_URL_FILE", log)
	if dbURL == "" {
		log.Fatal().Msg("VAKT_DB_URL or VAKT_DB_URL_FILE is required")
	}

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal().Msg("cannot determine migrations directory")
	}
	migrationsDir := filepath.Join(filepath.Dir(filename), "..", "..", "db", "migrations")

	log.Info().Str("dir", migrationsDir).Msg("running migrations")
	if err := shareddb.RunMigrations(dbURL, migrationsDir); err != nil {
		log.Fatal().Err(err).Msg("migration failed")
	}
	log.Info().Msg("all migrations applied successfully")
}

func readEnvOrFile(envKey, fileKey string, log zerolog.Logger) string {
	if f := os.Getenv(fileKey); f != "" {
		if !strings.HasPrefix(f, "/") {
			log.Fatal().Str("key", fileKey).Str("value", f).Msg("must be an absolute path")
		}
		b, err := os.ReadFile(f) // #nosec G703 — operator-controlled path
		if err != nil {
			log.Fatal().Err(err).Str("file", f).Msgf("cannot read %s", fileKey)
		}
		return strings.TrimSpace(string(b))
	}
	return os.Getenv(envKey)
}
