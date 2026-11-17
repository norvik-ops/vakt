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
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"gopkg.in/yaml.v3"

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/shared/apidocs"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestEvidenceWireContract pins the evidence contract of the control page
// (Kernweg 9, UX-Durchgang): handler ↔ openapi.yaml ↔ frontend.
//
// Before: the evidence table read `type` and `pending_review`, the API sends
// `source` and `pending` — "Typ" was empty and "Status" an empty badge for
// every row. The add dialog sent `type`/`notes`, which the binder drops, so
// AddEvidenceInput.Source stayed empty and every add without a file was 422.
//
// The test drives the real handlers with the bodies the frontend sends now
// (useControls.ts AddEvidencePayload) and checks the list response against the
// CKEvidence schema in openapi.yaml: every key documented, every required key
// present, status inside the enum, no `type` field.
func TestEvidenceWireContract(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	pgC, err := postgres.Run(ctx,
		imagePostgres,
		postgres.WithDatabase("vakt_test"),
		postgres.WithUsername("vakt"),
		postgres.WithPassword("vakt"),
		postgres.WithSQLDriver("pgx"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		if strings.Contains(err.Error(), "permission denied") ||
			strings.Contains(err.Error(), "Cannot connect to the Docker daemon") {
			t.Skipf("integration: Docker unavailable (%v)", err)
		}
		t.Fatalf("postgres container: %v", err)
	}
	defer func() { _ = pgC.Terminate(ctx) }()

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, shareddb.RunMigrations(dsn, migrationsDir(t)))

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	var orgID, userID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ('EvidenceOrg', 'evidenceorg')
		RETURNING id::text`).Scan(&orgID))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, display_name) VALUES ('editor@evidenceorg.test', 'Editor')
		RETURNING id::text`).Scan(&userID))
	fwID := seedFramework(ctx, t, pool, orgID, "ISO27001")
	ctrlID := seedControl(ctx, t, pool, orgID, fwID, "A.5.2", "5", "")

	h := vaktcomply.NewHandler(vaktcomply.NewService(pool))
	e := echo.New()

	call := func(t *testing.T, handler echo.HandlerFunc, method, body string) (int, []byte) {
		t.Helper()
		req := httptest.NewRequest(method, "/", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("org_id", orgID)
		c.Set("user_id", userID)
		c.SetParamNames("id")
		c.SetParamValues(ctrlID)
		require.NoError(t, handler(c))
		return rec.Code, rec.Body.Bytes()
	}

	// The body the add dialog sends now.
	code, body := call(t, h.AddEvidence, http.MethodPost,
		`{"title":"Protokoll Leitungsrunde","source":"manual","description":"Freigabe der Rollen"}`)
	require.Equal(t, http.StatusCreated, code, string(body))

	// The body it sent before: rejected, because `source` is required.
	code, body = call(t, h.AddEvidence, http.MethodPost,
		`{"title":"alt","type":"manual","notes":"x"}`)
	assert.Equal(t, http.StatusUnprocessableEntity, code, string(body))

	code, body = call(t, h.ListEvidence, http.MethodGet, "")
	require.Equal(t, http.StatusOK, code, string(body))
	var items []map[string]any
	require.NoError(t, json.Unmarshal(body, &items))
	require.Len(t, items, 1)
	item := items[0]
	assert.Equal(t, "manual", item["source"])
	assert.Equal(t, "pending", item["status"], "der Status eines neuen Nachweises heißt pending, nicht pending_review")
	assert.Equal(t, "Freigabe der Rollen", item["description"])
	assert.NotContains(t, item, "type", "die API kennt kein Feld type — die Tabelle leitet den Typ aus source ab")

	// Response ↔ openapi.yaml (components.schemas.CKEvidence).
	raw, err := apidocs.SpecBytes()
	require.NoError(t, err)
	var spec struct {
		Components struct {
			Schemas map[string]struct {
				Required   []string `yaml:"required"`
				Properties map[string]struct {
					Enum []string `yaml:"enum"`
				} `yaml:"properties"`
			} `yaml:"schemas"`
		} `yaml:"components"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &spec))
	schema, ok := spec.Components.Schemas["CKEvidence"]
	require.True(t, ok, "openapi.yaml muss components.schemas.CKEvidence führen")
	for key := range item {
		assert.Contains(t, schema.Properties, key, "Antwortfeld %q fehlt in openapi.yaml CKEvidence", key)
	}
	for _, key := range schema.Required {
		assert.Contains(t, item, key, "openapi.yaml nennt %q als required, die Antwort liefert es nicht", key)
	}
	assert.Contains(t, schema.Properties["status"].Enum, item["status"])
}
