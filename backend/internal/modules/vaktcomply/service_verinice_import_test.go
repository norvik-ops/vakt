// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import "testing"

func TestMapAssetType(t *testing.T) {
	cases := map[string]string{
		"asset":             "it_system",
		"network_asset":     "network",
		"application_asset": "application",
		"raum":              "room",
		"geschaeftsprozess": "process",
		"server_device":     "it_system",
	}
	for in, want := range cases {
		if got := mapAssetType(in); got != want {
			t.Errorf("mapAssetType(%q)=%q want %q", in, got, want)
		}
	}
}

func TestTruncateStr(t *testing.T) {
	if got := truncateStr("hello", 10); got != "hello" {
		t.Errorf("no-trunc got %q", got)
	}
	if got := truncateStr("hello world", 5); got != "hello" {
		t.Errorf("trunc got %q", got)
	}
}
