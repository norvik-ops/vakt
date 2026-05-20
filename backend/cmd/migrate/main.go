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

	shareddb "github.com/matharnica/vakt/internal/shared/db"
	"github.com/rs/zerolog"
)

func main() {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	dbURL := os.Getenv("VAKT_DB_URL")
	if dbURL == "" {
		log.Fatal().Msg("VAKT_DB_URL is required")
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
