// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import "github.com/matharnica/vakt/internal/modules/vaktcomply/reporting"

// KPI types — defined in reporting sub-package, aliased here for API compatibility.
type KPISnapshot = reporting.KPISnapshot
type KPIDashboard = reporting.KPIDashboard

// NIS2 types — defined in reporting sub-package, aliased here for API compatibility.
type NIS2ReportabilityCheck = reporting.NIS2ReportabilityCheck
type NIS2ReportInput = reporting.NIS2ReportInput
type NIS2ReportStatus = reporting.NIS2ReportStatus
type NIS2Deadlines = reporting.NIS2Deadlines
type NIS2StageReport = reporting.NIS2StageReport
type AuthorityContact = reporting.AuthorityContact
