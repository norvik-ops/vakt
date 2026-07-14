// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGuardedClientStillReachesALocalProvider (S129-1 / D19) is the test that had
// to exist before the guard could go in.
//
// The AI base URL is admin-configurable — an organisation can override it from the
// database — which makes it an outbound host an attacker could point at an internal
// address, and re-point after the check (DNS rebinding). httputil.GuardedClient
// closes that: it resolves the name and dials the resolved IP in one step, so the
// address that was validated is the address that gets connected to.
//
// The trap is the obvious fix. GuardedClient(timeout, false) refuses private and
// loopback targets — and the DEFAULT AI provider is a local Ollama container at
// http://ollama:11434, a private address by construction. Rejecting private targets
// would not have hardened anything; it would have switched the AI features off for
// every default installation, silently, and the failure would look like "the model
// is down" rather than "we broke it".
//
// So the client passes allowPrivate=true (as the SIEM forwarder and the alerting
// service already do — in a self-hosted product the target sitting inside the
// customer's own network is the normal case, not the attack), and this test pins
// that: a provider on loopback must still be reachable.
func TestGuardedClientStillReachesALocalProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"qwen2.5:7b"}]}`))
	}))
	defer srv.Close()

	// httptest listens on 127.0.0.1 — loopback, i.e. exactly what the guard blocks
	// when allowPrivate is false. This is the local-Ollama case in miniature.
	c := NewAIClient(srv.URL, "", "qwen2.5:7b")

	if !c.IsAvailable(context.Background()) {
		t.Fatal("the guarded client cannot reach a provider on loopback — " +
			"this is the default deployment (local Ollama), and it must keep working")
	}
}
