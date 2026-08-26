// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package licensing

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Dieses Gate haelt die Zusage ein, die ADR-0081 formuliert.
//
// Der Typ allein schliesst nur den halben Weg: `ForLicence`/`SignUntil`/`IssueUntil`
// koennen ohne Renewal-Token nicht kompiliert werden, aber `Issue(r Request, …)` und
// `Sign(r Request)` bleiben exportiert und nehmen weiter ein nacktes `Request` — eine
// neue zeilen-gestuetzte Aufrufstelle koennte sie benutzen und den Token dabei STILL
// verlieren (`sign()` gibt `r.renewalToken == ""` ohne Murren weiter; einen
// Leertoken-Guard hat nur `SignUntil`). Genau diese Luecke hat das Review von
// `fix/k4-money-path` gemessen: eine Probe-Methode im Paket `lexware`, die
// `issuer.Issue(licensing.Request{…}, nil, "")` aufruft, kompilierte.
//
// Das Gate schliesst sie ueber das ARGUMENT statt ueber den Aufruf, und das ist der
// vollstaendigere Schnitt: `Issue`/`Sign` sind ohne einen `Request`-Wert nicht
// aufrufbar, und einen solchen Wert kann ein anderes Paket nur auf zwei Wegen bekommen.
//
//	Regel 1 — den Typ NENNEN (`licensing.Request{…}`, `var r licensing.Request`,
//	          `new(licensing.Request)`, als Parameter- oder Feldtyp). Das darf nur,
//	          wer in requestAllowlist steht.
//	Regel 2 — ihn von `licensing` HERAUSGEREICHT bekommen (Rueckgabewert einer
//	          exportierten Funktion/Methode, exportiertes Feld eines exportierten
//	          Typs, exportierte Variable, exportiertes Alias). Das darf niemand.
//
// Beides zusammen ist dicht: es gibt keinen dritten Weg an einen `Request` heran.
// `internal/` verhindert, dass Code ausserhalb dieses Moduls das Paket ueberhaupt
// importiert, also ist der modulweite Lauf unten die vollstaendige Menge der
// Aufrufer. Parameter-Typen bleiben erlaubt (`Issue(r Request)` MUSS einen nehmen),
// herausgereichte Werte nicht.
//
// Warum ein Go-Test und kein Skript in scripts/: so laeuft es in jedem
// `go test ./...` mit, ohne eine Verdrahtungszeile in einem Workflow, der einer
// anderen Spur gehoert.

// requestAllowlist sind die Dateien, die `licensing.Request` nennen DUERFEN, mit
// Begruendung. Jeder Eintrag ist ein Loch im Gate und muss sich rechtfertigen.
var requestAllowlist = map[string]string{
	"cmd/admin/main.go": "Admin-CLI: signiert von Hand, hat legitim keine Lizenzzeile und damit keinen Renewal-Token",
}

