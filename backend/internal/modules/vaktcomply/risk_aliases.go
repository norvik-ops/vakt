// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import "github.com/matharnica/vakt/internal/modules/vaktcomply/risk"

// Type aliases re-export the risk-domain types from the risk/ sub-package so
// root handlers and helpers that still reference them unqualified keep
// compiling. ADR-0066 sub-package strategy.
type (
	Risk                        = risk.Risk
	CreateRiskInput             = risk.CreateRiskInput
	UpdateRiskInput             = risk.UpdateRiskInput
	UpdateRiskTreatmentInput    = risk.UpdateRiskTreatmentInput
	UpdateRiskResidualInput     = risk.UpdateRiskResidualInput
	AcceptRiskInput             = risk.AcceptRiskInput
	DORAThirdParty              = risk.DORAThirdParty
	CreateDORAThirdPartyInput   = risk.CreateDORAThirdPartyInput
	UpdateDORAThirdPartyInput   = risk.UpdateDORAThirdPartyInput
	ProtectionNeedAssessment    = risk.ProtectionNeedAssessment
	CreateProtectionNeedInput   = risk.CreateProtectionNeedInput
	UpdateProtectionNeedInput   = risk.UpdateProtectionNeedInput
	CAPA                        = risk.CAPA
	CreateCAPAInput             = risk.CreateCAPAInput
	UpdateCAPAInput             = risk.UpdateCAPAInput
	BulkUpdateCAPAsInput        = risk.BulkUpdateCAPAsInput
	CAPANCFields                = risk.CAPANCFields
	EffectivenessCheckInput     = risk.EffectivenessCheckInput
	ControlException            = risk.ControlException
	CreateControlExceptionInput = risk.CreateControlExceptionInput
	UpdateControlExceptionInput = risk.UpdateControlExceptionInput
)

// CalculateOverallProtectionNeed re-exports the risk-domain helper so existing
// root callers/tests keep working.
var CalculateOverallProtectionNeed = risk.CalculateOverallProtectionNeed
