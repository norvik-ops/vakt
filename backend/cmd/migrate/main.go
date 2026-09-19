// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

// Command migrate runs all pending database migrations and exits.
// Usage: VAKT_DB_URL=postgres://... go run ./cmd/migrate
package main

import (
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	shareddb "github.com/matharnica/vakt/internal/shared/db"
	"github.com/rs/zerolog"
)

func main() {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	dbURL := readEnvOrFile("VAKT_DB_URL", "VAKT_DB_URL_FILE", log)
	if dbURL == "" {
		dbURL = buildDBURLFromComponents(log)
	}
	if dbURL == "" {
		log.Fatal().Msg("VAKT_DB_URL or VAKT_DB_URL_FILE is required")
	}

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal().Msg("cannot determine migrations directory")
	}
	migrationsDir := filepath.Join(filepath.Dir(filename), "..", "..", "db", "migrations")

	// R1-B0-N2: Die Argumente wurden frueher GAR NICHT gelesen. Damit wendete
	// der dokumentierte Rueckweg `migrate down 1` alle ausstehenden
	// Up-Migrationen an — das Gegenteil dessen, was ein Operator im Fehlerfall
	// will, und ohne jeden Hinweis darauf. Ein unbekanntes Argument bricht
	// jetzt ab, statt still etwas anderes zu tun.
	args := os.Args[1:]
	switch {
	case len(args) == 0 || args[0] == "up":
		if len(args) > 1 {
			log.Fatal().Strs("args", args).Msg("`up` nimmt keine weiteren Argumente")
		}
		log.Info().Str("dir", migrationsDir).Msg("running migrations")
		if err := shareddb.RunMigrations(dbURL, migrationsDir); err != nil {
			log.Fatal().Err(err).Msg("migration failed")
		}
		log.Info().Msg("all migrations applied successfully")

	case args[0] == "down":
		// Die Schrittzahl ist Pflicht. Es gibt bewusst kein "down all": ein
		// Rollback ist destruktiv, und die Zahl ist die einzige Bremse
		// zwischen einem gezielten Rueckweg und einem leeren Schema.
		if len(args) != 2 {
			log.Fatal().Msg("`down` braucht genau eine Schrittzahl, z. B. `migrate down 1`")
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n <= 0 {
			log.Fatal().Str("arg", args[1]).Msg("die Schrittzahl muss eine positive Ganzzahl sein")
		}
		log.Warn().Int("steps", n).Str("dir", migrationsDir).
			Msg("rolling migrations BACK — this is destructive and cannot be undone")
		if err := shareddb.MigrateDown(dbURL, migrationsDir, n); err != nil {
			log.Fatal().Err(err).Msg("rollback failed")
		}
		log.Info().Int("steps", n).Msg("rollback applied successfully")

	default:
		log.Fatal().Str("arg", args[0]).Msg("unbekanntes Kommando — erlaubt sind `up` (Vorgabe) und `down <n>`")
	}
}

// buildDBURLFromComponents assembles a postgres DSN from VAKT_DB_PASSWORD_FILE
// plus non-secret host/port/user/dbname parts — mirrors internal/config's
// buildDBURLFromComponents. Needed because migrate doesn't import
// internal/config (see readEnvOrFile above) yet must support the same
// Docker-secret-mounted password as api/worker (S13/G-04).
func buildDBURLFromComponents(log zerolog.Logger) string {
	password := readEnvOrFile("VAKT_DB_PASSWORD", "VAKT_DB_PASSWORD_FILE", log)
	if password == "" {
		return ""
	}
	dsn := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(getEnv("VAKT_DB_USER", "vakt"), password),
		Host:     net.JoinHostPort(getEnv("VAKT_DB_HOST", "localhost"), getEnv("VAKT_DB_PORT", "5432")),
		Path:     "/" + getEnv("VAKT_DB_NAME", "vakt"),
		RawQuery: "sslmode=" + getEnv("VAKT_DB_SSLMODE", "disable"),
	}
	return dsn.String()
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
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
