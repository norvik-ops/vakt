// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package httputil

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/shared/apperr"
)

// RespondError is the single place where every handler's DB-error branch decides
// 4xx vs 5xx (S4 / D13-B). The classifier (apperr.Status) had tests; the
// responder did not — so nothing asserted that a route actually ANSWERS 409/422/
// 404 rather than merely classifying correctly. These tests close that gap by
// checking the real HTTP status and the real response body.

func respond(t *testing.T, err error) (int, map[string]string) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, RespondError(c, err, "internal error", "INTERNAL"))

	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return rec.Code, body
}

func pgErr(code string) error {
	return &pgconn.PgError{Code: code, Message: "db said no"}
}

func TestRespondError_NotFound(t *testing.T) {
	status, body := respond(t, fmt.Errorf("fetch project: %w", apperr.ErrNotFound))
	require.Equal(t, http.StatusNotFound, status)
	require.Equal(t, "NOT_FOUND", body["code"])
	require.Equal(t, "resource not found", body["error"])
}

// pgx.ErrNoRows is what a QueryRow scan returns for a missing row — the most
// common not-found source in this codebase.
func TestRespondError_NoRowsIsNotFound(t *testing.T) {
	status, body := respond(t, fmt.Errorf("get supplier: %w", pgx.ErrNoRows))
	require.Equal(t, http.StatusNotFound, status)
	require.Equal(t, "NOT_FOUND", body["code"])
}

// 22P02 = invalid_text_representation, e.g. a malformed UUID reaching a ::uuid
// cast. Client input, so 400 — never 500.
func TestRespondError_MalformedInputIs400(t *testing.T) {
	status, body := respond(t, pgErr("22P02"))
	require.Equal(t, http.StatusBadRequest, status)
	require.Equal(t, "BAD_REQUEST", body["code"])
}

func TestRespondError_UniqueViolationIs409(t *testing.T) {
	status, body := respond(t, pgErr("23505"))
	require.Equal(t, http.StatusConflict, status)
	require.Equal(t, "CONFLICT", body["code"])
}

func TestRespondError_CheckAndNotNullAre422(t *testing.T) {
	for _, code := range []string{"23514", "23502"} {
		status, body := respond(t, pgErr(code))
		require.Equal(t, http.StatusUnprocessableEntity, status, "SQLSTATE %s", code)
		require.Equal(t, "UNPROCESSABLE", body["code"], "SQLSTATE %s", code)
	}
}

// 23503 is raised both for "you referenced something that does not exist"
// (INSERT) and "you cannot delete this, children still point at it" (DELETE).
// The DELETE case has no request body, so "input violates a data rule" would be
// nonsense — it gets its own 409 with a reference-specific message.
func TestRespondError_ForeignKeyIsReferenceConflict(t *testing.T) {
	status, body := respond(t, pgErr("23503"))
	require.Equal(t, http.StatusConflict, status)
	require.Equal(t, "REFERENCE_CONFLICT", body["code"])
	require.Contains(t, body["error"], "referenced")
	require.NotContains(t, body["error"], "input",
		"a DELETE blocked by children has no input — the message must not claim otherwise")
}

// The critical negative: a genuine schema bug ("column does not exist", 42703)
// must stay a 500. Mapping it to 404 would hide a broken deployment behind a
// response that looks like ordinary missing data — the reason apperr uses
// HasSuffix rather than Contains for its "not found" fallback.
func TestRespondError_SchemaBugStays500(t *testing.T) {
	status, body := respond(t, pgErr("42703"))
	require.Equal(t, http.StatusInternalServerError, status)
	require.Equal(t, "INTERNAL", body["code"], "the caller's fallback code must be used for 5xx")
	require.Equal(t, "internal error", body["error"])
}

// An unclassified error keeps the caller's fallback — the responder must not
// invent a 4xx for something it does not recognise.
func TestRespondError_UnknownErrorUsesFallback(t *testing.T) {
	status, body := respond(t, errors.New("something exploded"))
	require.Equal(t, http.StatusInternalServerError, status)
	require.Equal(t, "INTERNAL", body["code"])
}

// The raw error must never reach the client (CLAUDE.md: no internal error
// details, no stack traces, no DB messages over the wire).
func TestRespondError_NeverLeaksRawError(t *testing.T) {
	secret := "pq: relation \"internal_secrets\" does not exist at character 42"
	_, body := respond(t, errors.New(secret))
	for _, v := range body {
		require.NotContains(t, v, "internal_secrets",
			"the raw DB error must never be echoed to the client")
		require.NotContains(t, v, "character 42")
	}
}
