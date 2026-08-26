//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktaware"
)

// TestVaktaware_ImportTargetsCSV_CountsWhatItWrote (L2-04) faehrt exakt die
// Datei aus dem Befund: Kopfzeile, eine gute Adresse, eine kaputte, dieselbe
// gute Adresse noch einmal, eine leere Zeile mit Komma, eine zweite gute
// Adresse.
//
// Gemessen vor dem Fix: {"imported":5,"errors":null} und 4 Zeilen in
// sr_targets, darunter eine leere Adresse und not-an-email. Genau diese beiden
// sind es, die der Mailserver spaeter mit 501 ablehnt.
func TestVaktaware_ImportTargetsCSV_CountsWhatItWrote(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	svc := vaktaware.NewService(pool, vaktaware.SMTPConfig{Host: "smtp.test", Port: "25"})
	repo := vaktaware.NewRepository(pool)
	group, err := repo.CreateTargetGroup(ctx, orgID, "Alle", "manual")
	require.NoError(t, err)

	csv := strings.Join([]string{
		"email,first_name,last_name,department",
		"alice@example.com,Alice,Ant,Sales",
		"not-an-email,Bob,Bee,IT",
		"alice@example.com,Alice,Ant,Sales",
		",Leer,Zeile,IT",
		"carol@example.com,Carol,Cee,HR",
	}, "\n")

	imported, errs := svc.ImportTargetsCSV(ctx, orgID, group.ID, csv)

	targets, err := repo.ListTargets(ctx, orgID, group.ID)
	require.NoError(t, err)

	assert.Equal(t, len(targets), imported,
		"die gemeldete Zahl muss der Zahl der geschriebenen Zielpersonen entsprechen — gemeldet %d, geschrieben %d",
		imported, len(targets))
	assert.Equal(t, 2, imported, "alice und carol; die Dublette fuegt niemanden hinzu")
	assert.Len(t, errs, 3, "kaputte Adresse, Dublette und leere Zeile muessen benannt werden, nicht verschwiegen: %v", errs)

	for _, target := range targets {
		assert.NotEmpty(t, target.Email, "eine leere Adresse darf nicht in der Zielliste stehen")
		assert.NotEqual(t, "not-an-email", target.Email,
			"eine ungueltige Adresse darf nicht in der Zielliste stehen — der Mailserver lehnt sie mit 501 ab")
	}
}

// TestVaktaware_ImportTargetsCSV_HappyPath: der Fix darf den normalen Import
// nicht kaputtmachen — sonst haette er nur die Richtung des Fehlers getauscht.
func TestVaktaware_ImportTargetsCSV_HappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	svc := vaktaware.NewService(pool, vaktaware.SMTPConfig{Host: "smtp.test", Port: "25"})
	repo := vaktaware.NewRepository(pool)
	group, err := repo.CreateTargetGroup(ctx, orgID, "Alle", "manual")
	require.NoError(t, err)

	csv := "email,first_name,last_name,department\n" +
		"ada@kunde.test,Ada,Lovelace,IT\n" +
		"grace@kunde.test,Grace,Hopper,Entwicklung\n"

	imported, errs := svc.ImportTargetsCSV(ctx, orgID, group.ID, csv)
	assert.Equal(t, 2, imported)
	assert.Empty(t, errs)

	targets, err := repo.ListTargets(ctx, orgID, group.ID)
	require.NoError(t, err)
	require.Len(t, targets, 2)

	// Ein zweiter Lauf derselben Datei aktualisiert dieselben Personen und legt
	// niemanden neu an — das ist der Upsert, und die Zahl muss ihn abbilden.
	imported2, _ := svc.ImportTargetsCSV(ctx, orgID, group.ID, csv)
	targets2, err := repo.ListTargets(ctx, orgID, group.ID)
	require.NoError(t, err)
	assert.Len(t, targets2, 2, "der zweite Lauf darf niemanden verdoppeln")
	assert.Equal(t, len(targets2), imported2, "gemeldet %d, in der Gruppe %d", imported2, len(targets2))
}

// TestVaktaware_ImportTargetsCSV_HeaderlessFileIsNotSilent: ohne Kopfzeile
// verschwand die erste Person kommentarlos.
func TestVaktaware_ImportTargetsCSV_HeaderlessFileIsNotSilent(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	svc := vaktaware.NewService(pool, vaktaware.SMTPConfig{Host: "smtp.test", Port: "25"})
	repo := vaktaware.NewRepository(pool)
	group, err := repo.CreateTargetGroup(ctx, orgID, "Alle", "manual")
	require.NoError(t, err)

	imported, errs := svc.ImportTargetsCSV(ctx, orgID, group.ID,
		"ada@kunde.test,Ada,Lovelace,IT\ngrace@kunde.test,Grace,Hopper,IT\n")
	assert.Equal(t, 1, imported)
	require.NotEmpty(t, errs, "die uebersprungene erste Zeile muss benannt werden")
	assert.Contains(t, errs[0], "Kopfzeile")
}
