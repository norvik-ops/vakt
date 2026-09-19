// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"errors"
	"testing"
)

// R1-W8C-N1(c): a genuine load error (config row present but unreadable) must
// NOT silently fall through to the Casdoor proxy — that would downgrade a
// customer who configured direct SAML. "Not configured" (nil cfg, nil err) must
// still fall through.
func TestClassifySAMLDirect(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *OrgSAMLConfig
		loadErr error
		want    samlDirectMode
	}{
		{
			name:    "load error must fail closed, not fall through",
			cfg:     nil,
			loadErr: errors.New("decrypt private key: cipher: message authentication failed"),
			want:    samlDirectLoadError,
		},
		{
			name:    "not configured (nil cfg, nil err) falls through to Casdoor",
			cfg:     nil,
			loadErr: nil,
			want:    samlDirectFallthrough,
		},
		{
			name:    "config present but disabled falls through to Casdoor",
			cfg:     &OrgSAMLConfig{Enabled: false},
			loadErr: nil,
			want:    samlDirectFallthrough,
		},
		{
			name:    "enabled config is used locally",
			cfg:     &OrgSAMLConfig{Enabled: true},
			loadErr: nil,
			want:    samlDirectUse,
		},
		{
			name:    "load error wins even if a stale cfg is also present",
			cfg:     &OrgSAMLConfig{Enabled: true},
			loadErr: errors.New("boom"),
			want:    samlDirectLoadError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifySAMLDirect(tt.cfg, tt.loadErr); got != tt.want {
				t.Fatalf("classifySAMLDirect() = %v, want %v", got, tt.want)
			}
		})
	}
}
