// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package veriniceimport

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

const sampleSNCA = `<?xml version="1.0" encoding="UTF-8"?>
<ns3:syncRequest xmlns:ns3="http://www.sernet.de/sync/data">
  <syncData>
    <syncObject extObjectType="asset" extId="ENTITY_1">
      <syncAttribute><name>asset_name</name><value>Webserver Produktion</value></syncAttribute>
    </syncObject>
    <syncObject extObjectType="control" extId="ENTITY_2">
      <syncAttribute><name>control_name</name><value>Zugriffskontrolle</value></syncAttribute>
    </syncObject>
    <syncObject extObjectType="incident_scenario" extId="ENTITY_3">
      <syncAttribute><name>incident_scenario_name</name><value>Ransomware-Befall</value></syncAttribute>
    </syncObject>
    <syncObject extObjectType="finance_record" extId="ENTITY_4">
      <syncAttribute><name>finance_name</name><value>Irrelevant</value></syncAttribute>
    </syncObject>
  </syncData>
</ns3:syncRequest>`

func TestParseXML_ClassifiesAndTitles(t *testing.T) {
	objs, err := ParseXML(strings.NewReader(sampleSNCA))
	if err != nil {
		t.Fatalf("ParseXML: %v", err)
	}
	if len(objs) != 4 {
		t.Fatalf("expected 4 objects, got %d", len(objs))
	}
	byCat := map[string]ImportObject{}
	for _, o := range objs {
		byCat[o.Category] = o
	}
	if byCat["asset"].Title != "Webserver Produktion" {
		t.Errorf("asset title = %q", byCat["asset"].Title)
	}
	if byCat["control"].Title != "Zugriffskontrolle" {
		t.Errorf("control title = %q", byCat["control"].Title)
	}
	if byCat["risk"].Title != "Ransomware-Befall" {
		t.Errorf("risk title = %q", byCat["risk"].Title)
	}
	if _, ok := byCat["unmapped"]; !ok {
		t.Error("finance_record should be unmapped")
	}
}

func TestBuildPreview(t *testing.T) {
	objs, _ := ParseXML(strings.NewReader(sampleSNCA))
	p := BuildPreview(objs)
	if p.TotalObjects != 4 || p.Assets != 1 || p.Controls != 1 || p.Risks != 1 || p.Unmapped != 1 {
		t.Fatalf("preview mismatch: %+v", p)
	}
	if len(p.UnmappedTypes) != 1 || p.UnmappedTypes[0] != "finance_record" {
		t.Errorf("unmapped types = %v", p.UnmappedTypes)
	}
}

func makeVNA(t *testing.T, xmlBody string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("verinice.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(xmlBody)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestParseVNA_FromZip(t *testing.T) {
	data := makeVNA(t, sampleSNCA)
	objs, err := ParseVNA(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("ParseVNA: %v", err)
	}
	p := BuildPreview(objs)
	if p.Assets != 1 || p.Controls != 1 || p.Risks != 1 {
		t.Fatalf("preview from zip mismatch: %+v", p)
	}
}

func TestParseVNA_NotAZip(t *testing.T) {
	_, err := ParseVNA(strings.NewReader("not a zip at all"), 16)
	if err == nil {
		t.Error("expected error for non-zip input")
	}
}

func TestParseXML_MalformedDoesNotPanic(t *testing.T) {
	inputs := []string{
		"", "<", "<syncObject", "<syncObject></syncObject>",
		"<a><syncObject extId='x'><syncAttribute><name>n</name></syncAttribute></syncObject>",
		"<!DOCTYPE foo [<!ENTITY xxe SYSTEM \"file:///etc/passwd\">]><r>&xxe;</r>",
	}
	for _, in := range inputs {
		_, _ = ParseXML(strings.NewReader(in)) // must not panic
	}
}

func TestParseXML_NoXXE(t *testing.T) {
	// An external entity reference must never be expanded / resolved.
	mal := `<?xml version="1.0"?>
<!DOCTYPE r [<!ENTITY xxe SYSTEM "file:///etc/passwd">]>
<ns:syncRequest xmlns:ns="x"><syncData>
<syncObject extObjectType="asset" extId="E1">
<syncAttribute><name>asset_name</name><value>&xxe;</value></syncAttribute>
</syncObject></syncData></ns:syncRequest>`
	objs, _ := ParseXML(strings.NewReader(mal))
	for _, o := range objs {
		if strings.Contains(o.Title, "root:") {
			t.Fatal("XXE: external entity was resolved")
		}
	}
}
