// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/shared/notify"
)

// R1-19-04: Der Versand der NIS2-/DORA-Meldefrist-Mail war eine Closure ohne
// Rückgabewert. Fehlte der Benachrichtigungsdienst, stieg sie wortlos aus —
// und der Aufrufer schrieb danach „overdue notification sent" ins Log.
//
// Diese Tests halten die Umkehrung fest: Der Versand MELDET sein Ergebnis,
// damit der Aufrufer den Erfolgssatz an etwas binden kann.

type fakeNotifier struct {
	sent []notify.Message
	// failFor: Adressen, bei denen die Zustellung fehlschlägt.
	failFor map[string]bool
}

func (f *fakeNotifier) Notify(_ context.Context, msg notify.Message) error {
	if f.failFor[msg.Target] {
		return errors.New("smtp: connection refused")
	}
	f.sent = append(f.sent, msg)
	return nil
}

// Der wichtigste Fall: genau die Verdrahtungslücke, die live zwölfmal bestand.
func TestDeliverAdminEmailReportsMissingNotifier(t *testing.T) {
	err := deliverAdminEmail(context.Background(), nil, "org-1",
		[]string{"admin@example.test"}, "Betreff", "Text")

	require.Error(t, err,
		"ein nicht verdrahteter Versender muss ein Fehler sein, kein stiller Normalfall — "+
			"sonst loggt der Aufrufer weiter Erfolg")
	assert.ErrorIs(t, err, errNoNotifier)
}

func TestDeliverAdminEmailReportsMissingRecipients(t *testing.T) {
	f := &fakeNotifier{}
	err := deliverAdminEmail(context.Background(), f, "org-1", nil, "Betreff", "Text")

	assert.ErrorIs(t, err, errNoAdminRecipients)
	assert.Empty(t, f.sent)
}

// Baseline-Abnahme: Ein korrekt verdrahteter Versand funktioniert weiterhin.
func TestDeliverAdminEmailSucceedsWhenWired(t *testing.T) {
	f := &fakeNotifier{}
	err := deliverAdminEmail(context.Background(), f, "org-1",
		[]string{"a@example.test", "b@example.test"}, "Betreff", "Text")

	require.NoError(t, err)
	require.Len(t, f.sent, 2)
	assert.Equal(t, "Betreff", f.sent[0].Title)
	assert.Equal(t, "org-1", f.sent[0].OrgID)
	assert.Equal(t, notify.ChannelEmail, f.sent[0].Channel)
	assert.Equal(t, "a@example.test", f.sent[0].Target)
	assert.Equal(t, "b@example.test", f.sent[1].Target)
}

// Scheitert JEDE Zustellung, hat niemand die Meldefrist erfahren — das ist ein
// Fehler, auch wenn der Versender vorhanden war.
func TestDeliverAdminEmailFailsWhenNobodyReached(t *testing.T) {
	f := &fakeNotifier{failFor: map[string]bool{
		"a@example.test": true,
		"b@example.test": true,
	}}
	err := deliverAdminEmail(context.Background(), f, "org-1",
		[]string{"a@example.test", "b@example.test"}, "Betreff", "Text")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "keine Zustellung gelungen")
	assert.Contains(t, err.Error(), "a@example.test",
		"der Fehler muss sagen, welche Adresse gescheitert ist")
}

// Eine Teilzustellung gilt als Erfolg: Die Erinnerung hat ihr Ziel erreicht.
// Die kaputte Adresse ist ein getrenntes Problem und wird geloggt, nicht
// zurückgemeldet — sonst meldet ein Tippfehler in einer von fünf Adressen die
// gesamte Meldefrist-Erinnerung als ausgefallen.
func TestDeliverAdminEmailPartialDeliveryCountsAsDelivered(t *testing.T) {
	f := &fakeNotifier{failFor: map[string]bool{"kaputt@example.test": true}}
	err := deliverAdminEmail(context.Background(), f, "org-1",
		[]string{"kaputt@example.test", "gut@example.test"}, "Betreff", "Text")

	require.NoError(t, err)
	require.Len(t, f.sent, 1)
	assert.Equal(t, "gut@example.test", f.sent[0].Target)
}