// TestOnlyTheAdminCLIMayNameTheNakedRequest ist Regel 1.
func TestOnlyTheAdminCLIMayNameTheNakedRequest(t *testing.T) {
	root := moduleRoot(t)
	fset := token.NewFileSet()

	scanned, unreadable := 0, []string{}
	offenders := map[string][]int{}
	usedAllowlist := map[string]bool{}

	require.NoError(t, filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			unreadable = append(unreadable, rel(root, path)+": "+err.Error())
			return nil
		}
		if d.IsDir() {
			if skipDir(d.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		r := rel(root, path)
		// Das Paket selbst ist nicht Gegenstand des Gates: hier WOHNT der Typ, und
		// `ForLicence` muss ihn bauen duerfen. Qualifizierte Nennungen gibt es hier
		// ohnehin nicht.
		if filepath.Dir(r) == "internal/billing/licensing" {
			scanned++
			return nil
		}

		src, readErr := os.ReadFile(path)
		if readErr != nil {
			unreadable = append(unreadable, r+": "+readErr.Error())
			return nil
		}
		f, parseErr := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
		if parseErr != nil {
			unreadable = append(unreadable, r+": "+parseErr.Error())
			return nil
		}
		scanned++

		lines := namesNakedRequest(fset, f)
		if len(lines) == 0 {
			return nil
		}
		if _, ok := requestAllowlist[r]; ok {
			usedAllowlist[r] = true
			return nil
		}
		offenders[r] = lines
		return nil
	}))

	// Nenner zuerst: ein Gate, das nichts gelesen hat, ist gruen aus dem falschen
	// Grund. Das war die Lektion aus dem Interface-Ratchet.
	require.Empty(t, unreadable,
		"Gate konnte %d Datei(en) nicht lesen/parsen — ungelesen ist nicht ungeprueft, sondern unbekannt:\n%s",
		len(unreadable), strings.Join(unreadable, "\n"))
	require.Greater(t, scanned, 100,
		"nur %d Go-Dateien gescannt — der Lauf hat den Baum nicht gefunden, das Gate waere leer gruen", scanned)

	// Die Ausnahme muss WIRKLICH benutzt werden. Sonst hat jemand das Admin-CLI
	// umgebaut, und die Ausnahmeliste steht als toter, unbegruendeter Freibrief da.
	for path, reason := range requestAllowlist {
		assert.True(t, usedAllowlist[path],
			"Ausnahme %q (%s) nennt licensing.Request nicht mehr — Eintrag streichen, damit die Liste kein toter Freibrief ist", path, reason)
	}

	if len(offenders) > 0 {
		for _, path := range sortedKeys(offenders) {
			t.Errorf("%s:%v nennt licensing.Request. Ein Schluessel mit einer Zeile in billing_licenses "+
				"MUSS ueber licensing.ForLicence(...) + SignUntil/IssueUntil gehen — Issue/Sign mit nacktem "+
				"Request verliert den Renewal-Token STILL, und die Instanz geht nach Ablauf dunkel, obwohl "+
				"bezahlt (ADR-0081). Wenn dieser Aufrufer wirklich keine Lizenzzeile hat: mit Begruendung in "+
				"requestAllowlist aufnehmen.", path, offenders[path])
		}
	}
}

// TestLicensingHandsOutNoNakedRequest ist Regel 2 — die Hintertuer, die Regel 1
// wertlos machen wuerde: wer einen Request von `licensing` GESCHENKT bekommt, muss
// den Typ nie nennen.
func TestLicensingHandsOutNoNakedRequest(t *testing.T) {
	root := moduleRoot(t)
	dir := filepath.Join(root, "internal", "billing", "licensing")

	// parser.ParseDir ist deprecated (SA1019), also selbst auflisten und Datei fuer Datei
	// parsen. Gleiches Ergebnis, und der Nenner ist dabei sichtbar statt implizit.
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	files := make([]*ast.File, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, perr := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		require.NoError(t, perr, name)
		files = append(files, f)
	}
	require.NotEmpty(t, files, "licensing-Paket nicht geparst — leerer Nenner")

	var leaks []string
	for _, f := range files {
		path := fset.Position(f.Pos()).Filename
		leaks = append(leaks, exportedLeaksOfRequest(fset, f, rel(root, path))...)
	}
	require.Greater(t, len(files), 0, "keine Datei im licensing-Paket gelesen")

	sort.Strings(leaks)
	assert.Empty(t, leaks,
		"licensing reicht `Request` nach draussen — damit kann eine Aufrufstelle Issue/Sign fuettern, ohne "+
			"den Typ zu nennen, und Regel 1 (TestOnlyTheAdminCLIMayNameTheNakedRequest) greift nicht mehr:\n%s",
		strings.Join(leaks, "\n"))
}

