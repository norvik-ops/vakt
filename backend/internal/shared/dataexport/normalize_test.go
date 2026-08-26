// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package dataexport

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sampleUUID is the raw 16-byte form pgx returns for a `uuid` column, plus its
// canonical text form. The byte array is what json.Marshal used to emit as
// [238,190,84,224,...] — see R1-20-08.
var (
	sampleUUIDBytes = [16]byte{0xee, 0xbe, 0x54, 0xe0, 0x11, 0x11, 0x22, 0x22, 0x33, 0x33, 0x44, 0x44, 0x55, 0x55, 0x66, 0x66}
	sampleUUIDText  = "eebe54e0-1111-2222-3333-444455556666"
)

func TestNormalizeValue_ByColumnType(t *testing.T) {
	berlin := time.FixedZone("CEST", 2*60*60)

	cases := []struct {
		name string
		oid  uint32
		in   any
		want any
	}{
		{"uuid becomes a resolvable string", pgtype.UUIDOID, sampleUUIDBytes, sampleUUIDText},
		{"null uuid stays null", pgtype.UUIDOID, nil, nil},
		{"bytea becomes a PostgreSQL hex literal", pgtype.ByteaOID, []byte{0x00, 0xff, 0x41}, `\x00ff41`},
		{"empty bytea", pgtype.ByteaOID, []byte{}, `\x`},
		{"date drops the fake midnight time", pgtype.DateOID,
			time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC), "2026-08-07"},
		{"timestamptz is normalised to UTC", pgtype.TimestamptzOID,
			time.Date(2026, 8, 7, 6, 35, 13, 0, berlin), "2026-08-07T04:35:13Z"},
		{"timestamp is normalised to UTC", pgtype.TimestampOID,
			time.Date(2026, 8, 7, 4, 35, 13, 0, time.UTC), "2026-08-07T04:35:13Z"},
		{"text passes through", pgtype.TextOID, "alice@acme.test", "alice@acme.test"},
		{"bool passes through", pgtype.BoolOID, true, true},
		{"int4 passes through", pgtype.Int4OID, int32(42), int32(42)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, normalizeValue(tc.oid, tc.in))
		})
	}
}

// TestNormalizeValue_NestedContainers covers the values where no per-element
// OID exists: arrays arrive as []any and jsonb as map[string]any, so a UUID
// hiding inside one of them can only be caught by Go type.
func TestNormalizeValue_NestedContainers(t *testing.T) {
	t.Run("uuid inside an array", func(t *testing.T) {
		got := normalizeValue(pgtype.UUIDArrayOID, []any{sampleUUIDBytes, sampleUUIDBytes})
		assert.Equal(t, []any{sampleUUIDText, sampleUUIDText}, got)
	})

	t.Run("text array untouched", func(t *testing.T) {
		got := normalizeValue(pgtype.TextArrayOID, []any{"a", "b"})
		assert.Equal(t, []any{"a", "b"}, got)
	})

	t.Run("jsonb object recurses", func(t *testing.T) {
		got := normalizeValue(pgtype.JSONBOID, map[string]any{
			"nested": map[string]any{"when": time.Date(2026, 8, 7, 4, 0, 0, 0, time.UTC)},
			"count":  float64(3),
		})
		assert.Equal(t, map[string]any{
			"nested": map[string]any{"when": "2026-08-07T04:00:00Z"},
			"count":  float64(3),
		}, got)
	})
}

// TestNormalizeValue_NoByteArraysReachJSON is the regression guard in its
// bluntest form: whatever the column type, the serialised archive must never
// contain a UUID rendered as a list of numbers.
func TestNormalizeValue_NoByteArraysReachJSON(t *testing.T) {
	row := map[string]any{
		"id":           normalizeValue(pgtype.UUIDOID, sampleUUIDBytes),
		"framework_id": normalizeValue(pgtype.UUIDOID, sampleUUIDBytes),
		"tags":         normalizeValue(pgtype.UUIDArrayOID, []any{sampleUUIDBytes}),
	}
	b, err := json.Marshal(row)
	require.NoError(t, err)
	assert.NotContains(t, string(b), "238,190", "uuid must not be serialised as a byte array")
	assert.Contains(t, string(b), sampleUUIDText)
}

// TestExportedTables_CoversEveryArchiveFile keeps the table list the
// schema-guard test walks in step with the entries the export actually writes.
func TestExportedTables_CoversEveryArchiveFile(t *testing.T) {
	tables := ExportedTables()
	assert.Len(t, tables, 22, "22 tables are exported; update the docs when this changes")
	assert.Contains(t, tables, "ck_controls")
	assert.Contains(t, tables, "hr_employees")
	assert.Contains(t, tables, "sr_events")
	assert.Contains(t, tables, "audit_log")

	seen := map[string]bool{}
	for _, tbl := range tables {
		assert.False(t, seen[tbl], "table listed twice: %s", tbl)
		seen[tbl] = true
	}
}
