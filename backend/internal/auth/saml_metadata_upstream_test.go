// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/config"
)

// R1-07-B04 — GET /auth/saml/metadata answered a Casdoor JSON error under
// Content-Type: application/xml with HTTP 200.
//
// Casdoor answers its controller envelope with HTTP 200 even when the SAML app
// id is unknown: {"status":"error","msg":"..."}. The handler only checked the
// status code and then blobbed the body out as application/xml. A SAML IdP
// consuming that endpoint gets broken XML under a success status — it has no
// way to tell that the federation was never configured.
func TestSAMLMetadata_upstreamJSONErrorIsNotServedAsXML(t *testing.T) {
	casdoor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK) // Casdoor's shape: 200 with an error body
		_, _ = w.Write([]byte(`{"status":"error","msg":"the application: vakt does not exist","data":null}`))
	}))
	defer casdoor.Close()

	rec := serveSAMLMetadata(t, casdoor.URL)

	assert.Equal(t, http.StatusBadGateway, rec.Code,
		"a Casdoor error body must not be handed to an IdP as successful SAML metadata")
	assert.NotContains(t, rec.Header().Get("Content-Type"), "xml",
		"the response is JSON — it must not claim to be XML")
}

// The success path has to stay intact: real metadata is passed through as-is.
func TestSAMLMetadata_upstreamMetadataIsServed(t *testing.T) {
	const meta = `<?xml version="1.0"?>
<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://casdoor.example.test/vakt"/>`

	casdoor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(meta))
	}))
	defer casdoor.Close()

	rec := serveSAMLMetadata(t, casdoor.URL)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/xml")
	assert.Equal(t, meta, rec.Body.String())
}

// serveSAMLMetadata drives the Casdoor fallback branch of the metadata handler.
// No DB is attached, so the direct SP path resolves no organisation and falls
// through to the proxy — which is exactly the branch under test.
func serveSAMLMetadata(t *testing.T, casdoorURL string) *httptest.ResponseRecorder {
	t.Helper()
	h := auth.NewHandler(nil, &config.Config{
		CasdoorURL:      casdoorURL,
		CasdoorClientID: "vakt",
	})
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/api/v1/auth/saml/metadata", nil), rec)
	require.NoError(t, h.SAMLDirectMetadata(c))
	return rec
}
