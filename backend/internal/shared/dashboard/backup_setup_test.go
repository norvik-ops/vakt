// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package dashboard

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// UX-10: a fresh instance gets "set up a backup", not "no backup in the last 7 days".
func TestBackupSetupPending(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	str := func(s string) *string { return &s }
	last := now.Add(-10 * day)

	cases := []struct {
		name     string
		last     *time.Time
		created  time.Time
		destType *string
		want     bool
	}{
		{"frische Instanz ohne Ziel", nil, now.Add(-1 * time.Hour), nil, true},
		{"Ziel-Typ none zaehlt als nicht eingerichtet", nil, now.Add(-2 * day), str("none"), true},
		{"leerer Ziel-Typ zaehlt als nicht eingerichtet", nil, now.Add(-2 * day), str(""), true},
		{"Instanz aelter als 7 Tage: Warnung", nil, now.Add(-8 * day), nil, false},
		{"genau 7 Tage: Warnung", nil, now.Add(-7 * day), nil, false},
		{"Ziel eingerichtet: Warnung", nil, now.Add(-1 * day), str("s3"), false},
		{"es gab schon ein Backup: kein Einrichtungshinweis", &last, now.Add(-1 * day), nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, backupSetupPending(tc.last, tc.created, tc.destType, now))
		})
	}
}
