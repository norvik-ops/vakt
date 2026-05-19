package db_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/shared/db"
)

func TestConnect_RequiresDBURL(t *testing.T) {
	dbURL := os.Getenv("TEST_DB_URL")
	if dbURL == "" {
		t.Skip("TEST_DB_URL not set — skipping integration test")
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, dbURL)
	require.NoError(t, err)
	require.NotNil(t, pool)
	defer pool.Close()

	assert.NoError(t, pool.Ping(ctx))
}

func TestRunMigrations_RequiresDBURL(t *testing.T) {
	dbURL := os.Getenv("TEST_DB_URL")
	if dbURL == "" {
		t.Skip("TEST_DB_URL not set — skipping integration test")
	}

	err := db.RunMigrations(dbURL, "../../../../db/migrations")
	assert.NoError(t, err)
}
