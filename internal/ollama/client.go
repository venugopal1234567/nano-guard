// Package ollama implements the HTTP client for the local Ollama inference
// server used by Nano-Guard.
//
// Spec reference: specs/06-ollama-api.md
//
// All network/parse failures are "soft" — the caller receives ("", nil)
// and should exit(0) (fail-open). Hard errors (e.g. request construction)
// are returned so the caller can log them to stderr before exiting 0.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// -----------------------------------------------------------------------
// Public types
// -----------------------------------------------------------------------

// GenerateOptions controls LLM generation behaviour.
type GenerateOptions struct {
	// Temperature controls randomness. 0.0 = fully deterministic.
	Temperature float64
	// MaxTokens caps the number of tokens the model will generate.
	MaxTokens int
}

// DefaultGenerateOptions returns the spec-recommended options for code review.
func DefaultGenerateOptions() GenerateOptions {
	return GenerateOptions{
		Temperature: 0.0,
		MaxTokens:   512,
	}
}

// Client is a thin wrapper around the Ollama /api/generate endpoint.
type Client struct {
	// Host is the base URL of the Ollama server, e.g. "http://localhost:11434".
	Host string
	// Timeout is the maximum time to wait for a response before giving up.
	Timeout time.Duration
	// httpClient is the underlying transport; injectable for tests.
	httpClient *http.Client
}

// NewClient constructs a Client with the given host and timeout.
func NewClient(host string, timeout time.Duration) *Client {
	return &Client{
		Host:    host,
		Timeout: timeout,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// -----------------------------------------------------------------------
// Wire types (private)
// -----------------------------------------------------------------------

// generateRequest is the exact JSON body sent to POST /api/generate.
type generateRequest struct {
	Model   string          `json:"model"`
	System  string          `json:"system"`
	Prompt  string          `json:"prompt"`
	Stream  bool            `json:"stream"`
	Format  string          `json:"format"`
	Options generateOptions `json:"options"`
}

// generateOptions maps to the nested "options" field in the Ollama request.
type generateOptions struct {
	Temperature float64 `json:"temperature"`
	NumPredict  int     `json:"num_predict"`
}

// generateResponse is the JSON body returned by Ollama.
// The LLM output lives inside Response as a JSON string (double-encoded).
type generateResponse struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Response  string `json:"response"` // inner JSON string — must be decoded again
	Done      bool   `json:"done"`
}

// -----------------------------------------------------------------------
// Evaluate
// -----------------------------------------------------------------------

// Evaluate sends the diff and system prompt to the Ollama /api/generate
// endpoint and returns the raw JSON string from the "response" field.
//
// Soft failure modes (return "", nil):
//   - Network / connection refused
//   - HTTP status >= 400
//   - Response done=false
//   - Response field is not valid JSON
//   - Timeout exceeded
//
// The caller should treat ("", nil) as a fail-open signal and exit(0).
// A non-nil error indicates an unrecoverable programming error (e.g. JSON
// marshalling of the request failed).
func (c *Client) Evaluate(model, systemPrompt, diff string, opts GenerateOptions) (string, error) {
	reqBody := generateRequest{
		Model:  model,
		System: systemPrompt,
		Prompt: fmt.Sprintf("Analyze this git diff:\n\n%s", diff),
		Stream: false,
		Format: "json",
		Options: generateOptions{
			Temperature: opts.Temperature,
			NumPredict:  opts.MaxTokens,
		},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		// This should never happen with a well-typed struct.
		return "", fmt.Errorf("ollama: marshal request: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	url := c.Host + "/api/generate"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("ollama: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		// Network error / timeout / connection refused → soft fail.
		fmt.Fprintf(c.stderr(), "nano-guard: ollama request failed: %v\n", err)
		return "", nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		fmt.Fprintf(c.stderr(), "nano-guard: ollama HTTP %d\n", resp.StatusCode)
		return "", nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(c.stderr(), "nano-guard: read ollama response: %v\n", err)
		return "", nil
	}

	var ollamaResp generateResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		fmt.Fprintf(c.stderr(), "nano-guard: parse ollama envelope: %v\n", err)
		return "", nil
	}

	if !ollamaResp.Done {
		fmt.Fprintf(c.stderr(), "nano-guard: ollama response incomplete (done=false)\n")
		return "", nil
	}

	// Validate that the inner response is itself valid JSON.
	if !json.Valid([]byte(ollamaResp.Response)) {
		fmt.Fprintf(c.stderr(), "nano-guard: ollama inner response is not valid JSON: %q\n", ollamaResp.Response)
		return "", nil
	}

	return ollamaResp.Response, nil
}

// stderr returns the writer used for diagnostic messages. In production this
// is os.Stderr; tests can override c.httpClient to intercept HTTP calls and
// inspect stderr separately.
func (c *Client) stderr() io.Writer {
	// Keeping a simple reference here avoids importing "os" everywhere and
	// makes the dependency graph clear. If we ever need to inject stderr in
	// tests we can add a field; for now os.Stderr is fine.
	return stderrWriter
}
