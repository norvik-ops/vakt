// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package main

import (
	"fmt"
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

// Dieser Test prueft die ABDECKUNG der Ableitung in internal/shared/redisopt,
// nicht ihr Verhalten.
//
// Der Unterschied ist genau der Defekt. redisopt_test.go beweist, dass die
// Ableitung die Datenbanknummer durchreicht — und waere gruen geblieben,
// waehrend vier Enqueue-Stellen in cmd/api/routes.go die Struktur weiterhin von
// Hand bauen und DB fallen lassen. Ein Verhaltenstest sagt "die Funktion
// stimmt"; er sagt nichts darueber, ob irgendjemand sie benutzt — und der Bug
// war das Zweite.
//
// Was der Bug war (R1-14b-01): Die API baute asynq.RedisClientOpt{Addr, Password}
// von Hand, der Worker setzte zusaetzlich DB. Traegt VAKT_REDIS_URL eine
// Datenbanknummer (redis://host:6379/1 — voellig ueblich, wenn man sich in einer
// bestehenden Redis-Instanz eine eigene Datenbank nimmt), schreibt die API nach
// Datenbank 0 und der Worker liest aus Datenbank 1. Es gibt keinen Fehler, kein
// Log und keinen Wiederholungsversuch: beide Seiten sind fuer sich gesund, sie
// reden nur nicht miteinander. Dieselbe Struktur bauten drei Inspectors
// (Admin-Jobs, Admin-Health, Prometheus) — die melden dann "gesund" ueber
// Warteschlangen, die sie gar nicht ansehen.
//
// Drei Entwurfsentscheidungen, alle gegen genau diese Wiederholung:
//
//  1. Geprueft werden AUFRUFSTELLEN, nicht die Ableitung. Jede Uebergabe an
//     asynq.NewClient/NewInspector/NewServer/NewScheduler muss aus redisopt
//     stammen oder ein durchgereichter Parameter sein. Ein handgebautes Literal
//     ist rot — namentlich, mit Datei und Zeile.
//
//  2. Der Test kennt die Feldliste NICHT und darf sie nicht kennen. Fragte er
//     "enthaelt das Literal DB?", waere ein Literal mit DB und ohne Password
//     gruen — dieselbe Teilmengen-Luecke eine Ebene hoeher, und Password ist
//     schon einmal genau so verlorengegangen (S121-C3/S122-B2, NOAUTH). Die
//     Invariante ist deshalb strenger und feldfrei: es gibt genau eine Stelle,
//     die diese Struktur baut.
//
//  3. Was NICHT geprueft wird, wird gezaehlt und benannt. Die Zaehler stehen im
//     Testprotokoll, und jedes Verzeichnis muss Dateien geliefert haben — ein
//     umbenanntes Paket wuerde sonst still aus der Abdeckung fallen und das Gate
//     bliebe gruen ueber einer Teilmenge. Das ist in diesem Repo schon mehrfach
//     passiert (check_routes.py meldete OK und uebersprang ein Viertel der
//     Aufrufe).

// scannedDirs sind alle Pakete, in denen eine Asynq-Verbindung entsteht —
// relativ zu backend/. Ermittelt mit:
//
//	grep -rn 'asynq.New\(Client\|Inspector\|Server\|Scheduler\)' --include='*.go'
//
// Wer ein Paket hinzufuegt, in dem eine Asynq-Verbindung gebaut wird, traegt es
// hier ein. Vergisst er es, faellt es diesem Gate nicht auf — deshalb steht der
// Nenner im Protokoll und TestAsynqScanCoversEveryConstructionSite unten
// vergleicht die Liste gegen einen Repo-weiten Suchlauf.
var scannedDirs = []string{
	"cmd/api",
	"cmd/worker",
	"internal/admin",
	"internal/shared/metrics",
	"internal/shared/notify",
	"internal/shared/redisopt",
	"internal/modules/vaktscan",
	"internal/modules/vaktprivacy",
	"internal/modules/vaktaware",
}

// asynqConnConstructors sind die Asynq-Einstiegspunkte, die eine
// Verbindungsoption entgegennehmen. Alle vier nehmen sie als erstes Argument.
var asynqConnConstructors = map[string]bool{
	"NewClient":    true,
	"NewInspector": true,
	"NewServer":    true,
	"NewScheduler": true,
}

// backendRoot ist das Wurzelverzeichnis des Go-Moduls, von cmd/api aus gesehen.
const backendRoot = "../.."

// findingKind unterscheidet, WARUM eine Uebergabe in Ordnung ist. Nur fuer die
// Ausgabe — die Zahlen machen sichtbar, ob das Gate tatsaechlich etwas gesehen
// hat oder ueber einer leeren Menge gruen ist.
type findingKind string

const (
	kindDerivation  findingKind = "redisopt-Aufruf"
	kindDerivedVar  findingKind = "Variable aus redisopt"
	kindPassThrough findingKind = "durchgereichter Parameter"
	kindZeroLiteral findingKind = "Null-Literal (kein Redis)"
)

func TestAsynqConnOptionsAlwaysComeFromTheSharedDerivation(t *testing.T) {
	fset := token.NewFileSet()

	var violations []string
	counts := map[findingKind]int{}
	filesScanned := 0
	callsChecked := 0
	perDirFiles := map[string]int{}

	for _, dir := range scannedDirs {
		abs := filepath.Join(backendRoot, dir)
		entries, err := os.ReadDir(abs)
		require.NoError(t, err, "Verzeichnis %s aus scannedDirs existiert nicht — "+
			"wurde ein Paket umbenannt? Ein stillschweigend weggefallenes Paket "+
			"macht dieses Gate gruen ueber einer Teilmenge.", dir)

		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(abs, name)
			src, err := os.ReadFile(path)
			require.NoError(t, err)

			file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
			// Ein nicht parsebarer Produktionsdateiname ist ein Fund, kein Grund
			// zum Ueberspringen: still uebersprungene Eingaben sind genau die
			// Klasse, an der die Gates dieses Repos schon zweimal gelogen haben.
			require.NoError(t, err, "%s liess sich nicht parsen", dir+"/"+name)

			filesScanned++
			perDirFiles[dir]++

			v, c, checked := checkFileForHandBuiltAsynqOpts(fset, file, string(src), dir+"/"+name)
			violations = append(violations, v...)
			for k, n := range c {
				counts[k] += n
			}
			callsChecked += checked
		}
	}

	// Nicht-Vakuitaet: Ein Gate, das nichts angesehen hat, ist gruen und wertlos.
	// Der Nenner gehoert deshalb in die Ausgabe UND in eine Zusicherung.
	t.Logf("geprueft: %d Dateien in %d Paketen, %d Asynq-Verbindungsaufrufe",
		filesScanned, len(scannedDirs), callsChecked)
	for _, k := range []findingKind{kindDerivation, kindDerivedVar, kindPassThrough, kindZeroLiteral} {
		t.Logf("  %-28s %d", k+":", counts[k])
	}
	for _, dir := range scannedDirs {
		require.NotZero(t, perDirFiles[dir], "Paket %s hat keine Go-Datei geliefert", dir)
	}
	require.Greater(t, callsChecked, 0,
		"kein einziger Asynq-Verbindungsaufruf gefunden — das Gate prueft nichts")
	require.Greater(t, counts[kindDerivation]+counts[kindDerivedVar], 0,
		"keine einzige Verbindung stammt aus redisopt — der Scanner findet die "+
			"Ableitung nicht, statt dass niemand sie benutzt")

	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("Asynq-Verbindungsoptionen werden an %d Stelle(n) von Hand gebaut, "+
			"statt aus internal/shared/redisopt zu kommen:\n\n%s\n\n"+
			"Warum das rot ist: eine handgebaute asynq.RedisClientOpt vergisst frueher "+
			"oder spaeter ein Feld. Genau so ist R1-14b-01 entstanden — DB fehlte an "+
			"vier Stellen, die Enqueue-Kette brach lautlos, und davor war es Password "+
			"(NOAUTH, S121-C3). Nutze redisopt.Asynq(opts) bzw. redisopt.AsynqFromURL(url).",
			len(violations), strings.Join(violations, "\n"))
	}
}

