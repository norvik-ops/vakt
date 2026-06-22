// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// KPI calculator logic lives in reporting/kpi_calculator.go.
// This file provides root-package shims so that existing tests in package
// vaktcomply continue to compile without modification.

package vaktcomply

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/matharnica/vakt/internal/modules/vaktcomply/reporting"
)

// CalculateKPIsForOrg delegates to the reporting sub-package.
func CalculateKPIsForOrg(ctx context.Context, db *pgxpool.Pool, orgID string) KPISnapshot {
	return reporting.CalculateKPIsForOrg(ctx, db, orgID)
}

// numericToFloat64Ptr is a package-level shim for tests in this package.
func numericToFloat64Ptr(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}
	v := f.Float64
	return &v
}

// calcFindingSLACompliance is a package-level shim for tests in this package.
func calcFindingSLACompliance(ctx context.Context, db *pgxpool.Pool, orgID string) *float64 {
	snap := reporting.CalculateKPIsForOrg(ctx, db, orgID)
	return snap.FindingSLACompliance
}

// calcOpenMajorNCs is a package-level shim for tests in this package.
func calcOpenMajorNCs(ctx context.Context, db *pgxpool.Pool, orgID string) *int {
	snap := reporting.CalculateKPIsForOrg(ctx, db, orgID)
	return snap.OpenMajorNCs
}
