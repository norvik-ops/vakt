// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktaware

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestDocumentedJobsExist is the regression guard for R1-04-L07.
//
// Das öffentliche Wiki nannte einen täglichen Job `vaktaware:training_reminder`,
// den es im Backend nicht gibt: Handler, Payload und Mail-Funktion wurden
// bewusst entfernt (docs/dev/dead-worker-chains.md, W2), die Job-Tabelle blieb
// stehen. Umgekehrt fehlte `aware:auto_enrollment`, den es gibt.
//
// Der Test hält keine Liste. Er liest die Task-Konstanten aus dem Paketquelltext
// und die Job-Tabellen aus der Doku und vergleicht beide Richtungen — ein neuer
// Job ohne Doku fällt genauso auf wie ein Doku-Eintrag ohne Job.
func TestDocumentedJobsExist(t *testing.T) {
	code := taskConstantsInPackage(t)
	if len(code) == 0 {
		t.Fatal("keine Task-Konstanten im Paket gefunden — der Leser ist kaputt, nicht die Doku")
	}

	for _, doc := range []string{
		"../../../../docs/modules/vaktaware.md",
		"../../../../docs/wiki/modules/aware.md",
	} {
		t.Run(filepath.Base(doc), func(t *testing.T) {
			documented := jobsInDoc(t, doc)
			if len(documented) == 0 {
				t.Fatalf("keine Job-Zeilen in %s gefunden — die Tabelle ist weg oder das Muster passt nicht", doc)
			}
			for _, name := range documented {
				if !code[name] {
					t.Errorf("%s nennt den Job %q, den das Backend nicht kennt (bekannt: %v)",
						doc, name, sortedKeys(code))
				}
			}
			for name := range code {
				found := false
				for _, d := range documented {
					if d == name {
						found = true
					}
				}
				if !found {
					t.Errorf("%s nennt den Job %q nicht, den das Backend ausführt", doc, name)
				}
			}
		})
	}
}

// taskConstantsInPackage sammelt jede Konstante, deren Name mit „Task" beginnt
// und deren Wert wie ein Asynq-Task-Name aussieht (`bereich:name`).
func taskConstantsInPackage(t *testing.T) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	fset := token.NewFileSet()
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("Quelldateien suchen: %v", err)
	}
	for _, src := range sources {
		if strings.HasSuffix(src, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, src, nil, 0)
		if err != nil {
			t.Fatalf("%s lesen: %v", src, err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range vs.Names {
					if !strings.HasPrefix(name.Name, "Task") || i >= len(vs.Values) {
						continue
					}
					lit, ok := vs.Values[i].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					v, err := strconv.Unquote(lit.Value)
					if err == nil && strings.Contains(v, ":") {
						out[v] = true
					}
				}
			}
		}
	}
	return out
}

// jobRow matcht die erste Backtick-Zelle einer Markdown-Tabellenzeile, sofern
// sie wie ein Task-Name aussieht.
var jobRow = regexp.MustCompile("(?m)^\\|\\s*`([a-z_]+:[a-z_]+)`\\s*\\|")

func jobsInDoc(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Doku lesen: %v", err)
	}
	var out []string
	for _, m := range jobRow.FindAllStringSubmatch(string(b), -1) {
		out = append(out, m[1])
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
