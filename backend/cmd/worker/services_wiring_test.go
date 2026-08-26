// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Das Deckungsgate zu services.go (R1-19-04).
//
// Der Defekt war nicht, dass jemand einen Setter falsch aufgerufen hat — er
// war, dass zwölf Handler den Dienst selbst gebaut und die Verdrahtung gar
// nicht erst erwähnt haben. Ein Test, der prüft „der Setter existiert" oder
// „der Setter gibt den Empfänger zurück", wäre dagegen blind: Beides war die
// ganze Zeit wahr, während die Mails nicht ankamen.
//
// Geprüft wird deshalb die BAUFORM, mit zwei Aussagen:
//
//	(A) Kein Worker-Quelltext außer services.go baut einen Dienst, dessen
//	    Verdrahtung folgenreich ist. Verstöße werden mit Datei UND Zeile
//	    benannt.
//	(B) Sperrklinke: Jeder Dienst-Konstruktor im Worker ist entweder in
//	    services.go verdrahtet oder steht in bareServiceConstructors mit
//	    einer gemessenen Begründung. Ein NEUER Diensttyp, den niemand
//	    eingeordnet hat, macht den Test rot.
//
// (B) ist der Teil, der die dreizehnte Stelle fängt. (A) allein wäre eine
// kuratierte Liste und hätte genau die Teilmengen-Lücke, an der dieses
// Projekt schon mehrfach hängengeblieben ist: Ein Dienst, an den beim
// Schreiben der Liste niemand gedacht hat, wäre stillschweigend erlaubt.

// wiringFile ist die einzige Datei, die verdrahtete Dienste bauen darf.
const wiringFile = "services.go"

// guardedConstructors sind Konstruktoren, deren fehlende Verdrahtung im Worker
// nachweislich Folgen hat. Schlüssel ist "paket.Funktion".
var guardedConstructors = map[string]string{
	"vaktcomply.NewService":             "ohne WithNotifyService gehen die NIS2-/DORA-Meldefrist-Mails still verloren",
	"vakthr.NewService":                 "ohne WithEvidenceWriter entsteht beim Vertragsablauf kein Offboarding-Nachweis; ohne WithEmployeeOnboardingTrigger erreicht kein Eintritt vaktaware (R1-SA25-01)",
	"vakthr.NewServiceFromPool":         "ohne WithEvidenceWriter entsteht beim Vertragsablauf kein Offboarding-Nachweis; ohne WithEmployeeOnboardingTrigger erreicht kein Eintritt vaktaware (R1-SA25-01)",
	"cloudintegration.NewService":       "mit dem Noop-Schreiber erhebt cloud_sync null Nachweise und meldet trotzdem Erfolg",
	"vaktcomply.NewHREvidenceWriter":    "gehört zu newHRService — gebaut wird an einer Stelle",
	"vaktcomply.NewCloudEvidenceWriter": "gehört zu newCloudService — gebaut wird an einer Stelle",
}

// bareServiceConstructors sind Dienste, die der Worker bewusst ohne die
// API-seitige Verdrahtung baut, weil die fehlende Abhängigkeit auf keinem
// Worker-Pfad gelesen wird. Jeder Eintrag trägt das Messergebnis.
//
// Wer hier etwas einträgt, behauptet: Ich habe nachgesehen, welche Methoden
// das nicht gesetzte Feld lesen, und keine davon ist aus einem Worker-Handler
// erreichbar. Das ist eine überprüfbare Aussage, keine Vermutung.
var bareServiceConstructors = map[string]string{
	"vaktprivacy.NewService":      "WithSubjectErasers/WithSubjectResolver werden nur aus ExecuteErasure gelesen; die drei Worker-Pfade (AVV-Ablauf, überfällige Anträge, Lösch-Erinnerungen) gehen über s.db und notify.Send",
	"vaktaware.NewService":        "der asynq-Client wird nur in LaunchCampaign/CompleteAssignment/EnqueueAutoEnrollment gelesen; der Worker ruft SendCampaignEmails und HandleAutoEnrollment",
	"vaktvault.NewService":        "masterKey und queue werden auf dem Worker-Pfad CreateAccessReview nicht berührt",
	"siem.NewService":             "API und Worker bauen identisch",
	"alerting.NewService":         "hat keine Setter; alle drei Argumente (Pool, aus \"vakt-alert-v1\" abgeleiteter Schlüssel, SMTP-Konfiguration) sind auf beiden Seiten dieselben",
	"scheduledreports.NewService": "der Worker setzt hier MEHR als die API (WithBoardReportProvider); die API ruft ProcessDue nicht auf",
}

