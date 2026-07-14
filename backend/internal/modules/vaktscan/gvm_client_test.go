// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Der GVM-Client (OpenVAS) sprach bis hierher mit niemandem außer einem echten
// Greenbone-Server — und war damit von keinem Test erreichbar. Ein httptest-Server
// spricht dasselbe Protokoll und kostet nichts.
//
// Was hier wirklich geprüft wird: dass die Basic-Auth mitgeht (ohne sie antwortet
// GVM 401 und der Scan endet ohne Funde — wieder ein „sauberes System", das keines
// ist), dass ein HTTP-Fehler ein Fehler bleibt und nicht als leeres Ergebnis
// durchrutscht, und dass die Antwort in unsere Strukturen passt.

func gvmTestClient(t *testing.T, h http.Handler) (*gvmClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &gvmClient{
		baseURL: srv.URL,
		user:    "admin",
		pass:    "geheim",
		http:    srv.Client(),
	}, srv
}

func TestGVMClient_CreateTask_SchicktBasicAuth(t *testing.T) {
	var sawUser, sawPass string
	var sawOK bool
	var sawPath, sawMethod string

	c, _ := gvmTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawUser, sawPass, sawOK = r.BasicAuth()
		sawPath, sawMethod = r.URL.Path, r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"task-42","status":"New"}`))
	}))

	id, err := c.createTask(context.Background(), "example.test")
	require.NoError(t, err)
	assert.Equal(t, "task-42", id)

	require.True(t, sawOK, "ohne Basic-Auth antwortet GVM mit 401 — der Scan endete dann ohne Funde, was aussieht wie ein sauberes System")
	assert.Equal(t, "admin", sawUser)
	assert.Equal(t, "geheim", sawPass)
	assert.Equal(t, http.MethodPost, sawMethod)
	assert.Equal(t, "/gvm/tasks", sawPath)
}

func TestGVMClient_HTTPFehlerBleibtFehler(t *testing.T) {
	c, _ := gvmTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"bad credentials"}`))
	}))

	_, err := c.createTask(context.Background(), "example.test")
	require.Error(t, err, "ein 401 darf nicht als leeres Ergebnis durchgehen — sonst meldet ein falsch konfigurierter Scanner ein sauberes System")
	assert.Contains(t, err.Error(), "401")
}

func TestGVMClient_FetchResults(t *testing.T) {
	c, _ := gvmTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "task-42", r.URL.Query().Get("task_id"),
			"ohne task_id liefert GVM die Funde ALLER Scans — das wäre eine Vermischung fremder Ergebnisse")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":"r1","name":"OpenSSL veraltet","description":"…","severity":9.8,
			 "nvt":{"cvss_base":"9.8","oid":"1.3.6.1.4.1.25623.1.0.1"}},
			{"id":"r2","name":"Schwache TLS-Suite","description":"…","severity":4.3,
			 "nvt":{"cvss_base":"4.3","oid":"1.3.6.1.4.1.25623.1.0.2"}}
		]`))
	}))

	results, err := c.fetchResults(context.Background(), "task-42")
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "OpenSSL veraltet", results[0].Name)
	assert.InDelta(t, 9.8, results[0].Severity, 0.001)
	assert.Equal(t, "1.3.6.1.4.1.25623.1.0.1", results[0].NVT.OID)
}

func TestGVMClient_PollTask_BrichtBeiStoppedAb(t *testing.T) {
	// pollTask wartet 10 s zwischen den Abfragen — der Test braucht deshalb einen
	// Kontext, der vorher abläuft, um nicht selbst zu warten. Geprüft wird, dass
	// pollTask den Kontext RESPEKTIERT: Ein abgebrochener Scan darf nicht zehn
	// Minuten lang einen Worker-Slot blockieren.
	c, _ := gvmTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"task-42","status":"Running"}`))
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := c.pollTask(ctx, "task-42")
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 2*time.Second,
		"pollTask muss den Kontext beachten und nicht bis zum eigenen 10-Minuten-Deadline weiterlaufen")
}

func TestNewGVMClient_OhneURLIstNichtKonfiguriert(t *testing.T) {
	t.Setenv("VAKT_OPENVAS_URL", "")

	_, err := newGVMClient()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotConfigured,
		"ein nicht konfigurierter Scanner muss sich als solcher melden — ein stiller Erfolg wäre ein Scan, den es nie gab")
}
