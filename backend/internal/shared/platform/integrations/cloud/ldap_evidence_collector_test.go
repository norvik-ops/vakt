// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package cloud

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- windowsFiletimeToTime unit tests ---

func TestWindowsFiletimeToTime_KnownValues(t *testing.T) {
	// Windows FILETIME for 2000-01-01T00:00:00Z
	// Ticks from 1601-01-01 to 2000-01-01 = 125911584000000000
	const ticks2000 int64 = 125_911_584_000_000_000
	got := windowsFiletimeToTime(ticks2000)

	assert.Equal(t, 2000, got.Year())
	assert.Equal(t, time.January, got.Month())
	assert.Equal(t, 1, got.Day())
}

func TestWindowsFiletimeToTime_Zero(t *testing.T) {
	got := windowsFiletimeToTime(0)
	assert.True(t, got.IsZero(), "zero filetime should return zero time")
}

func TestWindowsFiletimeToTime_Negative(t *testing.T) {
	got := windowsFiletimeToTime(-1)
	assert.True(t, got.IsZero(), "negative filetime should return zero time")
}

// --- LDAPEvidenceCollector integration tests via connect-DI ---
// The LDAPEvidenceCollector.connect() calls ldaplib.DialURL which requires a real TCP connection.
// We test the evidence assembly logic by exercising the collect* methods with a stubbed
// ldap.Conn-compatible connection. Because ldap.Conn is a struct (not an interface), we instead
// validate the collector's error handling when the connect step fails.

func TestLDAPEvidenceCollector_ConnectError(t *testing.T) {
	ew := &mockEvidenceWriter{}
	collector := &LDAPEvidenceCollector{evidence: ew}

	_, err := collector.Collect(context.Background(), "org-1", LDAPConfig{
		Host:         "127.0.0.1",
		Port:         1389, // nothing listening here
		BindDN:       "cn=admin,dc=test,dc=local",
		BaseDN:       "dc=test,dc=local",
		BindPassword: "password",
		UseTLS:       false,
	})

	assert.Error(t, err, "expected error when LDAP host is unreachable")
	assert.Contains(t, err.Error(), "ldap connect")
}

func TestLDAPEvidenceCollector_ConnectError_TLS(t *testing.T) {
	ew := &mockEvidenceWriter{}
	collector := &LDAPEvidenceCollector{evidence: ew}

	_, err := collector.Collect(context.Background(), "org-1", LDAPConfig{
		Host:         "127.0.0.1",
		Port:         1636,
		BindDN:       "cn=admin,dc=test,dc=local",
		BaseDN:       "dc=test,dc=local",
		BindPassword: "password",
		UseTLS:       true,
	})

	assert.Error(t, err, "expected error when LDAPS host is unreachable")
}
