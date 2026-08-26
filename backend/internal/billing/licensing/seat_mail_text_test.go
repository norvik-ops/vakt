// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package licensing

import (
	"strings"
	"testing"
	"time"
)

// Die Sitzlizenz-Mail geht an den ENDKUNDEN EINES MSP — an jemanden, der uns nie
// Geld geschickt hat. Sie behauptete ihm gegenueber drei Dinge:
//
//	"Deine Vakt Pro Lizenz — Zahlung eingegangen"
//	"deine Zahlung ist eingegangen — vielen Dank."
//	"Er ersetzt den 45-Tage-Schluessel aus der Auftragsbestaetigung."
//
// Keines davon stimmte: gezahlt hat der MSP, eine Auftragsbestaetigung hat der
// Empfaenger nie gesehen, einen 45-Tage-Schluessel nie gehabt. Es ist die einzige
// Mail, die ein Drittkunde je von NorvikOps bekommt.
//
// Geprueft wird hier wie bei der Jahr/Monat-Falle daneben: am Text, mit dem bloßen
// Wort statt der heutigen Formulierung — eine Liste der Phrasen, die wir gerade
// benutzen, wuerde die naechste nicht fangen.
func TestSeatMailNeverClaimsAPaymentTheRecipientDidNotMake(t *testing.T) {
	expires := time.Date(2027, 3, 4, 8, 0, 0, 0, time.UTC)

	subject, body := licenseMail(
		ForLicence("bb2e9d02-0000-4000-8000-000000000001",
			"Endkunde GmbH", "endkunde@example.com", "year").ForSeat().r,
		"vakt_key", expires)

	for _, forbidden := range []string{"Zahlung", "Auftragsbestätigung", "45-Tage", "Rechnung"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("Sitzlizenz-Mail behauptet %q gegenüber einem Empfänger, der nie gezahlt hat:\n%s", forbidden, body)
		}
		if strings.Contains(subject, forbidden) {
			t.Errorf("Betreff der Sitzlizenz-Mail behauptet %q: %q", forbidden, subject)
		}
	}

	// Keine Laufzeit-Floskel: ein Sitzschluessel endet, wenn die bezahlte Periode
	// DES MSP endet, nicht nach einem vollen Jahr. Wer drei Monate nach dem
	// Jahreskauf einen Platz bekommt, haelt einen 9-Monats-Schluessel — "ein volles
	// Jahr" waere die naechste falsche Tatsachenbehauptung in derselben Mail.
	for _, forbidden := range []string{"ein volles Jahr", "einen vollen Monat"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("Sitzlizenz-Mail verspricht eine Laufzeit (%q), die der Schlüssel nicht hat:\n%s", forbidden, body)
		}
	}

	// Was stattdessen drinstehen MUSS, sonst waere "keine Zahlung behaupten" durch
	// eine leere Mail erfuellbar: das echte Ablaufdatum, der Schluessel, und woher
	// der Zugang kommt.
	if !strings.Contains(body, "04.03.2027") {
		t.Errorf("Ablaufdatum fehlt in der Sitzlizenz-Mail:\n%s", body)
	}
	if !strings.Contains(body, "vakt_key") {
		t.Errorf("Schlüssel fehlt in der Sitzlizenz-Mail:\n%s", body)
	}
	if !strings.Contains(body, "Dienstleister") {
		t.Errorf("Sitzlizenz-Mail sagt dem Empfänger nicht, woher sein Zugang kommt:\n%s", body)
	}
	// Der Auto-Renewal-Absatz muss bleiben: der Token ist gesetzt, und ohne den
	// Absatz waere die Sitzlizenz die einzige, die sich nicht selbst verlaengert.
	if !strings.Contains(body, "VAKT_LICENSE_TOKEN=") {
		t.Errorf("Auto-Renewal-Absatz fehlt in der Sitzlizenz-Mail:\n%s", body)
	}
}

// TestPayingCustomerMailStillNamesThePayment ist die Gegenprobe, ohne die der Test
// oben vakuum waere: haette der Fix den Zahlungstext einfach ueberall entfernt,
// waeren beide Mails "korrekt" und die Mail an den zahlenden Direktkunden waere
// stumm ueber das, wofuer er gerade Geld ueberwiesen hat.
func TestPayingCustomerMailStillNamesThePayment(t *testing.T) {
	expires := time.Date(2027, 3, 4, 8, 0, 0, 0, time.UTC)

	subject, body := licenseMail(
		ForLicence("bb2e9d02-0000-4000-8000-000000000002",
			"Direktkunde GmbH", "kunde@example.com", "year").r,
		"vakt_key", expires)

	if !strings.Contains(subject, "Zahlung eingegangen") {
		t.Errorf("die Mail nach dem Zahlungseingang muss ihn benennen: %q", subject)
	}
	for _, want := range []string{"Zahlung ist eingegangen", "Auftragsbestätigung", "ein volles Jahr"} {
		if !strings.Contains(body, want) {
			t.Errorf("Mail an den zahlenden Direktkunden nennt %q nicht mehr:\n%s", want, body)
		}
	}
}

// TestTrialMailIsUnchangedBySeatHandling haelt fest, dass der dritte Zweig die
// beiden alten nicht verschoben hat — der 45-Tage-Fall ist derjenige, der eine
// Zahlung ANKUENDIGT statt sie zu behaupten.
func TestTrialMailIsUnchangedBySeatHandling(t *testing.T) {
	expires := time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)

	subject, body := licenseMail(
		ForLicence("bb2e9d02-0000-4000-8000-000000000003",
			"Neukunde GmbH", "neu@example.com", "month").AsTrial().r,
		"vakt_key", expires)

	if !strings.Contains(subject, "45 Tage") {
		t.Errorf("Trial-Betreff verändert: %q", subject)
	}
	if !strings.Contains(body, "Sobald deine Zahlung eingegangen ist") {
		t.Errorf("Trial-Mail kündigt den Zahlungseingang nicht mehr an:\n%s", body)
	}
	if strings.Contains(body, "Jahr") {
		t.Errorf("Monats-Trial spricht von einem Jahr:\n%s", body)
	}
}

// TestForLicenceCannotProduceATokenlessKey belegt die strukturelle Haelfte von
// K4-01: der Renewal-Token ist kein Feld mehr, das ein Aufrufer vergessen kann,
// sondern ein Argument von ForLicence. Der Zweig, der ihn dennoch leer sieht, muss
// LAUT scheitern und nicht stillschweigend einen tokenlosen Schluessel ausstellen —
// genau das war der CRITICAL.
func TestForLicenceCannotProduceATokenlessKey(t *testing.T) {
	iss := NewIssuer("not-a-real-pem-but-non-empty", SMTPConfig{})

	_, err := iss.SignUntil(ForLicence("", "Irgendwer GmbH", "x@example.com", "year"), time.Now().Add(time.Hour))
	if err == nil {
		t.Fatal("SignUntil hat einen Schlüssel ohne Renewal-Token ausgestellt — das ist der Zustand, in dem eine bezahlte Instanz dunkel geht")
	}
	if !strings.Contains(err.Error(), "renewal token") {
		t.Errorf("der Fehler nennt die Ursache nicht: %v", err)
	}
}
