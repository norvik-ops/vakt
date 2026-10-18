// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package assets

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── ImportAssetsCSV — header validation (no DB needed) ───────────────────────

func TestImportAssetsCSV_EmptyFile(t *testing.T) {
	svc := &Service{}
	_, _, _, err := svc.ImportAssetsCSV(context.Background(), "org1", strings.NewReader(""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read CSV header")
}

func TestImportAssetsCSV_MissingNameColumn(t *testing.T) {
	svc := &Service{}
	csv := "type,criticality\nserver,high\n"
	_, _, _, err := svc.ImportAssetsCSV(context.Background(), "org1", strings.NewReader(csv))
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"name"`)
}

func TestImportAssetsCSV_MissingTypeColumn(t *testing.T) {
	svc := &Service{}
	csv := "name,criticality\nmy-server,high\n"
	_, _, _, err := svc.ImportAssetsCSV(context.Background(), "org1", strings.NewReader(csv))
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"type"`)
}

func TestImportAssetsCSV_HeaderOnlyNoDataRows(t *testing.T) {
	svc := &Service{}
	csv := "name,type,criticality\n"
	_, _, _, err := svc.ImportAssetsCSV(context.Background(), "org1", strings.NewReader(csv))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no data rows")
}

func TestImportAssetsCSV_ColumnOrderFlexible(t *testing.T) {
	// Header column order may vary — the parser must handle reordering.
	// This returns "no data rows" because we pass no actual rows,
	// proving the column-index map was built without error.
	svc := &Service{}
	csv := "criticality,name,type\n"
	_, _, _, err := svc.ImportAssetsCSV(context.Background(), "org1", strings.NewReader(csv))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no data rows") // NOT "missing required column"
}

// ── ImportAssetsCSVExtended — header validation (no DB needed) ───────────────

func TestImportAssetsCSVExtended_MissingNameColumn(t *testing.T) {
	svc := &Service{}
	_, _, _, err := svc.ImportAssetsCSVExtended(context.Background(), "org1", strings.NewReader("type,ip\nserver,10.0.0.1\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"name"`)
}

func TestImportAssetsCSVExtended_EmptyNamesAreSkippedWithoutDB(t *testing.T) {
	// Every row is rejected before the repository is touched, so a zero Service works.
	svc := &Service{}
	imported, skipped, errs, err := svc.ImportAssetsCSVExtended(context.Background(), "org1", strings.NewReader("name,type\n,server\n ,repo\n"))
	require.NoError(t, err)
	assert.Equal(t, 0, imported)
	assert.Equal(t, 2, skipped)
	assert.Len(t, errs, 2)
}
