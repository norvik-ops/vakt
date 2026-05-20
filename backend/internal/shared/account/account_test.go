// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package account

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// TestDeleteAccount_ConfirmRequired verifies that the explicit "LÖSCHEN"
// confirmation string is mandatory — a misclicked button or stale request
// body must not silently anonymise an account.
func TestDeleteAccount_ConfirmRequired(t *testing.T) {
	h := &Handler{svc: nil}
	e := echo.New()

	req := httptest.NewRequest("POST", "/account/delete",
		strings.NewReader(`{"password":"hunter2","confirm":"yes"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", "550e8400-e29b-41d4-a716-446655440000")

	if err := h.DeleteAccount(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	assert.Equal(t, 422, rec.Code, "wrong confirmation should yield 422")
	assert.Contains(t, rec.Body.String(), "ACCOUNT_CONFIRM_REQUIRED")
}

// TestDeleteAccount_RequiresAuth ensures unauthenticated requests are rejected
// before any DB work happens.
func TestDeleteAccount_RequiresAuth(t *testing.T) {
	h := &Handler{svc: nil}
	e := echo.New()

	req := httptest.NewRequest("POST", "/account/delete",
		strings.NewReader(`{"password":"x","confirm":"LÖSCHEN"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	// No user_id set in context — simulates an unauthenticated call.

	if err := h.DeleteAccount(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	assert.Equal(t, 401, rec.Code)
}

// TestExportData_RequiresAuth — same guard for the export endpoint.
func TestExportData_RequiresAuth(t *testing.T) {
	h := &Handler{svc: nil}
	e := echo.New()

	req := httptest.NewRequest("GET", "/account/data-export", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.ExportData(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	assert.Equal(t, 401, rec.Code)
}
