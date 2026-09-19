// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// R1-W7C-N2 (a): CountPendingApprovals must enforce the same admin gate as
// ListPendingApprovals. Before the fix a non-admin could read the pending-
// approval count via GET /vaktcomply/approvals/count.
//
// This test exercises the gate branch without a DB: isOrgAdmin short-circuits to
// (false, nil) when the request carries no authenticated user/org, so the
// handler must answer 403 before it ever reaches the count service — exactly the
// branch a DB-backed Viewer would also hit. The role-lookup path
// (GetOrgMemberRole → non-Admin → 403) is DB-coupled and marked Live-REVERIFY.
package vaktcomply

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestCountPendingApprovalsRequiresAdmin(t *testing.T) {
	e := echo.New()
	h := &Handler{} // zero-value: the admin gate rejects before any service call

	newReq := func() (echo.Context, *httptest.ResponseRecorder) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		// No user_id / org_id set → non-admin.
		return e.NewContext(req, rec), rec
	}

	// Count endpoint: must be forbidden for a non-admin.
	cCount, recCount := newReq()
	assert.NoError(t, h.CountPendingApprovals(cCount))
	assert.Equal(t, http.StatusForbidden, recCount.Code,
		"non-admin must not read the pending-approval count")

	// Parity check: the list endpoint already forbids the same caller. Count must
	// now match it — this is the invariant the fix restores.
	cList, recList := newReq()
	assert.NoError(t, h.ListPendingApprovals(cList))
	assert.Equal(t, recList.Code, recCount.Code,
		"CountPendingApprovals must mirror ListPendingApprovals' admin gate")
}