// checkFileForHandBuiltAsynqOpts prueft eine Datei und liefert Verstoesse,
// die Zaehler je Kategorie und die Zahl der geprueften Aufrufe.
func checkFileForHandBuiltAsynqOpts(
	fset *token.FileSet, file *ast.File, src, label string,
) (violations []string, counts map[findingKind]int, checked int) {
	counts = map[findingKind]int{}

	// Idents, die in dieser Datei aus einem redisopt-Aufruf entstehen.
	derived := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Rhs) != 1 {
			return true
		}
		if !isRedisoptCall(as.Rhs[0]) {
			return true
		}
		for _, lhs := range as.Lhs {
			if id, ok := lhs.(*ast.Ident); ok {
				derived[id.Name] = true
			}
		}
		return true
	})

	pos := func(n ast.Node) string {
		p := fset.Position(n.Pos())
		return fmt.Sprintf("%s:%d", label, p.Line)
	}

	// Rekursiv durch die Funktionen laufen, damit die Parameter der UMGEBENDEN
	// Funktion bekannt sind. Eine Closure erbt die Parameter ihres Erzeugers —
	// deshalb wird die Menge beim Abstieg vereinigt, nicht ersetzt.
	var walk func(body ast.Node, params map[string]bool)
	walk = func(body ast.Node, params map[string]bool) {
		ast.Inspect(body, func(n ast.Node) bool {
			if fl, ok := n.(*ast.FuncLit); ok && fl.Body != body {
				inner := unionOptParams(params, fl.Type)
				walk(fl.Body, inner)
				return false // die Closure ist mit ihren eigenen Parametern behandelt
			}

			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 || !isAsynqConnConstructor(call.Fun) {
				return true
			}
			checked++

			arg := call.Args[0]
			switch a := arg.(type) {
			case *ast.CallExpr:
				if isRedisoptCall(a) {
					counts[kindDerivation]++
					return true
				}
			case *ast.Ident:
				if derived[a.Name] {
					counts[kindDerivedVar]++
					return true
				}
				if params[a.Name] {
					// Durchgereicht: der Wert stammt vom Aufrufer, und der
					// Aufrufer wird selbst von diesem Gate geprueft.
					counts[kindPassThrough]++
					return true
				}
			case *ast.IndexExpr:
				// Variadischer Parameter, z. B. asynqOpt[0] in vaktaware.
				if id, ok := a.X.(*ast.Ident); ok && params[id.Name] {
					counts[kindPassThrough]++
					return true
				}
			case *ast.CompositeLit:
				if isAsynqOptLiteral(a) && len(a.Elts) == 0 {
					// Das dokumentierte "kein Redis"-Null-Literal. Es baut keine
					// Verbindung, sondern schaltet sie ab — es kann kein Feld
					// verlieren, weil es keins setzt.
					counts[kindZeroLiteral]++
					return true
				}
			}

			violations = append(violations, fmt.Sprintf(
				"  %s: %s bekommt seine Verbindungsoptionen nicht aus redisopt",
				pos(call), constructorName(call.Fun)))
			return true
		})
	}

	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		walk(fd.Body, unionOptParams(nil, fd.Type))
	}

	// Zweite, unabhaengige Sicht: ein befuelltes Literal irgendwo in der Datei
	// ist auch dann falsch, wenn es nicht direkt an einen Konstruktor geht —
	// es wandert sonst ueber eine Variable oder einen Service-Konstruktor
	// dorthin. Ausgenommen ist redisopt selbst (dort MUSS die Struktur
	// entstehen) und jede Stelle mit einer begruendeten redisdb-ok-Notiz.
	if !strings.HasPrefix(label, "internal/shared/redisopt/") {
		lines := strings.Split(src, "\n")
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok || !isAsynqOptLiteral(lit) || len(lit.Elts) == 0 {
				return true
			}
			line := fset.Position(lit.Pos()).Line
			if hasMarkerAbove(lines, line, "redisdb-ok") {
				return true
			}
			violations = append(violations, fmt.Sprintf(
				"  %s: befuelltes asynq.RedisClientOpt-Literal ausserhalb von redisopt",
				pos(lit)))
			return true
		})
	}

	return violations, counts, checked
}

