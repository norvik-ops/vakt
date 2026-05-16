// Package ai provides an OpenAI-compatible LLM client for compliance reports and advice.
//
// Recommended models for CPU-only servers (no GPU required):
//
//	llama3.2:3b   — best quality/speed on CPU (~2 GB RAM)
//	phi3.5:mini   — fast, good reasoning (~2 GB RAM)
//	qwen2.5:3b    — strong multilingual, good for German (~2 GB RAM)
//
// Example docker-compose.yml addition:
//
//	ollama:
//	  image: ollama/ollama
//	  volumes: [ollama:/root/.ollama]
//	  environment:
//	    - OLLAMA_NUM_PARALLEL=1
//	# Pull model: docker exec ollama ollama pull llama3.2:3b
//
// Set env vars:
//
//	VAKT_AI_PROVIDER=openai
//	VAKT_AI_BASE_URL=http://ollama:11434/v1
//	VAKT_AI_API_KEY=  # leave empty for Ollama
//	VAKT_AI_MODEL=llama3.2:3b
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// AIClient speaks the OpenAI-compatible chat completions API.
// Works with: OpenAI, Mistral, Groq, Together, Ollama (/v1), LM Studio, vLLM, etc.
type AIClient struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

func NewAIClient(baseURL, apiKey, model string) *AIClient {
	return &AIClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model     string        `json:"model"`
	Messages  []chatMessage `json:"messages"`
	MaxTokens int           `json:"max_tokens,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// Generate sends a prompt and returns the response text.
func (c *AIClient) Generate(ctx context.Context, prompt string) (string, error) {
	return c.send(ctx, []chatMessage{{Role: "user", Content: prompt}}, 1500)
}

// GenerateWithSystem sends a system message plus a user prompt and returns the response text.
// Keeping max_tokens at 600 keeps responses compact and fast on CPU-only models.
func (c *AIClient) GenerateWithSystem(ctx context.Context, system, userPrompt string) (string, error) {
	msgs := []chatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: userPrompt},
	}
	return c.send(ctx, msgs, 600)
}

func (c *AIClient) send(ctx context.Context, messages []chatMessage, maxTokens int) (string, error) {
	body, _ := json.Marshal(chatRequest{
		Model:     c.model,
		Messages:  messages,
		MaxTokens: maxTokens,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ai request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ai provider returned %d", resp.StatusCode)
	}

	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}
	return result.Choices[0].Message.Content, nil
}

// IsAvailable checks connectivity to the provider's /v1/models endpoint.
func (c *AIClient) IsAvailable(ctx context.Context) bool {
	if c.baseURL == "" {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/models", nil)
	if err != nil {
		return false
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	check := &http.Client{Timeout: 5 * time.Second}
	resp, err := check.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
