// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktcomply/policy"
	"github.com/matharnica/vakt/internal/shared/platform/events"
)

// catalogueFrameworks ist genau die Konstellation, in der R1-19-W06 live
// gemessen wurde: BSI (245 Controls) plus ISO 27001 (93) = 338, beide Kataloge
// durchgehend deutsch. Sie ist bewusst gewaehlt und nicht die groesstmoegliche
// Auswahl — NIS2 traegt in zwei Titeln englische Lehnwoerter
// ("Identity- und Access-Management (IAM)", "Privileged Access Management"),
// an denen sich das englische Stichwort "access" festhaelt. Mit NIS2 im Korb
// waere der Test fuer vaktvault gruen, obwohl der Defekt danebensteht: gemessen,
// die alte rein englische Liste trifft mit NIS2 zwei Controls, ohne NIS2 null.
var catalogueFrameworks = []string{"BSI", "ISO27001"}

// matchingControls bildet nach, was FindCKControlsByKeywords in SQL tut:
// lower(title) ILIKE ANY(patterns) OR lower(domain) ILIKE ANY(patterns),
// mit %-umschlossenen, kleingeschriebenen Stichworten.
func matchingControls(keywords []string, controls []policy.Control) []string {
	var hits []string
	for _, c := range controls {
		title := strings.ToLower(c.Title)
		domain := strings.ToLower(c.Domain)
		for _, kw := range keywords {
			k := strings.ToLower(kw)
			if strings.Contains(title, k) || strings.Contains(domain, k) {
				hits = append(hits, c.ControlID)
				break
			}
		}
	}
	return hits
}

func loadCatalogue(t *testing.T) []policy.Control {
	t.Helper()
	var all []policy.Control
	for _, fw := range catalogueFrameworks {
		all = append(all, policy.BuiltinControls("fw", "org", fw, "full")...)
	}
	require.Equal(t, 338, len(all),
		"Katalog hat nicht mehr die live gemessene Groesse — dann misst der Test "+
			"eine andere Konstellation als die, in der R1-19-W06 auftrat")
	return all
}

// TestCrossEvidenceKeywordsMatchGermanCatalogue pinnt R1-19-W06.
//
// crossevidence sucht die passenden Controls ueber Stichworte gegen
// ck_controls.title und .domain. Der ausgelieferte Katalog ist deutsch, der
// Eintrag fuer vaktvault war rein englisch ("access", "password", "secret",
// "rotation", "credential") und traf damit keinen einzigen von 338 Controls:
// die Secret-Rotation erzeugte nie Evidenz, der Task meldete trotzdem
// "completed".
//
// Der Test rechnet dieselbe Bedingung nach, die die SQL-Query stellt, gegen den
// echten eingebauten Katalog. Setzt man sourceKeywords[vaktvault] auf die alte,
// rein englische Liste zurueck, faellt der Untertest "vaktvault" mit 0 Treffern.
func TestCrossEvidenceKeywordsMatchGermanCatalogue(t *testing.T) {
	catalogue := loadCatalogue(t)

	// Quellen mit einem echten Produzenten im Baum. Fuer sie ist ein
	// Null-Treffer ein aktiver Defekt.
	live := []string{
		events.SourceSecvault,   // vaktvault: events.SecretRotated
		events.SourceSecreflex,  // vaktaware: events.TrainingCompleted
		events.SourceSecprivacy, // vaktprivacy: events.DSRCompleted
		events.SourceSecpulse,   // vaktscan: events.CertExpiring
	}
	for _, src := range live {
		t.Run(src, func(t *testing.T) {
			kws := sourceKeywords[src]
			require.NotEmpty(t, kws, "Quelle %q hat keine Stichworte", src)
			hits := matchingControls(kws, catalogue)
			assert.NotEmpty(t, hits,
				"Quelle %q trifft keinen einzigen Control des deutschen Katalogs — "+
					"der Task meldete dann completed, ohne Evidenz zu erzeugen", src)
		})
	}

	// Quellen ohne Produzenten: kein aktiver Defekt, aber sie duerfen nicht
	// unbesetzt bleiben, sonst laeuft der erste Produzent wieder ins Leere.
	for _, src := range []string{events.SourceHR, events.SourceSecvitals} {
		t.Run(src+"_vorbereitet", func(t *testing.T) {
			kws := sourceKeywords[src]
			require.NotEmpty(t, kws, "Quelle %q fehlt in sourceKeywords", src)
			assert.NotEmpty(t, matchingControls(kws, catalogue),
				"Quelle %q ist eingetragen, trifft aber nichts", src)
		})
	}

	// Gegenprobe: die Nachbildung der SQL-Bedingung muss unterscheiden koennen.
	// Ein Stichwort, das im Katalog nicht vorkommt, darf nichts treffen —
	// sonst waeren alle Zusicherungen oben vakuos.
	assert.Empty(t, matchingControls([]string{"zzz-kein-control-heisst-so"}, catalogue),
		"die Nachbildung trifft alles — dann beweist der Test nichts")
}
