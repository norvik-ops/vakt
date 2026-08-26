package pdfutil

import (
	"bytes"
	"testing"

	"github.com/go-pdf/fpdf"
)

// TestNewRendersGermanWithoutMojibake proves the helper's core guarantee: text with
// umlauts, ß, € and typographic quotes written through the ordinary SetFont("Helvetica", …)
// call path lands as an embedded UTF-8 composite font, never as raw UTF-8 in a WinAnsi
// core font. /FontFile2 + /CIDFontType2 are absent in the garbled WinAnsi path and
// present only when AddUTF8Font is active.
func TestNewRendersGermanWithoutMojibake(t *testing.T) {
	pdf := New("P")
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 14)
	pdf.Cell(0, 10, "Größe: „Anführung“ – Prämie 5 € ü ö ä ß")
	pdf.SetFont("Helvetica", "I", 10)
	pdf.Cell(0, 10, "Fußzeile schräg")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		t.Fatalf("output: %v", err)
	}
	raw := buf.Bytes()

	if !bytes.Contains(raw, []byte("/FontFile2")) {
		t.Error("no embedded TrueType font (/FontFile2) — UTF-8 path not active")
	}
	if !bytes.Contains(raw, []byte("/CIDFontType2")) {
		t.Error("no composite UTF-8 font (/CIDFontType2) — UTF-8 path not active")
	}
}

// TestNewCustomRegistersFonts guards the landscape/NewCustom entry point too.
func TestNewCustomRegistersFonts(t *testing.T) {
	pdf := NewCustom(&fpdf.InitType{OrientationStr: "L", UnitStr: "mm", SizeStr: "A4"})
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 11)
	pdf.Cell(0, 10, "Übersicht — Datenschutz-Folgenabschätzung")
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		t.Fatalf("output: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("/FontFile2")) {
		t.Error("NewCustom did not embed a UTF-8 font")
	}
}
