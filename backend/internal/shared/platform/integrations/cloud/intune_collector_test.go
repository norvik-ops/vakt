// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package cloud

import (
	"testing"

	"github.com/matharnica/vakt/internal/shared/httputil"
)

func TestParseManagedDevices(t *testing.T) {
	body := []byte(`{"value":[
		{"deviceName":"LAPTOP-1","complianceState":"compliant","operatingSystem":"Windows","osVersion":"10.0.22631","isEncrypted":true,"lastSyncDateTime":"2026-06-15T08:00:00Z"},
		{"deviceName":"LAPTOP-2","complianceState":"noncompliant","operatingSystem":"Windows","osVersion":"10.0.19045","isEncrypted":false,"lastSyncDateTime":"2026-06-10T08:00:00Z"},
		{"deviceName":"IPHONE-3","complianceState":"compliant","operatingSystem":"iOS","osVersion":"18.1","isEncrypted":true,"lastSyncDateTime":"2026-06-15T07:00:00Z"}
	]}`)
	devices, err := parseManagedDevices(body)
	if err != nil {
		t.Fatalf("parseManagedDevices: %v", err)
	}
	if len(devices) != 3 {
		t.Fatalf("expected 3 devices, got %d", len(devices))
	}
	if devices[0].DeviceName != "LAPTOP-1" || !devices[0].IsEncrypted {
		t.Errorf("device[0] parsed wrong: %+v", devices[0])
	}
}

func TestParseManagedDevices_Empty(t *testing.T) {
	devices, err := parseManagedDevices([]byte(`{"value":[]}`))
	if err != nil {
		t.Fatalf("empty parse: %v", err)
	}
	if len(devices) != 0 {
		t.Errorf("expected 0 devices, got %d", len(devices))
	}
	p := computePosture(devices)
	if p.Total != 0 || p.CompliancePct != 0 || p.EncryptionPct != 0 {
		t.Errorf("empty posture should be all zero: %+v", p)
	}
}

func TestComputePosture(t *testing.T) {
	devices := []managedDevice{
		{ComplianceState: "compliant", IsEncrypted: true},
		{ComplianceState: "noncompliant", IsEncrypted: false},
		{ComplianceState: "compliant", IsEncrypted: true},
		{ComplianceState: "compliant", IsEncrypted: false},
	}
	p := computePosture(devices)
	if p.Total != 4 {
		t.Fatalf("total = %d, want 4", p.Total)
	}
	if p.Compliant != 3 || p.NonCompliant != 1 {
		t.Errorf("compliance counts wrong: %+v", p)
	}
	if p.Encrypted != 2 {
		t.Errorf("encrypted count = %d, want 2", p.Encrypted)
	}
	if p.CompliancePct != 75 {
		t.Errorf("compliance pct = %.1f, want 75", p.CompliancePct)
	}
	if p.EncryptionPct != 50 {
		t.Errorf("encryption pct = %.1f, want 50", p.EncryptionPct)
	}
}

// TestParseManagedDevices_Malformed ensures hostile/garbage input errors cleanly.
func TestParseManagedDevices_Malformed(t *testing.T) {
	if _, err := parseManagedDevices([]byte(`not json`)); err == nil {
		t.Error("expected error on malformed JSON")
	}
}

// TestIntuneOutboundGuard documents that the Graph endpoint passes the SSRF
// guard while a private/IMDS address would be rejected (S88-7 AC).
func TestIntuneOutboundGuard(t *testing.T) {
	if err := httputil.ValidateOutboundURL(intuneGraphBaseURL, false); err != nil {
		t.Errorf("public graph endpoint must pass outbound guard: %v", err)
	}
	if err := httputil.ValidateOutboundURL("http://169.254.169.254/latest/meta-data/", false); err == nil {
		t.Error("IMDS address must be rejected by outbound guard")
	}
	if err := httputil.ValidateOutboundURL("http://10.0.0.5/", false); err == nil {
		t.Error("RFC1918 address must be rejected by outbound guard")
	}
}
