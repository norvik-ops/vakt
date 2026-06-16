// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package audit

import (
	"bufio"
	"net"
	"strings"
	"testing"
	"time"
)

func TestFormatRFC5424(t *testing.T) {
	e := WriteEntry{
		OrgID: "org-1", UserID: "user-1", Action: "import",
		ResourceType: "vakt-comply/verinice-import", ResourceID: "r1",
		ResourceName: "verinice .vna", IPAddress: "203.0.113.5",
		CreatedAt: time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC),
	}
	msg := formatRFC5424(e)
	if !strings.HasPrefix(msg, "<134>1 2026-06-15T10:00:00Z vakt vakt-audit - AUDIT ") {
		t.Errorf("RFC5424 prefix wrong: %q", msg)
	}
	if !strings.Contains(msg, `org="org-1"`) || !strings.Contains(msg, `action="import"`) {
		t.Errorf("RFC5424 structured data missing fields: %q", msg)
	}
	if !strings.HasSuffix(msg, "\n") {
		t.Error("syslog line must end with newline")
	}
}

func TestFormatCEF(t *testing.T) {
	e := WriteEntry{OrgID: "org-1", UserID: "u", Action: "delete", ResourceType: "control", ResourceID: "c1"}
	msg := formatCEF(e)
	if !strings.HasPrefix(msg, "CEF:0|NorvikOps|Vakt|1.0|") {
		t.Errorf("CEF header wrong: %q", msg)
	}
	if !strings.Contains(msg, "cs1=org-1") {
		t.Errorf("CEF org extension missing: %q", msg)
	}
}

func TestEscapeSD(t *testing.T) {
	if got := escapeSD(`a"b]c\d`); got != `a\"b\]c\\d` {
		t.Errorf("escapeSD = %q", got)
	}
}

func TestNewSyslogForwarder_DisabledWhenNoTarget(t *testing.T) {
	f, err := NewSyslogForwarder(SyslogForwarderConfig{Target: ""})
	if err != nil || f != nil {
		t.Errorf("empty target must yield (nil,nil), got (%v,%v)", f, err)
	}
}

func TestNewSyslogForwarder_RejectsPrivateTarget(t *testing.T) {
	_, err := NewSyslogForwarder(SyslogForwarderConfig{Target: "127.0.0.1:514", AllowPrivate: false})
	if err == nil {
		t.Error("loopback target must be rejected when AllowPrivate=false")
	}
}

func TestForward_BackpressureDrops(t *testing.T) {
	resetForwardCounters()
	// Tiny buffer, no worker draining (we never start one here): fill then overflow.
	f := &SyslogForwarder{target: "x:514", format: "rfc5424", ch: make(chan WriteEntry, 1)}
	f.Forward(WriteEntry{Action: "a"}) // buffered
	f.Forward(WriteEntry{Action: "b"}) // dropped (full)
	f.Forward(WriteEntry{Action: "c"}) // dropped (full)
	_, dropped, _ := ForwardStats()
	if dropped != 2 {
		t.Errorf("expected 2 drops, got %d", dropped)
	}
}

// TestSyslogForwarder_DeliversToLocalSink is an end-to-end check against a real
// local TCP listener (no Docker needed). Verifies the RFC-5424 line is received.
func TestSyslogForwarder_DeliversToLocalSink(t *testing.T) {
	resetForwardCounters()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	received := make(chan string, 1)
	go func() {
		conn, aErr := ln.Accept()
		if aErr != nil {
			return
		}
		defer conn.Close()
		line, _ := bufio.NewReader(conn).ReadString('\n')
		received <- line
	}()

	// AllowPrivate=true because the sink is on loopback.
	f, err := NewSyslogForwarder(SyslogForwarderConfig{
		Target: ln.Addr().String(), Proto: "tcp", Format: "rfc5424", AllowPrivate: true, BufferSize: 8,
	})
	if err != nil {
		t.Fatalf("new forwarder: %v", err)
	}
	if f == nil {
		t.Fatal("forwarder should be enabled")
	}

	f.Forward(WriteEntry{OrgID: "org-1", Action: "create", ResourceType: "risk", ResourceID: "r1"})

	select {
	case line := <-received:
		if !strings.Contains(line, "AUDIT") || !strings.Contains(line, `action="create"`) {
			t.Errorf("unexpected syslog line: %q", line)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for syslog delivery")
	}
}

func resetForwardCounters() {
	forwardSent.Store(0)
	forwardDropped.Store(0)
	forwardFailed.Store(0)
}
