// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package bcm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBCPTestIsStale(t *testing.T) {
	assert.True(t, BCPTestIsStale(""))
	assert.True(t, BCPTestIsStale("invalid-date"))
	assert.True(t, BCPTestIsStale("2020-01-01"))
	assert.False(t, BCPTestIsStale(time.Now().Format("2006-01-02")))
}
