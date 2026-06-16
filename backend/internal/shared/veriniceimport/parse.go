// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package veriniceimport parses the verinice ".vna" exchange format (a ZIP
// containing an SNCA sync XML) into normalized objects that can be mapped onto
// Vakt entities. Parsing is defensive: hard size limits, no external entity
// resolution (Go's encoding/xml never resolves external entities, so XXE is not
// possible), and a streaming token reader that tolerates unknown namespaces and
// malformed fragments without panicking. S88-4.
package veriniceimport

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

const (
	// MaxArchiveSize bounds the uncompressed .vna payload we will read (50 MiB).
	MaxArchiveSize = 50 << 20
	// MaxXMLSize bounds a single decompressed XML entry (40 MiB).
	MaxXMLSize = 40 << 20
	// MaxObjects bounds the number of sync objects parsed (zip-bomb / DoS guard).
	MaxObjects = 100_000
)

// ImportObject is one normalized SNCA object.
type ImportObject struct {
	Type       string            // raw extObjectType, lower-cased
	ExtID      string            // verinice external id
	Title      string            // best-effort human title
	Properties map[string]string // flattened syncAttributes (last value wins)
	Category   string            // classified: asset | control | risk | unmapped
}

// syncAttribute / syncObject mirror the SNCA sync XML. Tags use local names only
// so any namespace prefix (ns3:, ns2:, none) matches.
type xmlAttribute struct {
	Name   string   `xml:"name"`
	Values []string `xml:"value"`
}

type xmlObject struct {
	ExtObjectType string         `xml:"extObjectType,attr"`
	ExtID         string         `xml:"extId,attr"`
	Attributes    []xmlAttribute `xml:"syncAttribute"`
}

// classify maps a raw extObjectType to a Vakt category.
func classify(rawType string) string {
	t := strings.ToLower(rawType)
	switch {
	case strings.Contains(t, "scenario"), strings.Contains(t, "szenario"),
		strings.Contains(t, "incident"), strings.Contains(t, "vulnerability"),
		strings.Contains(t, "threat"), strings.Contains(t, "gefaehrdung"),
		strings.Contains(t, "risk"):
		return "risk"
	case strings.Contains(t, "control"), strings.Contains(t, "safeguard"),
		strings.Contains(t, "massnahme"), strings.Contains(t, "baustein"),
		strings.Contains(t, "anforderung"), strings.Contains(t, "control_group"):
		return "control"
	case strings.Contains(t, "asset"), strings.Contains(t, "anlage"),
		strings.Contains(t, "system"), strings.Contains(t, "device"),
		strings.Contains(t, "server"), strings.Contains(t, "network"),
		strings.Contains(t, "application"):
		return "asset"
	default:
		return "unmapped"
	}
}

// bestTitle picks a human-readable title from the object's properties.
func bestTitle(props map[string]string, extID string) string {
	// Prefer keys that look like a name/title.
	for _, k := range []string{"title", "name"} {
		for pk, pv := range props {
			lk := strings.ToLower(pk)
			if pv != "" && (lk == k || strings.HasSuffix(lk, "_"+k) || strings.Contains(lk, k)) {
				return pv
			}
		}
	}
	// Fall back to the first non-empty property value, then extId.
	for _, pv := range props {
		if pv != "" {
			return pv
		}
	}
	return extID
}

// ParseVNA reads a .vna archive (ZIP) and returns the normalized SNCA objects.
// The reader is bounded to MaxArchiveSize.
func ParseVNA(r io.Reader, size int64) ([]ImportObject, error) {
	if size <= 0 || size > MaxArchiveSize {
		// Read with a hard cap when size is unknown/oversized.
		buf, err := io.ReadAll(io.LimitReader(r, MaxArchiveSize+1))
		if err != nil {
			return nil, fmt.Errorf("read archive: %w", err)
		}
		if int64(len(buf)) > MaxArchiveSize {
			return nil, fmt.Errorf("archive exceeds %d bytes", MaxArchiveSize)
		}
		return parseVNABytes(buf)
	}
	buf, err := io.ReadAll(io.LimitReader(r, size))
	if err != nil {
		return nil, fmt.Errorf("read archive: %w", err)
	}
	return parseVNABytes(buf)
}

