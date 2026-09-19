package db

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// RunMigrations applies all pending up migrations from the given path.
func RunMigrations(dbURL, migrationsPath string) error {
	m, err := migrate.New("file://"+migrationsPath, "pgx5://"+dbURL[len("postgres://"):])
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}

// MigrateDown rolls back exactly n migration steps.
//
// R1-B0-N2: docs/update-guide.md dokumentiert seit jeher
// `docker compose run --rm api migrate down 1` als Rueckweg im Fehlerfall.
// cmd/migrate wertete os.Args aber gar nicht aus und rief immer m.Up() — der
// Befehl wendete also alle AUSSTEHENDEN Migrationen an, exakt das Gegenteil
// dessen, was ein Operator in dem Moment will, in dem er ihn eingibt.
//
// n MUSS positiv und ausdruecklich sein. Es gibt bewusst kein "down all":
// ein Rollback ist destruktiv, und die Schrittzahl ist die einzige Bremse
// zwischen einem gezielten Rueckweg und einem leeren Schema.
func MigrateDown(dbURL, migrationsPath string, n int) error {
	if n <= 0 {
		return fmt.Errorf("migrate down: die Schrittzahl muss positiv sein (bekommen: %d)", n)
	}
	m, err := migrate.New("file://"+migrationsPath, "pgx5://"+dbURL[len("postgres://"):])
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Steps(-n); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("rollback %d step(s): %w", n, err)
	}
	return nil
}
