// Package shieldsdk is the official Go client for the Vakt API.
// It provides type-safe access to secrets managed by a Vakt instance.
package shieldsdk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is the Vakt API client.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// New creates a new Client.
// baseURL must be the root URL of your Vakt instance (e.g. "https://secrets.example.com").
// token must be a valid Vakt Vault API token (sk_so_...).
func New(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Secrets returns the secrets accessor.
func (c *Client) Secrets() *SecretsClient {
	return &SecretsClient{c: c}
}

// SecretsClient provides secrets access methods.
type SecretsClient struct{ c *Client }

// secretResponse is the JSON shape returned by GET /secrets/:key.
type secretResponse struct {
	ID    string `json:"id"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

// secretKeyResponse is the JSON shape returned by GET /secrets (list).
type secretKeyResponse struct {
	Key string `json:"key"`
}

// Get retrieves a secret value by key.
// projectID and envID are the UUIDs of the project and environment respectively.
func (sc *SecretsClient) Get(ctx context.Context, projectID, envID, key string) (string, error) {
	path := fmt.Sprintf("/api/v1/secvault/projects/%s/envs/%s/secrets/%s", projectID, envID, key)

	var resp secretResponse
	if err := sc.c.doGet(ctx, path, &resp); err != nil {
		return "", err
	}
	return resp.Value, nil
}

// List returns all secret keys for a project+environment (values are not included).
func (sc *SecretsClient) List(ctx context.Context, projectID, envID string) ([]string, error) {
	path := fmt.Sprintf("/api/v1/secvault/projects/%s/envs/%s/secrets", projectID, envID)

	var resp []secretKeyResponse
	if err := sc.c.doGet(ctx, path, &resp); err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(resp))
	for _, r := range resp {
		keys = append(keys, r.Key)
	}
	return keys, nil
}

// doGet executes an authenticated GET request and JSON-decodes the response body into dst.
func (c *Client) doGet(ctx context.Context, path string, dst interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr apiErrorBody
		if jsonErr := json.Unmarshal(body, &apiErr); jsonErr == nil && apiErr.Code != "" {
			return &APIError{
				Code:       apiErr.Code,
				Message:    apiErr.Error,
				StatusCode: resp.StatusCode,
			}
		}
		return &APIError{
			Code:       "UNKNOWN",
			Message:    string(body),
			StatusCode: resp.StatusCode,
		}
	}

	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// apiErrorBody is the JSON error shape returned by Vakt.
type apiErrorBody struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// APIError is a typed error returned from the Vakt API.
type APIError struct {
	Code       string
	Message    string
	StatusCode int
}

func (e *APIError) Error() string {
	return fmt.Sprintf("[%d] %s: %s", e.StatusCode, e.Code, e.Message)
}
