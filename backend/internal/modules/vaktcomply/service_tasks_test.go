// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Eine Aufgabenliste je Kontrolle (Migration 273): Die frühere Checkliste
// ck_control_tasks ist in ck_tasks aufgegangen. Diese Tests halten die zwei
// Regeln fest, die das Zusammenlegen braucht.

// requireTaskEntity: Aufgaben an einer fremden Kontrolle sind „nicht gefunden“
// (404), nicht eine leere Liste und kein stilles Anlegen.
func TestRequireTaskEntity(t *testing.T) {
	ctx := context.Background()

	t.Run("eigene Kontrolle ist erlaubt", func(t *testing.T) {
		var gotOrg, gotControl string
		err := requireTaskEntity(ctx, "org-a", "control", "ctl-1",
			func(_ context.Context, orgID, controlID string) (bool, error) {
				gotOrg, gotControl = orgID, controlID
				return true, nil
			})
		require.NoError(t, err)
		assert.Equal(t, "org-a", gotOrg, "die Prüfung muss mit der Org des Aufrufers laufen")
		assert.Equal(t, "ctl-1", gotControl)
	})

	t.Run("fremde oder unbekannte Kontrolle ist ErrNotFound", func(t *testing.T) {
		err := requireTaskEntity(ctx, "org-a", "control", "ctl-fremd",
			func(context.Context, string, string) (bool, error) { return false, nil })
		require.Error(t, err)
		assert.True(t, isNotFound(err), "Handler muss daraus 404 machen, got %v", err)
	})

	t.Run("Datenbankfehler bleibt ein Fehler, nicht 404", func(t *testing.T) {
		boom := errors.New("db down")
		err := requireTaskEntity(ctx, "org-a", "control", "ctl-1",
			func(context.Context, string, string) (bool, error) { return false, boom })
		require.ErrorIs(t, err, boom)
		assert.False(t, isNotFound(err))
	})

	t.Run("andere Entitäten werden nicht geprüft", func(t *testing.T) {
		for _, et := range []string{"risk", "incident", "policy", "audit"} {
			called := false
			err := requireTaskEntity(ctx, "org-a", et, "x",
				func(context.Context, string, string) (bool, error) { called = true; return false, nil })
			require.NoError(t, err, et)
			assert.False(t, called, "%s: keine Kontroll-Prüfung erwartet", et)
		}
	})
}

// Die frühere Checkliste nahm jeden nicht leeren Schritt bis 500 Zeichen an.
// Die zusammengelegte Liste muss das weiter tun — Person, Frist und Priorität
// bleiben optional.
func TestCreateTaskInputAcceptsChecklistSteps(t *testing.T) {
	v := validator.New()

	ok := []CreateTaskInput{
		{Title: "A"},                      // ein Zeichen wie in der alten Checkliste
		{Title: strings.Repeat("x", 500)}, // Obergrenze der alten Checkliste
		{Title: "Firewall-Regeln prüfen", AssigneeEmail: "it@example.org"},
		{Title: "Firewall-Regeln prüfen", Status: "done", Priority: "high"},
	}
	for _, in := range ok {
		assert.NoError(t, v.Struct(in), "erwartet gültig: %+v", in.Title[:min(len(in.Title), 20)])
	}

	bad := []CreateTaskInput{
		{Title: ""},
		{Title: strings.Repeat("x", 501)},
		{Title: "x", AssigneeEmail: "keine-mail"},
		{Title: "x", Status: "erledigt"},
		{Title: "x", Priority: "dringend"},
	}
	for _, in := range bad {
		assert.Error(t, v.Struct(in), "erwartet ungültig: %q", in.Title[:min(len(in.Title), 20)])
	}

	upd := UpdateTaskInput{Title: ptr(strings.Repeat("y", 500))}
	assert.NoError(t, v.Struct(upd))
	upd = UpdateTaskInput{Title: ptr(strings.Repeat("y", 501))}
	assert.Error(t, v.Struct(upd))
}