// TestRequestGateActuallyFires ist die Nicht-Vakuitaets-Sperre.
//
// Beide Tests oben sind heute gruen. Gruen heisst nur dann etwas, wenn der Detektor
// ueberhaupt ausloest — sonst pinnt dieses Gate nichts und behauptet es doch. Also
// werden hier synthetische Quellen durch DENSELBEN Detektor geschickt.
func TestRequestGateActuallyFires(t *testing.T) {
	fset := token.NewFileSet()
	parse := func(src string) *ast.File {
		f, err := parser.ParseFile(fset, "probe.go", src, parser.SkipObjectResolution)
		require.NoError(t, err)
		return f
	}

	// Regel 1 — genau die Probe, mit der das Review die Luecke gemessen hat.
	reviewProbe := parse(`package lexware
import "github.com/matharnica/vakt/internal/billing/licensing"
func (h *Handler) zzzProbeForgetTheToken(org, email, interval string) (string, error) {
	return h.issuer.Issue(licensing.Request{OrgName: org, Email: email, Interval: interval}, nil, "")
}`)
	assert.NotEmpty(t, namesNakedRequest(fset, reviewProbe),
		"Detektor findet das Composite-Literal nicht — dann ist Regel 1 leer gruen")

	for name, src := range map[string]string{
		"var-Deklaration": `package p
import "github.com/matharnica/vakt/internal/billing/licensing"
func f(i *licensing.Issuer) { var r licensing.Request; _, _ = i.Sign(r) }`,
		"new()": `package p
import "github.com/matharnica/vakt/internal/billing/licensing"
func f(i *licensing.Issuer) { _, _ = i.Sign(*new(licensing.Request)) }`,
		"Parameter-Typ": `package p
import "github.com/matharnica/vakt/internal/billing/licensing"
func helper(r licensing.Request) licensing.Request { return r }`,
		"Struct-Feld": `package p
import "github.com/matharnica/vakt/internal/billing/licensing"
type job struct{ r licensing.Request }`,
		"Alias": `package p
import "github.com/matharnica/vakt/internal/billing/licensing"
type naked = licensing.Request`,
	} {
		assert.NotEmpty(t, namesNakedRequest(fset, parse(src)), "Regel 1 uebersieht %s", name)
	}

	// Gegenprobe: der erlaubte Weg darf NICHT anschlagen, sonst waere das Gate ein
	// Dauer-Rot und wuerde abgeschaltet statt befolgt.
	legit := parse(`package lexware
import (
	"time"

	"github.com/matharnica/vakt/internal/billing/licensing"
)
func f(i *licensing.Issuer, token, org, email, interval string, expires time.Time) {
	_, _ = i.SignUntil(licensing.ForLicence(token, org, email, interval).AsTrial(), expires)
}`)
	assert.Empty(t, namesNakedRequest(fset, legit),
		"der ForLicence-Weg darf nicht anschlagen — ein Gate, das den richtigen Weg verbietet, wird abgeschaltet")
	assert.Empty(t, namesNakedRequest(fset, parse(`package p
// licensing.Request steht hier nur im Kommentar.
const s = "licensing.Request"`)),
		"Kommentar/String darf nicht zaehlen — Gates, die Prosa zaehlen, luegen")

	// Regel 2 — jede der vier Hintertueren muss auffallen.
	for name, src := range map[string]string{
		"exportierte Funktion mit Rueckgabe": `package licensing
func NakedFor(org string) Request { return Request{OrgName: org} }`,
		"exportierte Methode mit Rueckgabe": `package licensing
func (l LicenceRequest) Raw() Request { return l.r }`,
		"exportiertes Feld": `package licensing
type Job struct{ R Request }`,
		"exportiertes Alias": `package licensing
type Naked = Request`,
		"eingebettetes Request": `package licensing
type Job struct{ Request }`,
		"exportierte Variable": `package licensing
var Blank Request`,
	} {
		assert.NotEmpty(t, exportedLeaksOfRequest(fset, parse(src), "probe.go"),
			"Regel 2 uebersieht %s", name)
	}

	// Und die echten Signaturen von Issue/Sign duerfen nicht als Leck gelten: sie
	// NEHMEN einen Request, sie geben keinen heraus.
	assert.Empty(t, exportedLeaksOfRequest(fset, parse(`package licensing
func (i *Issuer) Issue(r Request, pdf []byte, name string) (string, error) { return "", nil }
func (i *Issuer) Sign(r Request) (string, error) { return "", nil }`), "probe.go"),
		"ein Request als PARAMETER ist kein Leck — sonst ist Regel 2 ein Dauer-Rot")
}

// ── Detektoren ───────────────────────────────────────────────────────────────

// namesNakedRequest liefert die Zeilen, in denen die Datei den Typ
// `<licensing-Alias>.Request` nennt — in JEDER Rolle (Literal, var, new(),
// Parameter, Feld, Alias). Es liest den AST, nicht den Text: ein
// `licensing.Request` in einem Kommentar oder String zaehlt nicht.
func namesNakedRequest(fset *token.FileSet, f *ast.File) []int {
	alias := licensingImportName(f)
	if alias == "" || alias == "_" {
		// Kein Import oder Blank-Import: dieser Datei kann der Typ nicht in die Hand
		// fallen.
		return nil
	}
	seen := map[int]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		// Dot-Import: `Request` steht dann unqualifiziert da. Kein heutiger Fall,
		// aber ein Gate, das nur die qualifizierte Form kennt, waere damit umgehbar.
		if alias == "." {
			if id, ok := n.(*ast.Ident); ok && id.Name == "Request" {
				seen[fset.Position(id.Pos()).Line] = true
			}
			return true
		}
		sel, ok := n.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Request" {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == alias {
			seen[fset.Position(sel.Pos()).Line] = true
		}
		return true
	})
	lines := make([]int, 0, len(seen))
	for l := range seen {
		lines = append(lines, l)
	}
	sort.Ints(lines)
	return lines
}

