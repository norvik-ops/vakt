// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S78-2 regression: the ck_controls LEFT JOIN in BSI check queries must be
// scoped to the current org to prevent cross-org row multiplication on the demo.

package vaktcomply

import (
	"strings"
	"testing"
)

// TestBSICheckQueries_OrgScopedJoin is a source-level guard that verifies the
// SQL for GetCheckSheet and GetBSIGapReport contains "AND c.org_id = cr.org_id"
// on the ck_controls JOIN. This catches regressions without needing a live DB.
func TestBSICheckQueries_OrgScopedJoin(t *testing.T) {
	queries := map[string]string{
		"GetCheckSheet": checkSheetSQL,
		"GetBSIGapReport": gapReportSQL,
	}
	for name, sql := range queries {
		if !strings.Contains(sql, "AND c.org_id = cr.org_id") {
			t.Errorf("%s: ck_controls JOIN lacks org_id scope — cross-org data leak possible", name)
		}
		// Make sure the unscoped pattern is gone.
		unscopedJoin := "LEFT JOIN ck_controls c ON c.control_id = cr.anforderung_id\n"
		if strings.Contains(sql, unscopedJoin) {
			t.Errorf("%s: unscoped ck_controls JOIN found", name)
		}
	}
}
