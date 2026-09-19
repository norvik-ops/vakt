// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package scheduledreports

import (
	"bytes"
	"errors"
	"testing"
)

// R1-W8C-N1(b): when the findings CSV cannot be built, RunReport must abort
// delivery instead of sending a bare-header "empty" report. findingsAttachment
// is the decision seam: a build error becomes a returned error (→ RunReport
// returns before the send loop), and there is no fallback attachment.
func TestFindingsAttachment_BuildErrorAborts(t *testing.T) {
	af, err := findingsAttachment(nil, errors.New("query findings: connection reset"))
	if err == nil {
		t.Fatal("expected an error so delivery is aborted, got nil")
	}
	if af != nil {
		t.Fatal("no attachment closure must be returned on build failure")
	}
}

// The normal path is unchanged: the closure yields exactly the built bytes under
// findings.csv, and specifically does NOT substitute a header-only placeholder.
func TestFindingsAttachment_SuccessYieldsBuiltBytes(t *testing.T) {
	want := []byte("id,title\nabc,Something\n")
	af, err := findingsAttachment(want, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if af == nil {
		t.Fatal("expected an attachment closure on success")
	}
	got, name, aerr := af()
	if aerr != nil {
		t.Fatalf("attachment closure returned error: %v", aerr)
	}
	if name != "findings.csv" {
		t.Fatalf("attachment name = %q, want findings.csv", name)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("attachment bytes = %q, want %q", got, want)
	}
}