func parseVNABytes(buf []byte) ([]ImportObject, error) {
	zr, err := zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		return nil, fmt.Errorf("open .vna (not a valid ZIP): %w", err)
	}
	var objects []ImportObject
	for _, f := range zr.File {
		if !strings.HasSuffix(strings.ToLower(f.Name), ".xml") {
			continue
		}
		objs, err := parseXMLEntry(f)
		if err != nil {
			// Skip a malformed entry rather than failing the whole import.
			continue
		}
		objects = append(objects, objs...)
		if len(objects) >= MaxObjects {
			break
		}
	}
	return objects, nil
}

func parseXMLEntry(f *zip.File) ([]ImportObject, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return ParseXML(io.LimitReader(rc, MaxXMLSize))
}

// ParseXML streams an SNCA sync XML and extracts every <syncObject> regardless
// of namespace or nesting depth. Exposed for fuzzing and unit tests.
func ParseXML(r io.Reader) ([]ImportObject, error) {
	dec := xml.NewDecoder(r)
	// Defence-in-depth: Go does not resolve external entities, but we also
	// refuse any custom entity map and keep the charset reader nil.
	dec.Strict = false
	dec.Entity = map[string]string{}

	var objects []ImportObject
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Tolerate malformed tail — return what we have so far.
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "syncObject" {
			continue
		}
		var obj xmlObject
		if err := dec.DecodeElement(&obj, &se); err != nil {
			continue
		}
		props := make(map[string]string, len(obj.Attributes))
		for _, a := range obj.Attributes {
			if a.Name == "" {
				continue
			}
			val := ""
			if len(a.Values) > 0 {
				val = strings.TrimSpace(a.Values[len(a.Values)-1])
			}
			props[a.Name] = val
		}
		objects = append(objects, ImportObject{
			Type:       strings.ToLower(strings.TrimSpace(obj.ExtObjectType)),
			ExtID:      strings.TrimSpace(obj.ExtID),
			Title:      bestTitle(props, strings.TrimSpace(obj.ExtID)),
			Properties: props,
			Category:   classify(obj.ExtObjectType),
		})
		if len(objects) >= MaxObjects {
			break
		}
	}
	return objects, nil
}

// Preview summarizes what an import would do, without writing anything.
type Preview struct {
	TotalObjects  int            `json:"total_objects"`
	Assets        int            `json:"assets"`
	Controls      int            `json:"controls"`
	Risks         int            `json:"risks"`
	Unmapped      int            `json:"unmapped"`
	UnmappedTypes []string       `json:"unmapped_types"`
	SampleTitles  map[string]any `json:"sample_titles"` // category -> []string (max 5)
}

// BuildPreview classifies objects into a Preview without DB access.
func BuildPreview(objects []ImportObject) Preview {
	p := Preview{TotalObjects: len(objects), SampleTitles: map[string]any{}}
	unmappedSet := map[string]bool{}
	samples := map[string][]string{}
	for _, o := range objects {
		switch o.Category {
		case "asset":
			p.Assets++
		case "control":
			p.Controls++
		case "risk":
			p.Risks++
		default:
			p.Unmapped++
			if o.Type != "" {
				unmappedSet[o.Type] = true
			}
		}
		if len(samples[o.Category]) < 5 && o.Title != "" {
			samples[o.Category] = append(samples[o.Category], o.Title)
		}
	}
	for t := range unmappedSet {
		p.UnmappedTypes = append(p.UnmappedTypes, t)
	}
	for cat, s := range samples {
		p.SampleTitles[cat] = s
	}
	return p
}
