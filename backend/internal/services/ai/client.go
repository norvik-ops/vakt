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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/matharnica/vakt/internal/shared/httputil"
)

// aiAllowsPrivateTargets is why every client below passes `true`.
//
// The AI provider is an admin-configurable outbound host — the base URL can be
// overridden per organisation from the database (ai/routes.go: BaseURLOverride) —
// which is exactly the DNS-rebinding surface httputil.GuardedClient exists to
// close: it resolves the hostname and dials the resolved IP in one step, so the
// address that was checked is the address that gets connected to.
//
// But it must still ALLOW private targets, because the default AI provider is a
// local Ollama container at http://ollama:11434 — a private address by
// construction. Refusing private IPs here would not harden anything; it would
// switch the AI features off for every default installation. The same reasoning
// (and the same `true`) applies to the SIEM forwarder and the alerting service:
// in a self-hosted product, the target being inside the customer's own network is
// the normal case, not the attack. Each such dial is logged at WARN, so the
// exception stays auditable.
const aiAllowsPrivateTargets = true

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
		client:  httputil.GuardedClient(120*time.Second, aiAllowsPrivateTargets),
	}
}

// WithTimeout overrides the HTTP client timeout used for non-streaming AI calls.
// Call this after NewAIClient when VAKT_AI_REPORT_TIMEOUT is configured.
func (c *AIClient) WithTimeout(d time.Duration) *AIClient {
	c.client.Timeout = d
	return c
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
	Usage *chatUsage `json:"usage,omitempty"`
}

type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// ChatResult bundles the generated text with the provider's token usage.
type ChatResult struct {
	Text      string
	TokensIn  *int
	TokensOut *int
}

// Generate sends a prompt and returns the response text.
func (c *AIClient) Generate(ctx context.Context, prompt string) (string, error) {
	result, err := c.send(ctx, []chatMessage{{Role: "user", Content: prompt}}, 1500)
	return result.Text, err
}

// GenerateWithSystem sends a system message plus a user prompt and returns the response text.
// Keeping max_tokens at 600 keeps responses compact and fast on CPU-only models.
func (c *AIClient) GenerateWithSystem(ctx context.Context, system, userPrompt string) (string, error) {
	result, err := c.GenerateWithSystemFull(ctx, system, userPrompt)
	return result.Text, err
}

// GenerateWithSystemFull is like GenerateWithSystem but returns token counts from the provider.
func (c *AIClient) GenerateWithSystemFull(ctx context.Context, system, userPrompt string) (ChatResult, error) {
	msgs := []chatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: userPrompt},
	}
	return c.send(ctx, msgs, 600)
}

func (c *AIClient) send(ctx context.Context, messages []chatMessage, maxTokens int) (ChatResult, error) {
	body, _ := json.Marshal(chatRequest{
		Model:     c.model,
		Messages:  messages,
		MaxTokens: maxTokens,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return ChatResult{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return ChatResult{}, fmt.Errorf("ai request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ChatResult{}, fmt.Errorf("ai provider returned %d", resp.StatusCode)
	}

	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ChatResult{}, fmt.Errorf("decode response: %w", err)
	}
	if len(result.Choices) == 0 {
		return ChatResult{}, fmt.Errorf("no choices in response")
	}
	out := ChatResult{Text: result.Choices[0].Message.Content}
	if result.Usage != nil {
		in := result.Usage.PromptTokens
		comp := result.Usage.CompletionTokens
		out.TokensIn = &in
		out.TokensOut = &comp
	}
	return out, nil
}

// StreamChunk ist ein einzelnes Delta vom Streaming-Endpoint. Bei einer
// Cloud-OpenAI-konformen API enthaelt es entweder Content (das naechste
// Stueck Text) oder Done=true am Ende des Streams.
type StreamChunk struct {
	Content string
	Done    bool
}

// StreamGenerate sendet system + user-Prompt und streamt die Response als
// OpenAI-konforme `data: { ... }`-SSE-Frames zurueck. Liefert den Channel,
// auf dem die Chunks ankommen; der Channel wird geschlossen wenn der Stream
// endet oder ein Fehler auftritt.
//
// Verwendet wird das Standard-OpenAI-Streaming-Format ("stream": true im
// Request, "delta.content" im jedem SSE-Frame, "[DONE]" als End-Marker).
// Ollama und LM Studio implementieren das gleiche Format ueber /v1/.
//
// Sprint 15 / S15-5.
func (c *AIClient) StreamGenerate(ctx context.Context, system, userPrompt string, maxTokens int) (<-chan StreamChunk, error) {
	msgs := buildMessages(system, userPrompt)
	reqBody := map[string]any{
		"model":      c.model,
		"messages":   msgs,
		"stream":     true,
		"max_tokens": maxTokens,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal stream request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build stream request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	// ponytail: 90 s ceiling prevents a hung AI provider from blocking a goroutine
	// forever. The caller's ctx may have no deadline (SSE handler). (PERF-M03)
	streamCtx, streamCancel := context.WithTimeout(ctx, 90*time.Second)
	defer streamCancel()
	req = req.WithContext(streamCtx)
	// Timeout 0: the stream must not be cut off mid-answer; streamCtx above
	// carries the 90 s ceiling.
	stream := httputil.GuardedClient(0, aiAllowsPrivateTargets)
	resp, err := stream.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stream request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("ai provider returned %d on stream", resp.StatusCode)
	}

	out := make(chan StreamChunk, 16)
	go runStreamReader(resp.Body, out)
	return out, nil
}

// runStreamReader parst den OpenAI-SSE-Stream Zeile-für-Zeile und emittiert
// StreamChunks. Schließt den Output-Channel + den Body, wenn der Stream endet
// (entweder via "[DONE]" oder via EOF). NICHT durch safego.Run gewrapped —
// der Aufrufer (Handler) kann die Cancellation via ctx kontrollieren.
func runStreamReader(body io.ReadCloser, out chan<- StreamChunk) {
	defer close(out)
	defer body.Close()
	scanner := bufio.NewScanner(body)
	// SSE-Frames können größer sein als der Default-Buffer (64 KB).
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			out <- StreamChunk{Done: true}
			return
		}
		var frame struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(payload), &frame); err != nil {
			continue
		}
		if len(frame.Choices) > 0 && frame.Choices[0].Delta.Content != "" {
			out <- StreamChunk{Content: frame.Choices[0].Delta.Content}
		}
	}
}

// IsAvailable checks connectivity to the provider's /v1/models endpoint.
func (c *AIClient) IsAvailable(ctx context.Context) bool {
	if c.baseURL == "" {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/models", nil)
	if err != nil {
		return false
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	check := httputil.GuardedClient(5*time.Second, aiAllowsPrivateTargets)
	resp, err := check.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