// collectWorkerConstructorCalls parst alle Nicht-Test-Dateien des Worker-Pakets
// und liefert jeden Aufruf der Form <paket>.New<Etwas>(...) mit Fundort.
//
// Bewusst über den Syntaxbaum statt per regulärem Ausdruck: Ein Textmuster
// trifft auch Kommentare und Zeichenketten — der Vorgänger dieses Tests hat
// genau deshalb den eigenen Erklärtext mitgezählt.
type ctorCall struct {
	pkg, fn, file string
	line          int
}

func (c ctorCall) qualified() string { return c.pkg + "." + c.fn }

func collectWorkerConstructorCalls(t *testing.T) (calls []ctorCall, filesChecked int) {
	t.Helper()

	entries, err := os.ReadDir(".")
	require.NoError(t, err)

	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := parser.ParseFile(fset, filepath.Clean(name), nil, 0)
		require.NoError(t, err, "%s ließ sich nicht parsen", name)
		filesChecked++

		ast.Inspect(src, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkgIdent, ok := sel.X.(*ast.Ident)
			if !ok || !strings.HasPrefix(sel.Sel.Name, "New") {
				return true
			}
			calls = append(calls, ctorCall{
				pkg:  pkgIdent.Name,
				fn:   sel.Sel.Name,
				file: name,
				line: fset.Position(call.Pos()).Line,
			})
			return true
		})
	}
	return calls, filesChecked
}

// TestGuardedServicesAreBuiltOnlyInServicesGo ist Aussage (A).
func TestGuardedServicesAreBuiltOnlyInServicesGo(t *testing.T) {
	calls, filesChecked := collectWorkerConstructorCalls(t)

	require.Greater(t, filesChecked, 5,
		"Deckungsgate hat kaum Dateien gesehen (%d) — Verzeichnis oder Filter stimmen nicht", filesChecked)
	require.NotEmpty(t, calls,
		"kein einziger Konstruktoraufruf gefunden — der Syntaxbaum-Durchlauf greift nicht")

	var seenGuarded int
	for _, c := range calls {
		reason, guarded := guardedConstructors[c.qualified()]
		if !guarded {
			continue
		}
		seenGuarded++
		assert.Equal(t, wiringFile, c.file,
			"%s:%d baut %s am zentralen Konstruktor vorbei — %s. "+
				"Bau den Dienst in %s und ruf ihn hier nur ab.",
			c.file, c.line, c.qualified(), reason, wiringFile)
	}

	// Nicht-Vakuität: Stünde in services.go keiner dieser Konstruktoren mehr,
	// liefe die Schleife leer und der Test wäre grün, ohne etwas geprüft zu
	// haben.
	require.GreaterOrEqual(t, seenGuarded, 3,
		"es wurden nur %d bewachte Konstruktoren gefunden — services.go baut die Dienste nicht mehr, "+
			"das Gate prüft ins Leere", seenGuarded)

	t.Logf("geprüft: %d Dateien, %d Konstruktoraufrufe, davon %d bewacht",
		filesChecked, len(calls), seenGuarded)
}

// TestNoUnclassifiedServiceConstructor ist Aussage (B) — die Sperrklinke.
func TestNoUnclassifiedServiceConstructor(t *testing.T) {
	calls, filesChecked := collectWorkerConstructorCalls(t)
	require.Greater(t, filesChecked, 5, "zu wenige Dateien gesehen")

	var unclassified []string
	var classified int
	for _, c := range calls {
		// Nur Dienst-Konstruktoren. Repositories, Task-Konstruktoren und
		// Hilfsobjekte tragen im Projekt andere Namen und werden nicht
		// verdrahtet.
		if c.fn != "NewService" && !strings.HasPrefix(c.fn, "NewServiceFrom") {
			continue
		}
		if c.file == wiringFile {
			classified++
			continue
		}
		if _, ok := bareServiceConstructors[c.qualified()]; ok {
			classified++
			continue
		}
		unclassified = append(unclassified,
			c.file+":"+itoa(c.line)+" — "+c.qualified())
	}

	sort.Strings(unclassified)
	assert.Empty(t, unclassified,
		"neuer Dienst-Konstruktor im Worker, den niemand eingeordnet hat:\n  %s\n\n"+
			"Entweder in %s bauen (wenn eine Abhängigkeit fehlt, die auf einem "+
			"Worker-Pfad gelesen wird), oder in bareServiceConstructors eintragen — "+
			"mit dem Messergebnis, welche Methoden das nicht gesetzte Feld lesen "+
			"und warum keine davon aus einem Worker-Handler erreichbar ist.",
		strings.Join(unclassified, "\n  "), wiringFile)

	require.Greater(t, classified, 5,
		"nur %d eingeordnete Dienst-Konstruktoren gefunden — die Sperrklinke misst ins Leere", classified)
	t.Logf("geprüft: %d Dateien, %d eingeordnete Dienst-Konstruktoren, %d offen",
		filesChecked, classified, len(unclassified))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
