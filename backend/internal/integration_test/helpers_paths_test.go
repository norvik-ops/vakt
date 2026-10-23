//go:build integration

package integration_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// migrationsDir returns the absolute path to backend/db/migrations from the
// test file's runtime location (works regardless of working directory). It lived
// in hr_evidence_real_test.go until the HR module was archived.
func migrationsDir(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed")
	// here = .../backend/internal/integration_test/helpers_paths_test.go
	return filepath.Join(filepath.Dir(here), "..", "..", "db", "migrations")
}
