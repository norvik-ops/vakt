// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package features

// ProTier lists every feature a Pro license unlocks.
//
// This is the single source of truth for what "Pro" means. It used to live
// unexported inside the Polar webhook package — which meant a license issued
// through any other path (CLI, direct sale, manual re-issue) would have needed
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
}
