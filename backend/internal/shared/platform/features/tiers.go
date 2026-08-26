// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package features

// ProTier lists every feature a Pro license unlocks.
//
// This is the single source of truth for what "Pro" means. It used to live
// unexported inside the Polar webhook package — which meant a license issued
// through a different path (CLI, direct sale, manual re-issue) would have needed
// its own copy of the list. Two copies drift; a customer who bought by invoice
// would silently get a different feature set than one who paid by card.
//
// It lives in this package, not in internal/license, because features already
// imports license — the reverse would be an import cycle.
var ProTier = []string{
	FeatureEUAIAct,
	FeatureCRA,
	FeatureAIAdvisor,
	FeatureAuditPDF,
	FeatureSSO,
	FeatureAPI,
	FeatureSecReflex,
	FeatureSecPulse,
	FeatureSecVault,
	FeatureSecPrivacy,
	FeatureBSIGrundschutz,
	FeatureGranularPermissions,
	FeatureSupplierPortal,
	FeatureNIS2Reporting,
	FeatureSAMLAuth,
	FeatureAgentWriteTools,
	FeatureSCIMProvisioning,
	FeatureSIEM,
	// FeatureMultiFramework gates ISO27017, ISO27018, DSGVO-TOM, CIS, KRITIS and
	// C5 (vaktcomply frameworkFeatureGate). It belongs to Pro per S131-G1/V08-D
	// (2026-07-23) and ADR-0021, but was never added here — so every Pro key the
	// sales path signed answered 402 on all six frameworks (R1-17-L01).
	FeatureMultiFramework,
}

// UnsoldFeatures lists every feature that exists in the code, gates real routes,
// and is deliberately in NO sellable tier — nobody can buy it, and that is the
// decision, not an oversight.
//
// It replaces EnterpriseTier, removed on 2026-08-08 together with the Enterprise
// tier itself. The tier was never issuable: both signing paths hard-code "pro"
// (billing/licensing/service.go), the admin CLI has no --tier flag, and the only
// License with Tier=="enterprise" the platform ever produced was the demo one.
// So the 25 routes gated on these three features answered 402 for every paying
// customer, while the price list sold them (R1-17-02).
//
// The three stay unsold rather than becoming Pro. DORA and TISAX were taken out
// of the offering in v0.42.20 and are marked "draft" in the backend catalogue
// (vaktcomply/policy/plugins.go); ISO 42001 joins them because the AI-management
// standard has no sales motion behind it either. Moving them into ProTier would
// be a new product promise, not a bug fix.
//
// This list is not decorative: the coverage test in tiers_test.go fails when a
// new feature belongs to neither ProTier nor here, so "sellable?" stays a
// decision someone has to make out loud instead of a slice nobody updated.
var UnsoldFeatures = []string{
	FeatureTISAX,
	FeatureDORA,
	FeatureISO42001,
}
