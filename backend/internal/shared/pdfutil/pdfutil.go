// Package pdfutil centralises PDF document construction so that every generator
// in the platform renders German text (umlauts, ß, €, typographic quotes) correctly.
//
// Background (R1-20-A1): 15 of 16 fpdf generators wrote raw UTF-8 into fonts with
// /Encoding /WinAnsiEncoding, producing mojibake (Ã¼, ÃŸ, â€") in practically every
// deliverable — and the deliverable IS the product for a compliance tool. The single
// clean generator wrapped every string through a cp1252 UnicodeTranslator; that works
// for the Windows-1252 subset but cannot be centralised (it requires wrapping every
// call site) and silently re-breaks the moment someone forgets a wrap.
//
// This package instead registers a full UTF-8 TrueType face (DejaVu Sans) UNDER the
// core family name "Helvetica" for every style the generators use ("", "B", "I", "BI").
// Because fpdf looks up fonts by family+style, all existing SetFont("Helvetica", …)
// calls transparently pick up the embedded UTF-8 font — no per-call-site changes, and
// UTF-8 becomes the default that cannot be forgotten. Callers only swap fpdf.New(…) for
// pdfutil.New(…) (or call RegisterFonts on a NewCustom document).
//
// Known limitation: the italic style ("I") maps to the regular face because Debian's
// fonts-dejavu-core ships no DejaVuSans-Oblique. Italic is used only for subtle
// footers/captions, so it renders upright but fully legible. Follow-up: embed a real
// oblique if true italic is ever required.
package pdfutil

import (
	_ "embed"

	"github.com/go-pdf/fpdf"
)

//go:embed fonts/DejaVuSans.ttf
var dejaVuSansRegular []byte

//go:embed fonts/DejaVuSans-Bold.ttf
var dejaVuSansBold []byte

// fontFamily is the core family name the generators already pass to SetFont.
// Registering the embedded TTF under this exact name means no SetFont call has to change.
const fontFamily = "Helvetica"

// RegisterFonts registers the embedded DejaVu Sans faces under the "Helvetica"
// family for every style used across the generators, then selects a sane default.
// It must be called once per document before any text is written.
func RegisterFonts(pdf *fpdf.Fpdf) {
	pdf.AddUTF8FontFromBytes(fontFamily, "", dejaVuSansRegular)
	pdf.AddUTF8FontFromBytes(fontFamily, "B", dejaVuSansBold)
	// No oblique face available — map italic and bold-italic to the closest real face.
	pdf.AddUTF8FontFromBytes(fontFamily, "I", dejaVuSansRegular)
	pdf.AddUTF8FontFromBytes(fontFamily, "BI", dejaVuSansBold)
	pdf.SetFont(fontFamily, "", 10)
}

// New builds an A4 document (millimetre units) in the given orientation
// ("P" portrait / "L" landscape) with UTF-8 fonts already registered. It mirrors
// the fpdf.New(orientation, "mm", "A4", "") call the generators used.
func New(orientation string) *fpdf.Fpdf {
	pdf := fpdf.New(orientation, "mm", "A4", "")
	RegisterFonts(pdf)
	return pdf
}

// NewCustom builds a document from a full fpdf.InitType (for generators that need a
// non-default page setup) with UTF-8 fonts already registered.
func NewCustom(init *fpdf.InitType) *fpdf.Fpdf {
	pdf := fpdf.NewCustom(init)
	RegisterFonts(pdf)
	return pdf
}
