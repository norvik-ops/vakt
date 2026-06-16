// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package docxexport

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"
	"time"
)

// openDocx unzips a .docx and returns its parts; fails the test on any problem.
func openDocx(t *testing.T, data []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("not a valid zip: %v", err)
	}
	out := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		out[f.Name] = string(b)
	}
	return out
}

// assertWellFormed parses each XML part to prove there is no malformed markup
// (which would make Word prompt to repair).
func assertWellFormed(t *testing.T, parts map[string]string) {
	t.Helper()
	for name, content := range parts {
		dec := xml.NewDecoder(strings.NewReader(content))
		for {
			_, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("%s is not well-formed XML: %v", name, err)
			}
		}
	}
}

func TestDocument_MinimalValidStructure(t *testing.T) {
	d := New()
	d.Heading("Title").Paragraph("Hello <world> & \"friends\"").Table(
		[]string{"A", "B"}, [][]string{{"1", "2"}, {"3"}},
	)
	data, err := d.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	parts := openDocx(t, data)

	for _, required := range []string{"[Content_Types].xml", "_rels/.rels", "word/document.xml"} {
		if _, ok := parts[required]; !ok {
			t.Errorf("missing required part %q", required)
		}
	}
	assertWellFormed(t, parts)

	doc := parts["word/document.xml"]
	if !strings.Contains(doc, "Title") {
		t.Error("heading text missing from document")
	}
	// Special chars must be escaped, not raw.
	if strings.Contains(doc, "<world>") {
		t.Error("unescaped angle brackets leaked into the document XML")
	}
	if !strings.Contains(doc, "&lt;world&gt;") {
		t.Error("expected escaped angle brackets")
	}
}

func TestRenderSoA_Valid(t *testing.T) {
	rows := []SoARow{
		{ControlRef: "A.5.1", ControlName: "Policies for information security", ControlGroup: "Organizational",
			Applicable: true, ImplementationStatus: "implemented", Owner: "CISO", Justification: "Required", UpdatedAt: time.Now()},
		{ControlRef: "A.7.4", ControlName: "Physical security monitoring", ControlGroup: "Physical",
			Applicable: false, Justification: "No premises"},
	}
	data, err := RenderSoA(rows, SoASummary{ApplicableCount: 1, ExcludedCount: 1, ImplementedCount: 1, ImplementationPct: 50})
	if err != nil {
		t.Fatalf("RenderSoA: %v", err)
	}
	parts := openDocx(t, data)
	assertWellFormed(t, parts)
	if !strings.Contains(parts["word/document.xml"], "A.5.1") {
		t.Error("SoA control ref missing")
	}
}

func TestRenderRisiken_Valid(t *testing.T) {
	residual := 4
	due := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	rows := []RiskRow{
		{ID: "1", Title: "Ransomware", Category: "Malware", Likelihood: 4, Impact: 5, RiskScore: 20,
			Treatment: "mitigate", Status: "open", Owner: "IT", DueDate: &due, ResidualScore: &residual},
	}
	data, err := RenderRisiken(rows)
	if err != nil {
		t.Fatalf("RenderRisiken: %v", err)
	}
	parts := openDocx(t, data)
	assertWellFormed(t, parts)
	doc := parts["word/document.xml"]
	if !strings.Contains(doc, "Ransomware") || !strings.Contains(doc, "20") {
		t.Error("risk row content missing")
	}
}

func TestRenderRisiken_Empty(t *testing.T) {
	data, err := RenderRisiken(nil)
	if err != nil {
		t.Fatalf("empty RenderRisiken: %v", err)
	}
	assertWellFormed(t, openDocx(t, data))
}
