// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package features

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/license"
)

// R1-17-L01: features.ProTier ist die Feature-Liste, mit der der Verkaufsweg
// jeden Schluessel signiert (billing/licensing/service.go SignUntil). Ein
// Feature, das ein Routen-Gate benutzt, aber in KEINER Tier-Liste steht, ist
// von niemandem kaufbar — die betroffenen Routen antworten jedem Kunden mit
// 402, egal was er bezahlt hat. Genau das war bei vier Features der Fall
// (tisax, dora, iso_42001, multi_framework); sechs Frameworks
// (ISO27017, ISO27018, DSGVO-TOM, CIS, KRITIS, C5) haengen an multi_framework
// und waren dadurch fuer jeden Pro-Kunden gesperrt.
//
// Der Test prueft die Zuordnung listenfrei in beide Richtungen: jedes Feature,
// das es gibt, ist entweder verkaeuflich (ProTier) oder ausdruecklich nicht
// verkaeuflich (UnsoldFeatures) — nie unentschieden. Und keine der beiden
// Listen fuehrt ein Feature, das die Lizenz gar nicht kennt.
//
// Seit dem 2026-08-08 gibt es nur noch EIN verkaufbares Tier. Die zweite Liste
// ist deshalb kein Tier mehr, sondern die Gegenprobe: „nicht kaufbar" muss
// jemand hinschreiben, sonst ist es wieder bloss eine vergessene Zeile.

func TestEveryFeatureIsEitherSellableOrExplicitlyUnsold(t *testing.T) {
	sellable := map[string]bool{}
	for _, f := range ProTier {
		sellable[f] = true
	}
	unsold := map[string]bool{}
	for _, f := range UnsoldFeatures {
		unsold[f] = true
	}

	all := license.AllFeatures()
	require.NotEmpty(t, all, "license.AllFeatures() ist leer — der Test prueft sonst nichts")

	for _, f := range all {
		assert.True(t, sellable[f] || unsold[f],
			"Feature %q steht weder in ProTier noch in UnsoldFeatures: kein Kunde kann es kaufen, jede damit gegatete Route antwortet dauerhaft 402 — und niemand hat das entschieden", f)
		assert.False(t, sellable[f] && unsold[f],
			"Feature %q steht in ProTier UND in UnsoldFeatures — eine der beiden Aussagen ist falsch", f)
	}

	known := map[string]bool{}
	for _, f := range all {
		known[f] = true
	}
	for _, list := range []map[string]bool{sellable, unsold} {
		for f := range list {
			assert.True(t, known[f],
				"Tier-Liste fuehrt %q, aber license kennt das Feature nicht — ein signierter Schluessel traegt dann einen String, den kein Gate prueft", f)
		}
	}

	t.Logf("geprueft: %d Features, ProTier=%d, UnsoldFeatures=%d", len(all), len(ProTier), len(UnsoldFeatures))
}

// TestMultiFrameworkIsPro haelt die Produktentscheidung fest, die der
// Abdeckungstest oben allein nicht festhalten kann: FeatureMultiFramework
// koennte auch nach UnsoldFeatures geschoben werden und die Abdeckung waere
// weiterhin vollstaendig — die sechs Zusatz-Frameworks blieben fuer Pro-Kunden
// trotzdem gesperrt.
//
// Quelle: internal/modules/vaktcomply/handler.go (frameworkFeatureGate,
// S131-G1/V08-D, 2026-07-23) stellt DSGVO-TOM/CIS/KRITIS/C5 ausdruecklich
// "hinter FeatureMultiFramework (Pro)"; ADR-0021 fuehrt den
// Multi-Framework-Wizard ebenfalls als Pro.
func TestMultiFrameworkIsPro(t *testing.T) {
	assert.Contains(t, ProTier, FeatureMultiFramework,
		"multi_framework ist laut Gate-Kommentar und ADR-0021 ein Pro-Feature")

	// Der wertgenaue Nachweis: ein Schluessel, wie ihn der Verkaufsweg signiert.
	lic := &license.License{Tier: "pro", Features: ProTier}
	assert.True(t, lic.Has(FeatureMultiFramework),
		"ein vom Verkaufsweg signierter Pro-Schluessel gewaehrt multi_framework nicht — ISO27017, ISO27018, DSGVO-TOM, CIS, KRITIS und C5 antworten dem zahlenden Kunden mit 402")
}

// TestUnsoldFeaturesAreNotInAnySoldKey ist die Rot-Abnahme fuer die Abschaffung
// des Enterprise-Tiers: kaeme TISAX, DORA oder ISO 42001 durch einen
// Wieder-Einbau in ProTier zurueck, wuerde jeder verkaufte Schluessel drei
// Rahmenwerke freischalten, die niemand verkauft hat — der Fehler des alten
// Zustands, nur in die andere Richtung.
//
// Wertgenau statt listenweise: geprueft wird ein Schluessel, wie ihn der
// Verkaufsweg wirklich signiert (Tier "pro", Features = ProTier).
func TestUnsoldFeaturesAreNotInAnySoldKey(t *testing.T) {
	require.NotEmpty(t, UnsoldFeatures, "UnsoldFeatures ist leer — der Test prueft sonst nichts")

	lic := &license.License{Tier: "pro", Features: ProTier}
	for _, f := range UnsoldFeatures {
		assert.NotContains(t, ProTier, f,
			"nicht verkauftes Feature %q ist nach ProTier gewandert — das ist eine Produktzusage, kein Refactoring", f)
		assert.False(t, lic.Has(f),
			"ein vom Verkaufsweg signierter Pro-Schluessel gewaehrt %q — verkauft wird das Feature aber nicht", f)
	}
}

// TestUnsoldFeaturesCoverTheThreeFrameworks nagelt fest, WELCHE drei es sind.
// Ohne diesen Test bliebe TestUnsoldFeaturesAreNotInAnySoldKey gruen, wenn
// jemand die Liste leert und die Features nach ProTier schiebt: die Schleife
// laeuft dann ueber nichts.
func TestUnsoldFeaturesCoverTheThreeFrameworks(t *testing.T) {
	assert.ElementsMatch(t,
		[]string{license.FeatureTISAX, license.FeatureDORA, license.FeatureISO42001},
		UnsoldFeatures,
		"die Menge der nicht verkauften Rahmenwerke hat sich geaendert — das ist eine Produktentscheidung und gehoert in ein ADR, nicht in einen Diff")
}