// unionOptParams liefert base plus alle Parameter von ft, die vom Typ
// asynq.RedisClientOpt (auch variadisch) sind.
func unionOptParams(base map[string]bool, ft *ast.FuncType) map[string]bool {
	out := map[string]bool{}
	for k := range base {
		out[k] = true
	}
	if ft == nil || ft.Params == nil {
		return out
	}
	for _, field := range ft.Params.List {
		typ := field.Type
		if ell, ok := typ.(*ast.Ellipsis); ok {
			typ = ell.Elt
		}
		if !isAsynqOptType(typ) {
			continue
		}
		for _, name := range field.Names {
			out[name.Name] = true
		}
	}
	return out
}

func isAsynqOptType(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "asynq" &&
		(sel.Sel.Name == "RedisClientOpt" || sel.Sel.Name == "RedisConnOpt")
}

func isAsynqOptLiteral(lit *ast.CompositeLit) bool {
	return lit.Type != nil && isAsynqOptType(lit.Type)
}

func isAsynqConnConstructor(fun ast.Expr) bool {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "asynq" && asynqConnConstructors[sel.Sel.Name]
}

func constructorName(fun ast.Expr) string {
	if sel, ok := fun.(*ast.SelectorExpr); ok {
		return "asynq." + sel.Sel.Name
	}
	return "asynq-Konstruktor"
}

