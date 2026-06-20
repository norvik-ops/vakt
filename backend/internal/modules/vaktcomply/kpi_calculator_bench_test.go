// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S100-7 / ARCH-L01: Benchmarks for the KPI calculator.
// The nil-DB path exercises the guard logic and struct allocation overhead;
// the numericToFloat64Ptr helper is the inner-loop hot path in all
// float64-returning sub-calculators.
//
// Run with: go test -bench=. -benchmem ./internal/modules/vaktcomply/

package vaktcomply

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

// BenchmarkCalculateKPIsForOrg_NilDB measures the allocation cost of the
// snapshot struct and the nil-guard short-circuits in all sub-calculators.
// This baseline ensures the guard overhead stays sub-microsecond.
func BenchmarkCalculateKPIsForOrg_NilDB(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = CalculateKPIsForOrg(ctx, nil, "org-bench-nil")
	}
}

// BenchmarkNumericToFloat64Ptr_Valid benchmarks the happy path (valid Numeric).
func BenchmarkNumericToFloat64Ptr_Valid(b *testing.B) {
	var n pgtype.Numeric
	_ = n.Scan("98.76")
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = numericToFloat64Ptr(n)
	}
}

// BenchmarkNumericToFloat64Ptr_Invalid benchmarks the nil-return path (NULL Numeric).
func BenchmarkNumericToFloat64Ptr_Invalid(b *testing.B) {
	var n pgtype.Numeric // zero value — Valid = false
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = numericToFloat64Ptr(n)
	}
}
