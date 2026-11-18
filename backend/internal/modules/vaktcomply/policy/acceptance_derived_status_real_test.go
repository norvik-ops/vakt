// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// UX-17: NIS2 zeigte nach dem Kernweg-Durchlauf neun Maßnahmen „In Bearbeitung",
// die niemand angefasst hatte. Ursache, hier gegen die echte Datenbank belegt:
//
//  1. Eine Mitarbeiterin bestätigt eine Richtlinie. AcceptPolicy legt je
//     Treffer von FindControlsByKeywords(policyAcceptanceKeywords) einen
//     Nachweis „Richtlinien-Akzeptanz: …" ab — A.5.1 zuerst, danach bis zu neun
//     Kontrollen anderer Frameworks, deren Titel „Richtlinie" enthält (gewollt,
//     R1-KW-16: „the words find related controls in the other frameworks").
//  2. Ein einzelner Nachweis ohne manuellen Status ergibt über ResolveStatus
//     den Zustand „partial" — die Zeile zeigt „In Bearbeitung" und zählt halb.
//
// Kein Cross-Mapping zu ISO, sondern ein automatisch abgelegter Nachweis. Die
// Oberfläche kennzeichnet solche Zeilen jetzt („aus 1 Nachweis abgeleitet").
// Der Test hält beide Schritte fest; ändert sich einer, muss die Kennzeichnung
// neu bewertet werden.
func TestPolicyAcceptance_NIS2StatusAbgeleitetAusNachweis(t *testing.T) {
	url := os.Getenv("VAKT_DB_URL")
	if url == "" {
		t.Skip("VAKT_DB_URL not set — dieser Test braucht eine migrierte Postgres (CI setzt sie)")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	var orgID string
	slug := fmt.Sprintf("ux-17-%d", os.Getpid())
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ($1, $1) RETURNING id::text`, slug).Scan(&orgID))
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM ck_evidence WHERE org_id = $1::uuid`, orgID)
		_, _ = pool.Exec(bg, `DELETE FROM ck_controls WHERE org_id = $1::uuid`, orgID)
		_, _ = pool.Exec(bg, `DELETE FROM ck_frameworks WHERE org_id = $1::uuid`, orgID)
		_, _ = pool.Exec(bg, `DELETE FROM organizations WHERE id = $1::uuid`, orgID)
	})

	framework := func(name string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO ck_frameworks (org_id, name) VALUES ($1::uuid, $2) RETURNING id::text`,
			orgID, name).Scan(&id))
		return id
	}
	control := func(fwID, controlID, title string) {
		_, err := pool.Exec(ctx,
			`INSERT INTO ck_controls (framework_id, org_id, control_id, title, domain)
			 VALUES ($1::uuid, $2::uuid, $3, $4, 'Governance')`,
			fwID, orgID, controlID, title)
		require.NoError(t, err)
	}

	iso := framework("ISO27001")
	control(iso, "A.5.1", "Richtlinien zur Informationssicherheit")
	nis2 := framework("NIS2")
	control(nis2, "NIS2-A.1", "Informationssicherheitsrichtlinie")
	control(nis2, "NIS2-B.1", "Incident-Response-Richtlinie")
	control(nis2, "NIS2-B.2", "Erkennung und Überwachung von Vorfällen") // kein Treffer

	repo := NewRepository(pool)
	hits, err := repo.FindControlsByKeywords(ctx, orgID, policyAcceptanceKeywords)
	require.NoError(t, err)
	got := map[string]bool{}
	for _, c := range hits {
		got[c.ControlID] = true
	}
	require.True(t, got["A.5.1"], "A.5.1 bekommt den Nachweis (R1-KW-16)")
	require.True(t, got["NIS2-A.1"] && got["NIS2-B.1"],
		"Schritt 1: die Stichwortsuche trifft NIS2-Kontrollen mit „Richtlinie\" im Titel, got %v", got)
	require.False(t, got["NIS2-B.2"], "ohne Stichwort kein Treffer")

	// Was AcceptPolicy je Treffer tut.
	for _, c := range hits {
		_, err := repo.AddCollectorEvidence(ctx, orgID, c.ID, "", "policy_acceptance",
			"Richtlinien-Akzeptanz: Passwortrichtlinie", []byte(`{"source":"policy_acceptance"}`))
		require.NoError(t, err)
	}

	controls, err := NewService(pool).ListControls(ctx, orgID, nis2)
	require.NoError(t, err)
	byID := map[string]Control{}
	for _, c := range controls {
		byID[c.ControlID] = c
	}
	for _, id := range []string{"NIS2-A.1", "NIS2-B.1"} {
		c := byID[id]
		require.Equal(t, "partial", c.Status, "Schritt 2: %s steht durch den einen Nachweis auf teilweise", id)
		require.Empty(t, c.ManualStatus, "%s: niemand hat den Status gesetzt — er ist abgeleitet", id)
		require.Equal(t, 1, c.EvidenceCount, "%s: genau der Akzeptanz-Nachweis", id)
	}
	require.Equal(t, "missing", byID["NIS2-B.2"].Status)
}
