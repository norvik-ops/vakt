// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Wer und wann eine Richtlinie freigegeben hat, entscheidet allein die
// Service-Schicht aus dem Statusübergang und der authentifizierten Identität.
// Bis Migration 272 war das einzige „Wer" der Freitext last_updated_by, den der
// Client mitschickt — fälschbar (Sicherheitsreview 2026-11-18).

const (
	approvalTestActor = "11111111-1111-1111-1111-111111111111"
)

var approvalTestNow = time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)

func TestApprovalTransition_FreigabeSetztAuthentifizierteIdentitaet(t *testing.T) {
	for _, prev := range []string{"draft", "archived"} {
		t.Run(prev+"_nach_active", func(t *testing.T) {
			c, err := approvalTransition(prev, "active", approvalTestActor, approvalTestNow)
			require.NoError(t, err)
			assert.True(t, c.Touch)
			assert.Equal(t, approvalTestActor, c.By, "der Freigeber ist die angemeldete Identität")
			require.NotNil(t, c.At)
			assert.True(t, c.At.Equal(approvalTestNow))
			assert.Equal(t, ApprovalGranted, c.Event)
		})
	}
}

func TestApprovalTransition_VerlassenVonActiveLeert(t *testing.T) {
	for _, next := range []string{"draft", "archived"} {
		t.Run("active_nach_"+next, func(t *testing.T) {
			c, err := approvalTransition("active", next, approvalTestActor, approvalTestNow)
			require.NoError(t, err)
			assert.True(t, c.Touch, "die Spalten müssen geschrieben werden")
			assert.Empty(t, c.By, "approved_by wird geleert")
			assert.Nil(t, c.At, "approved_at wird geleert")
			assert.Equal(t, ApprovalRevoked, c.Event)
		})
	}
}

func TestApprovalTransition_SonstUnveraendert(t *testing.T) {
	cases := [][2]string{
		{"active", "active"}, // Text einer aktiven Richtlinie ändern ist keine neue Freigabe
		{"draft", "draft"},
		{"draft", "archived"},
		{"archived", "draft"},
	}
	for _, tc := range cases {
		t.Run(tc[0]+"_nach_"+tc[1], func(t *testing.T) {
			c, err := approvalTransition(tc[0], tc[1], "22222222-2222-2222-2222-222222222222", approvalTestNow)
			require.NoError(t, err)
			assert.False(t, c.Touch, "approved_by/approved_at bleiben wie sie sind")
			assert.Equal(t, ApprovalUnchanged, c.Event)
		})
	}
}

func TestApprovalTransition_OhneIdentitaetKeineFreigabe(t *testing.T) {
	_, err := approvalTransition("draft", "active", "", approvalTestNow)
	assert.ErrorIs(t, err, ErrApproverUnknown)
}

// Ein Client, der approved_by/approved_at mitschickt, kann sie nicht setzen:
// das Request-DTO hat diese Felder nicht, der JSON-Decoder verwirft sie.
func TestUpdatePolicyInput_IgnoriertClientFreigabeFelder(t *testing.T) {
	typ := reflect.TypeOf(UpdatePolicyInput{})
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("json")
		assert.False(t, strings.HasPrefix(tag, "approved"),
			"UpdatePolicyInput.%s (json %q) würde eine Freigabe aus dem Client übernehmen", typ.Field(i).Name, tag)
	}

	body := `{"title":"T","status":"active","updated_by":"Fremder",
	          "approved_by":"33333333-3333-3333-3333-333333333333",
	          "approved_at":"2020-01-01T00:00:00Z","approved_by_name":"Fremder"}`
	var in UpdatePolicyInput
	require.NoError(t, json.Unmarshal([]byte(body), &in))

	c, err := approvalTransition("draft", in.Status, approvalTestActor, approvalTestNow)
	require.NoError(t, err)
	assert.Equal(t, approvalTestActor, c.By, "nicht der Wert aus dem Body, nicht der Freitext updated_by")
	assert.True(t, c.At.Equal(approvalTestNow), "nicht der Zeitpunkt aus dem Body")
}
