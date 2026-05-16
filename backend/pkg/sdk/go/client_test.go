package shieldsdk_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	shieldsdk "github.com/sechealth-app/sechealth/pkg/sdk/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecretsClient_Get(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer sk_so_testtoken", r.Header.Get("Authorization"))
		assert.Equal(t, "/api/v1/secvault/projects/proj1/envs/env1/secrets/MY_KEY", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"id":    "secret-uuid",
			"key":   "MY_KEY",
			"value": "supersecret",
		})
	}))
	defer srv.Close()

	client := shieldsdk.New(srv.URL, "sk_so_testtoken")
	value, err := client.Secrets().Get(context.Background(), "proj1", "env1", "MY_KEY")
	require.NoError(t, err)
	assert.Equal(t, "supersecret", value)
}

func TestSecretsClient_List(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/secvault/projects/proj1/envs/env1/secrets", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]string{
			{"key": "KEY_A"},
			{"key": "KEY_B"},
		})
	}))
	defer srv.Close()

	client := shieldsdk.New(srv.URL, "tok")
	keys, err := client.Secrets().List(context.Background(), "proj1", "env1")
	require.NoError(t, err)
	assert.Equal(t, []string{"KEY_A", "KEY_B"}, keys)
}

func TestSecretsClient_Get_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "secret not found",
			"code":  "NOT_FOUND",
		})
	}))
	defer srv.Close()

	client := shieldsdk.New(srv.URL, "tok")
	_, err := client.Secrets().Get(context.Background(), "p", "e", "MISSING")
	require.Error(t, err)

	var apiErr *shieldsdk.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "NOT_FOUND", apiErr.Code)
	assert.Equal(t, 404, apiErr.StatusCode)
}

func TestAPIError_Error(t *testing.T) {
	err := &shieldsdk.APIError{Code: "NOT_FOUND", Message: "secret not found", StatusCode: 404}
	assert.Equal(t, "[404] NOT_FOUND: secret not found", err.Error())
}

func TestNew_TrailingSlash(t *testing.T) {
	// Ensure trailing slash in base URL doesn't double-slash paths.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.NotContains(t, r.URL.Path, "//")
		_ = json.NewEncoder(w).Encode(map[string]string{"key": "X", "value": "1"})
	}))
	defer srv.Close()

	client := shieldsdk.New(srv.URL+"/", "tok")
	_, err := client.Secrets().Get(context.Background(), "p", "e", "X")
	require.NoError(t, err)
}
