// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"testing"

	"github.com/crewjam/saml"
)

// GO-2026-6909 (Teil 2): crewjam/saml prüft den Namespace der NameID nicht.
// Eine NameID kann wie die E-Mail eines fremden Kontos aussehen. Deshalb zählt
// für den Login nur das E-Mail-Attribut, nie die NameID.
func TestSAMLLoginEmail(t *testing.T) {
	withNameID := func(attrs ...saml.Attribute) *saml.Assertion {
		a := &saml.Assertion{
			Subject: &saml.Subject{NameID: &saml.NameID{
				Format: "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent",
				Value:  "admin@victim.example",
			}},
		}
		if len(attrs) > 0 {
			a.AttributeStatements = []saml.AttributeStatement{{Attributes: attrs}}
		}
		return a
	}
	attr := func(name, value string) saml.Attribute {
		return saml.Attribute{Name: name, Values: []saml.AttributeValue{{Value: value}}}
	}

	tests := []struct {
		name string
		a    *saml.Assertion
		want string
	}{
		{"kein E-Mail-Attribut: NameID wird nicht als E-Mail genommen", withNameID(), ""},
		{"Attribut email gewinnt", withNameID(attr("email", "alice@corp.example")), "alice@corp.example"},
		{"Azure-Claim-URI", withNameID(attr("http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress", "bob@corp.example")), "bob@corp.example"},
		{"leerer Attributwert zählt als fehlend", withNameID(saml.Attribute{Name: "email"}), ""},
		{"ohne Subject kein Panic", &saml.Assertion{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := samlLoginEmail(tt.a); got != tt.want {
				t.Fatalf("samlLoginEmail() = %q, want %q", got, tt.want)
			}
		})
	}
}
