// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import "github.com/matharnica/vakt/internal/modules/vaktcomply/policy"

// Type aliases re-export the policy-domain types from the policy/ sub-package so
// root handlers and helpers that still reference them unqualified keep
// compiling. ADR-0066 sub-package strategy.
type (
	Framework                = policy.Framework
	EnableFrameworkInput     = policy.EnableFrameworkInput
	SwitchDORAVariantInput   = policy.SwitchDORAVariantInput
	Control                  = policy.Control
	ControlReview            = policy.ControlReview
	RecordReviewInput        = policy.RecordReviewInput
	UpdateControlInput       = policy.UpdateControlInput
	BulkUpdateControlsInput  = policy.BulkUpdateControlsInput
	UpdateSoAMetadataInput   = policy.UpdateSoAMetadataInput
	SoARow                   = policy.SoARow
	SoAEntry                 = policy.SoAEntry
	TISAXControlGap          = policy.TISAXControlGap
	TISAXGapAnalysis         = policy.TISAXGapAnalysis
	TISAXMaturitySummary     = policy.TISAXMaturitySummary
	ChapterMaturity          = policy.ChapterMaturity
	Policy                   = policy.Policy
	PolicyVersion            = policy.PolicyVersion
	CreatePolicyInput        = policy.CreatePolicyInput
	UpdatePolicyInput        = policy.UpdatePolicyInput
	GeneratePolicyDraftInput = policy.GeneratePolicyDraftInput
	ControlMeasure           = policy.ControlMeasure
	CreateMeasureInput       = policy.CreateMeasureInput
	UpdateMeasureInput       = policy.UpdateMeasureInput
	FrameworkMapping         = policy.FrameworkMapping
	ControlMapping           = policy.ControlMapping
	FrameworkPairCountRow    = policy.FrameworkPairCountRow
	ControlPrerequisiteRow   = policy.ControlPrerequisiteRow
	MappingResult            = policy.MappingResult
	ReadinessReport          = policy.ReadinessReport
	DomainScore              = policy.DomainScore
	GapAnalysis              = policy.GapAnalysis
	ControlGap               = policy.ControlGap
	ControlTask              = policy.ControlTask
	CreateControlTaskInput   = policy.CreateControlTaskInput
	UpdateControlTaskInput   = policy.UpdateControlTaskInput
	Evidence                 = policy.Evidence

	// SoA-dedicated types (defined in policy/repository_soa_dedicated.go).
	SoADedicatedEntry   = policy.SoADedicatedEntry
	SoAVersion          = policy.SoAVersion
	SoASummary          = policy.SoASummary
	UpdateSoAEntryInput = policy.UpdateSoAEntryInput

	// Physical-control templates (defined in policy/service_physical_templates.go).
	PhysicalControlTemplate = policy.PhysicalControlTemplate

	// Framework plugins (defined in policy/plugins.go).
	FrameworkPlugin    = policy.FrameworkPlugin
	PluginControl      = policy.PluginControl
	AvailableFramework = policy.AvailableFramework

	// Policy templates (defined in policy/policy_templates_seed.go).
	DBPolicyTemplate = policy.DBPolicyTemplate
	PolicyTemplate   = policy.PolicyTemplate

	// Policy-acceptance types (defined in policy/policy_acceptance.go).
	CreateCampaignInput        = policy.CreateCampaignInput
	PolicyAcceptanceCampaign   = policy.PolicyAcceptanceCampaign
	PolicyAcceptanceRequest    = policy.PolicyAcceptanceRequest
	PolicyAcceptanceSMTPConfig = policy.PolicyAcceptanceSMTPConfig
)

// Re-exported package-level symbols moved into the policy/ sub-package that
// root-staying handlers still reference unqualified.
var (
	// ErrSoANotInitialized signals a dedicated SoA has not been initialised yet.
	ErrSoANotInitialized = policy.ErrSoANotInitialized
	// BuiltinPolicyTemplates returns the built-in policy template catalogue.
	BuiltinPolicyTemplates = policy.BuiltinPolicyTemplates
	// ISO27001AnnexAControls is the embedded ISO 27001:2022 Annex A control template list.
	ISO27001AnnexAControls = policy.ISO27001AnnexAControls
	// ErrExclusionReasonRequired signals a SoA exclusion lacks a justification.
	ErrExclusionReasonRequired = policy.ErrExclusionReasonRequired
)
