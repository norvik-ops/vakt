// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktaware

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
)

func TestAssignmentStatus_NoCompletionRow(t *testing.T) {
	assert.Equal(t, "assigned", assignmentStatus(pgtype.Bool{Valid: false}))
}

func TestAssignmentStatus_Passed(t *testing.T) {
	assert.Equal(t, "completed", assignmentStatus(pgtype.Bool{Valid: true, Bool: true}))
}

func TestAssignmentStatus_Failed(t *testing.T) {
	assert.Equal(t, "failed", assignmentStatus(pgtype.Bool{Valid: true, Bool: false}))
}
