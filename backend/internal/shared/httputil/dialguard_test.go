// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package httputil

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGuardedClient_BlocksLoopback proves the dial-time guard actually refuses a
// connection to a private/loopback target — the S121-F4 (F1-Inj) property. An
// httptest server binds to 127.0.0.1, so a guarded client with allowPrivate=false
// must fail to reach it, and the same client with allowPrivate=true must succeed.
// This is a behavioural test: it dials, it doesn't just inspect a struct.
func TestGuardedClient_BlocksLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// allowPrivate=false → the loopback target is refused at dial time.
	blocked := GuardedClient(5*time.Second, false)
	_, err := blocked.Get(srv.URL)
	require.Error(t, err, "guard must refuse a loopback target when allowPrivate=false")
	assert.Contains(t, strings.ToLower(err.Error()), "private",
		"the error must explain it was a private/link-local refusal")

	// allowPrivate=true → the same target is reachable (on-prem opt-in).
	allowed := GuardedClient(5*time.Second, true)
	resp, err := allowed.Get(srv.URL)
	require.NoError(t, err, "allowPrivate=true must permit an intentional internal target")
	require.NotNil(t, resp)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestGuardedDialContext_BlocksMetadataIP is the concrete SSRF case: the cloud
// metadata endpoint 169.254.169.254 is link-local and must be refused regardless
// of DNS. We dial it directly (no DNS needed) so the test is hermetic.
func TestGuardedDialContext_BlocksMetadataIP(t *testing.T) {
	dial := GuardedDialContext(false)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := dial(ctx, "tcp", "169.254.169.254:80")
	require.Error(t, err, "the cloud metadata IP must never be dialled when allowPrivate=false")
	assert.Contains(t, strings.ToLower(err.Error()), "private")
}
