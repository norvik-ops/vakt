// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktprivacy

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// S9/CZ-2: channel, reference_id and assigned_to were declared on the DSR model
// and written by CreateDSR/AssignDSR, but no row mapper ever scanned them back —
// every DSR response silently showed an empty channel/reference and no assignee.
func TestDsrFromFields_MapsChannelReferenceAssignee(t *testing.T) {
	f := dsrFields{
		ID: "dsr-1", OrgID: "org-1",
		RequesterName: "Jane Doe", RequesterEmail: "jane@example.com",
		Type: "access", Status: "open",
		Channel:     pgtype.Text{String: "email", Valid: true},
		ReferenceID: pgtype.Text{String: "TICKET-42", Valid: true},
		AssignedTo:  pgtype.Text{String: "user-123", Valid: true},
	}

	d := dsrFromFields(f)

	assert.Equal(t, "email", d.Channel)
	assert.Equal(t, "TICKET-42", d.ReferenceID)
	require.NotNil(t, d.AssignedTo)
	assert.Equal(t, "user-123", *d.AssignedTo)
}

// An unassigned DSR (assigned_to IS NULL) must round-trip as a nil pointer,
// not an empty string — callers distinguish "unassigned" from "assigned to ”".
func TestDsrFromFields_UnsetAssigneeIsNil(t *testing.T) {
	f := dsrFields{ID: "dsr-2", OrgID: "org-1", Type: "access", Status: "open"}

	d := dsrFromFields(f)

	assert.Empty(t, d.Channel)
	assert.Empty(t, d.ReferenceID)
	assert.Nil(t, d.AssignedTo)
}