// licensingImportName gibt den Namen zurueck, unter dem die Datei das
// licensing-Paket importiert ("" = importiert es nicht). Ein Alias-Import
// (`lic "…/licensing"`) wuerde ein Gate auf den festen String "licensing"
// umgehen.
func licensingImportName(f *ast.File) string {
	const pkgPath = `"github.com/matharnica/vakt/internal/billing/licensing"`
	for _, imp := range f.Imports {
		if imp.Path.Value != pkgPath {
			continue
		}
		if imp.Name != nil {
			return imp.Name.Name
		}
		return "licensing"
	}
	return ""
}

// exportedLeaksOfRequest findet jede exportierte Stelle, an der das
// licensing-Paket einen `Request` HERAUSREICHT. Parameter sind ausgenommen —
// `Issue(r Request, …)` muss einen nehmen duerfen.
func exportedLeaksOfRequest(fset *token.FileSet, f *ast.File, path string) []string {
	var leaks []string
	at := func(n ast.Node, what string) {
		leaks = append(leaks, path+":"+strconv.Itoa(fset.Position(n.Pos()).Line)+": "+what)
	}

	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if !d.Name.IsExported() || d.Type.Results == nil {
				continue
			}
			for _, res := range d.Type.Results.List {
				if mentionsRequest(res.Type) {
					at(d, "exportierte Funktion/Methode "+d.Name.Name+" gibt Request zurueck")
				}
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.ValueSpec:
					for _, name := range s.Names {
						if name.IsExported() && s.Type != nil && mentionsRequest(s.Type) {
							at(s, "exportierte Variable/Konstante "+name.Name+" hat den Typ Request")
						}
					}
				case *ast.TypeSpec:
					if !s.Name.IsExported() {
						continue
					}
					if s.Assign.IsValid() && mentionsRequest(s.Type) {
						at(s, "exportiertes Alias "+s.Name.Name+" auf Request")
					}
					st, ok := s.Type.(*ast.StructType)
					if !ok {
						continue
					}
					for _, fld := range st.Fields.List {
						if !mentionsRequest(fld.Type) {
							continue
						}
						if len(fld.Names) == 0 {
							at(fld, "exportierter Typ "+s.Name.Name+" bettet Request ein (promoviertes Feld ist exportiert)")
							continue
						}
						for _, name := range fld.Names {
							if name.IsExported() {
								at(fld, "exportiertes Feld "+s.Name.Name+"."+name.Name+" hat den Typ Request")
							}
						}
					}
				}
			}
		}
	}
	return leaks
}

// mentionsRequest prueft einen Typ-Ausdruck auf den paketlokalen Namen `Request`
// — auch verpackt (*Request, []Request, map[string]Request, chan Request). Ein
// `x.Request` aus einem ANDEREN Paket ist nicht unser Typ, deshalb wird in
// Selector-Ausdruecke nicht abgestiegen.
func mentionsRequest(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		if _, ok := n.(*ast.SelectorExpr); ok {
			return false
		}
		if id, ok := n.(*ast.Ident); ok && id.Name == "Request" {
			found = true
		}
		return !found
	})
	return found
}

// ── Rahmen ───────────────────────────────────────────────────────────────────

// moduleRoot laeuft von diesem Paket aus nach oben bis zur go.mod. Ein festes
// "../../../.." waere beim naechsten Umbenennen einer Ebene still falsch — und
// ein Gate, das ins Leere zeigt, ist gruen ohne Aussage.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, parent, dir, "keine go.mod oberhalb von %s gefunden", dir)
		dir = parent
	}
}

func skipDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", "testdata", "dist", "build":
		return true
	}
	return false
}

func rel(root, path string) string {
	r, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(r)
}

func sortedKeys(m map[string][]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