func isRedisoptCall(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "redisopt"
}

// hasMarkerAbove sucht eine Begruendungsnotiz in den Zeilen oberhalb (Muster
// des vorhandenen redisauth-ok aus scripts/check_outbound_security.py).
func hasMarkerAbove(lines []string, line int, marker string) bool {
	start := line - 8
	if start < 1 {
		start = 1
	}
	for i := start; i <= line && i <= len(lines); i++ {
		if strings.Contains(lines[i-1], marker) {
			return true
		}
	}
	return false
}

// TestAsynqScanCoversEveryConstructionSite haelt scannedDirs ehrlich.
//
// Ohne diesen Test waere die Liste oben eine Behauptung: Ein neues Paket, das
// eine Asynq-Verbindung baut, wuerde vom Gate schlicht nicht angesehen, und das
// Gate bliebe gruen — die Teilmengen-Falle, an der ValidateUUIDParams schon
// einmal haengengeblieben ist ("ich habe stichprobenartig geprueft, der Rest ist
// sauber" war eine Vermutung, und sie war falsch).
//
// Der Suchlauf geht ueber den GESAMTEN Backend-Baum, nicht ueber scannedDirs.
func TestAsynqScanCoversEveryConstructionSite(t *testing.T) {
	covered := map[string]bool{}
	for _, d := range scannedDirs {
		covered[filepath.Clean(d)] = true
	}

	fset := token.NewFileSet()
	var uncovered []string
	filesWalked := 0

	err := filepath.Walk(backendRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Weder Fremdcode noch generierter sqlc-Code bauen Asynq-Verbindungen.
			if name := info.Name(); name == "vendor" || name == "node_modules" || name == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		filesWalked++

		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return nil
		}
		found := false
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok && isAsynqConnConstructor(call.Fun) {
				found = true
				return false
			}
			return true
		})
		if !found {
			return nil
		}
		rel, rerr := filepath.Rel(backendRoot, filepath.Dir(path))
		if rerr != nil {
			return nil
		}
		if !covered[filepath.Clean(rel)] {
			uncovered = append(uncovered, filepath.Clean(rel))
		}
		return nil
	})
	require.NoError(t, err)

	t.Logf("Repo-weiter Suchlauf: %d Go-Dateien, %d abgedeckte Pakete", filesWalked, len(scannedDirs))
	require.Greater(t, filesWalked, 0, "der Suchlauf hat keine Datei gesehen")

	sort.Strings(uncovered)
	uncovered = dedupe(uncovered)
	assert.Empty(t, uncovered,
		"diese Pakete bauen eine Asynq-Verbindung, stehen aber nicht in scannedDirs — "+
			"das Gate sieht sie nicht an und meldet trotzdem OK: %v", uncovered)
}

func dedupe(in []string) []string {
	if len(in) == 0 {
		return in
	}
	out := in[:1]
	for _, s := range in[1:] {
		if s != out[len(out)-1] {
			out = append(out, s)
		}
	}
	return out
}
