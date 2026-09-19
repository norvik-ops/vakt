// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package ai

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

// TestListOllamaModels_GuardedClientReachesLocalProvider is the R1-SA22-05
// regression companion to TestGuardedClientStillReachesALocalProvider (S129-1/D19).
//
// ListOllamaModels dials the org-overridable AI base URL — the same SSRF /
// DNS-rebinding surface as the three chat clients in client.go — but it was the
// fourth client in the package and kept a raw &http.Client (variant miss). It now
// uses httputil.GuardedClient(…, aiAllowsPrivateTargets), which resolves the name
// and dials the resolved IP in one step.
//
// What this test pins: allowPrivate=true, so a provider on loopback (the default
// local-Ollama deployment) is still reachable and its model list is parsed. If a
// future change flipped the guard to reject private targets, ListOllamaModels
// would silently return an empty list and this test would fail. It does NOT prove
// DNS-rebinding closure — that is not unit-testable without a rebinding resolver,
// same limitation the D19 test carries.
func TestListOllamaModels_GuardedClientReachesLocalProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"name":"qwen2.5:7b"},{"name":"llama3"}]}`))
	}))
	defer srv.Close()

	// httptest listens on 127.0.0.1 — loopback, the local-Ollama case in miniature.
	svc := NewService(nil, srv.URL, "", "qwen2.5:7b")
	h := NewHandler(svc)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vaktcomply/ai/models", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.ListOllamaModels(c); err != nil {
		t.Fatalf("ListOllamaModels returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, `"models":[]`) {
		t.Fatalf("guarded client could not reach the loopback provider — the default "+
			"local-Ollama deployment would show an empty model list; body=%s", body)
	}
	// Sanity: the parsed names made it through.
	for _, want := range []string{"qwen2.5:7b", "llama3"} {
		if !strings.Contains(body, want) {
			t.Errorf("expected model %q in response, got %s", want, body)
		}
	}
}
