// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package docxexport builds minimal, valid Word (.docx / OOXML) documents using
// only the Go standard library — no external library and no external service, so
// it stays fully self-hosted (S89-6). A .docx is a ZIP of XML parts; we emit the
// three parts a word processor needs to open the file without a repair prompt:
// [Content_Types].xml, _rels/.rels and word/document.xml.
package docxexport

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
)

const (
	// ContentType is the IANA media type for a Word document.
	ContentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

	contentTypesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`

	relsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`

	docNS = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"`
)

// Document accumulates body content and renders it to a .docx byte slice.
type Document struct {
	body strings.Builder
}

// New returns an empty document.
func New() *Document { return &Document{} }

// escape XML-escapes text for use inside an element body.
func escape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	// Strip control chars that are illegal in XML 1.0 (except tab/newline).
	var b strings.Builder
	for _, r := range s {
		if r == '\t' || r == '\n' || r >= 0x20 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func run(text string, bold bool) string {
	rpr := ""
	if bold {
		rpr = `<w:rPr><w:b/></w:rPr>`
	}
	return fmt.Sprintf(`<w:r>%s<w:t xml:space="preserve">%s</w:t></w:r>`, rpr, escape(text))
}

// Heading appends a bold paragraph (no styles.xml needed to render).
func (d *Document) Heading(text string) *Document {
	d.body.WriteString(`<w:p>` + run(text, true) + `</w:p>`)
	return d
}

// Paragraph appends a normal text paragraph.
func (d *Document) Paragraph(text string) *Document {
	d.body.WriteString(`<w:p>` + run(text, false) + `</w:p>`)
	return d
}

// Table appends a bordered table. The header row is bold.
func (d *Document) Table(headers []string, rows [][]string) *Document {
	var b strings.Builder
	b.WriteString(`<w:tbl><w:tblPr><w:tblW w:w="0" w:type="auto"/><w:tblBorders>`)
	for _, edge := range []string{"top", "left", "bottom", "right", "insideH", "insideV"} {
		b.WriteString(fmt.Sprintf(`<w:%s w:val="single" w:sz="4" w:space="0" w:color="auto"/>`, edge))
	}
	b.WriteString(`</w:tblBorders></w:tblPr>`)

	cell := func(text string, bold bool) string {
		return `<w:tc><w:tcPr><w:tcW w:w="0" w:type="auto"/></w:tcPr><w:p>` + run(text, bold) + `</w:p></w:tc>`
	}

	b.WriteString(`<w:tr>`)
	for _, h := range headers {
		b.WriteString(cell(h, true))
	}
	b.WriteString(`</w:tr>`)
	for _, r := range rows {
		b.WriteString(`<w:tr>`)
		for i := range headers {
			val := ""
			if i < len(r) {
				val = r[i]
			}
			b.WriteString(cell(val, false))
		}
		b.WriteString(`</w:tr>`)
	}
	b.WriteString(`</w:tbl>`)
	// A trailing empty paragraph keeps Word happy after a table.
	b.WriteString(`<w:p/>`)
	d.body.WriteString(b.String())
	return d
}

// Bytes renders the document as a .docx (ZIP) byte slice.
func (d *Document) Bytes() ([]byte, error) {
	documentXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:document ` + docNS + `><w:body>` + d.body.String() +
		`<w:sectPr/></w:body></w:document>`

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	parts := []struct {
		name, content string
	}{
		{"[Content_Types].xml", contentTypesXML},
		{"_rels/.rels", relsXML},
		{"word/document.xml", documentXML},
	}
	for _, p := range parts {
		w, err := zw.Create(p.name)
		if err != nil {
			return nil, fmt.Errorf("docx zip create %s: %w", p.name, err)
		}
		if _, err := w.Write([]byte(p.content)); err != nil {
			return nil, fmt.Errorf("docx zip write %s: %w", p.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("docx zip close: %w", err)
	}
	return buf.Bytes(), nil
}
