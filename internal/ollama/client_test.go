package ollama

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

// newTestClient returns a Client pointed at the given test server.
func newTestClient(server *httptest.Server) *Client {
	return &Client{
		Host:    server.URL,
		Timeout: 5 * time.Second,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// captureStderr swaps stderrWriter for a buffer, calls f, then restores it.
// Returns whatever was written to stderr during f.
func captureStderr(f func()) string {
	buf := &bytes.Buffer{}
	orig := stderrWriter
	stderrWriter = buf
	defer func() { stderrWriter = orig }()
	f()
	return buf.String()
}

// ollamaHandler returns an http.HandlerFunc that writes the given body with
// the given status code.
func ollamaHandler(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write([]byte(body))
	}
}

// fixture reads a file from testdata/responses/.
func fixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "responses", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	return string(data)
}

// validEnvelope builds a complete Ollama response envelope around an inner JSON string.
func validEnvelope(innerJSON string) string {
	env := generateResponse{
		Model:    "qwen2.5-coder:7b",
		Response: innerJSON,
		Done:     true,
	}
	b, _ := json.Marshal(env)
	return string(b)
}

// -----------------------------------------------------------------------
// DefaultGenerateOptions
// -----------------------------------------------------------------------

func TestDefaultGenerateOptions(t *testing.T) {
	opts := DefaultGenerateOptions()
	if opts.Temperature != 0.0 {
		t.Errorf("Temperature: want 0.0, got %v", opts.Temperature)
	}
	if opts.MaxTokens != 512 {
		t.Errorf("MaxTokens: want 512, got %d", opts.MaxTokens)
	}
}

// -----------------------------------------------------------------------
// NewClient
// -----------------------------------------------------------------------

func TestNewClient(t *testing.T) {
	c := NewClient("http://localhost:11434", 30*time.Second)
	if c.Host != "http://localhost:11434" {
		t.Errorf("Host: got %q", c.Host)
	}
	if c.Timeout != 30*time.Second {
		t.Errorf("Timeout: got %v", c.Timeout)
	}
	if c.httpClient == nil {
		t.Error("httpClient must not be nil")
	}
}

// -----------------------------------------------------------------------
// Request shape — verify the JSON body sent to Ollama
// -----------------------------------------------------------------------

func TestEvaluate_RequestShape(t *testing.T) {
	var captured generateRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.WriteHeader(200)
		w.Write([]byte(validEnvelope(`{"approved":true,"errors":[],"warnings":[],"summary":"ok"}`)))
	}))
	defer server.Close()

	c := newTestClient(server)
	opts := GenerateOptions{Temperature: 0.0, MaxTokens: 512}
	c.Evaluate("qwen2.5-coder:7b", "system prompt", "diff content", opts)

	if captured.Model != "qwen2.5-coder:7b" {
		t.Errorf("model: got %q", captured.Model)
	}
	if captured.System != "system prompt" {
		t.Errorf("system: got %q", captured.System)
	}
	if !strings.Contains(captured.Prompt, "diff content") {
		t.Errorf("prompt missing diff content: %q", captured.Prompt)
	}
	if captured.Stream {
		t.Error("stream must be false")
	}
	if captured.Format != "json" {
		t.Errorf("format: want json, got %q", captured.Format)
	}
	if captured.Options.Temperature != 0.0 {
		t.Errorf("temperature: want 0.0, got %v", captured.Options.Temperature)
	}
	if captured.Options.NumPredict != 512 {
		t.Errorf("num_predict: want 512, got %d", captured.Options.NumPredict)
	}
}

func TestEvaluate_EndpointPath(t *testing.T) {
	var requestPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		w.Write([]byte(validEnvelope(`{"approved":true,"errors":[],"warnings":[],"summary":"ok"}`)))
	}))
	defer server.Close()

	c := newTestClient(server)
	c.Evaluate("model", "sys", "diff", DefaultGenerateOptions())

	if requestPath != "/api/generate" {
		t.Errorf("endpoint: want /api/generate, got %q", requestPath)
	}
}

func TestEvaluate_ContentTypeHeader(t *testing.T) {
	var contentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		w.Write([]byte(validEnvelope(`{"approved":true,"errors":[],"warnings":[],"summary":"ok"}`)))
	}))
	defer server.Close()

	c := newTestClient(server)
	c.Evaluate("model", "sys", "diff", DefaultGenerateOptions())

	if contentType != "application/json" {
		t.Errorf("Content-Type: want application/json, got %q", contentType)
	}
}

// -----------------------------------------------------------------------
// Happy path — approved fixture
// -----------------------------------------------------------------------

func TestEvaluate_ApprovedFixture(t *testing.T) {
	body := fixture(t, "approved.json")
	server := httptest.NewServer(ollamaHandler(200, body))
	defer server.Close()

	c := newTestClient(server)
	result, err := c.Evaluate("qwen2.5-coder:7b", "sys", "diff", DefaultGenerateOptions())
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}

	var parsed struct {
		Approved bool     `json:"approved"`
		Errors   []string `json:"errors"`
		Summary  string   `json:"summary"`
	}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("inner JSON: %v", err)
	}
	if !parsed.Approved {
		t.Error("approved: want true")
	}
	if len(parsed.Errors) != 0 {
		t.Errorf("errors: want [], got %v", parsed.Errors)
	}
}

// -----------------------------------------------------------------------
// Happy path — rejected_multi fixture
// -----------------------------------------------------------------------

func TestEvaluate_RejectedMultiFixture(t *testing.T) {
	body := fixture(t, "rejected_multi.json")
	server := httptest.NewServer(ollamaHandler(200, body))
	defer server.Close()

	c := newTestClient(server)
	result, err := c.Evaluate("qwen2.5-coder:7b", "sys", "diff", DefaultGenerateOptions())
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	var parsed struct {
		Approved bool     `json:"approved"`
		Errors   []string `json:"errors"`
	}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("inner JSON: %v", err)
	}
	if parsed.Approved {
		t.Error("approved: want false")
	}
	if len(parsed.Errors) != 2 {
		t.Errorf("errors: want 2, got %d", len(parsed.Errors))
	}
}

// -----------------------------------------------------------------------
// Soft failures — all must return ("", nil)
// -----------------------------------------------------------------------

func TestEvaluate_HTTP400_SoftFail(t *testing.T) {
	server := httptest.NewServer(ollamaHandler(400, `{"error":"bad request"}`))
	defer server.Close()

	c := newTestClient(server)
	var result string
	stderr := captureStderr(func() {
		var err error
		result, err = c.Evaluate("model", "sys", "diff", DefaultGenerateOptions())
		if err != nil {
			t.Errorf("HTTP 400: want nil error, got %v", err)
		}
	})
	if result != "" {
		t.Errorf("HTTP 400: want empty result, got %q", result)
	}
	if !strings.Contains(stderr, "400") {
		t.Errorf("HTTP 400: want '400' in stderr, got %q", stderr)
	}
}

func TestEvaluate_HTTP500_SoftFail(t *testing.T) {
	server := httptest.NewServer(ollamaHandler(500, `{"error":"internal"}`))
	defer server.Close()

	c := newTestClient(server)
	var result string
	stderr := captureStderr(func() {
		var err error
		result, err = c.Evaluate("model", "sys", "diff", DefaultGenerateOptions())
		if err != nil {
			t.Errorf("HTTP 500: want nil error, got %v", err)
		}
	})
	if result != "" {
		t.Errorf("HTTP 500: want empty result, got %q", result)
	}
	if !strings.Contains(stderr, "500") {
		t.Errorf("HTTP 500: want '500' in stderr, got %q", stderr)
	}
}

func TestEvaluate_DoneFalse_SoftFail(t *testing.T) {
	envelope := `{"model":"m","response":"{}","done":false}`
	server := httptest.NewServer(ollamaHandler(200, envelope))
	defer server.Close()

	c := newTestClient(server)
	var result string
	stderr := captureStderr(func() {
		var err error
		result, err = c.Evaluate("model", "sys", "diff", DefaultGenerateOptions())
		if err != nil {
			t.Errorf("done=false: want nil error, got %v", err)
		}
	})
	if result != "" {
		t.Errorf("done=false: want empty result, got %q", result)
	}
	if !strings.Contains(stderr, "done=false") {
		t.Errorf("done=false: want 'done=false' in stderr, got %q", stderr)
	}
}

func TestEvaluate_MalformedEnvelope_SoftFail(t *testing.T) {
	server := httptest.NewServer(ollamaHandler(200, `not json at all`))
	defer server.Close()

	c := newTestClient(server)
	var result string
	stderr := captureStderr(func() {
		var err error
		result, err = c.Evaluate("model", "sys", "diff", DefaultGenerateOptions())
		if err != nil {
			t.Errorf("malformed envelope: want nil error, got %v", err)
		}
	})
	if result != "" {
		t.Errorf("malformed envelope: want empty result, got %q", result)
	}
	_ = stderr // stderr will contain parse error
}

func TestEvaluate_InvalidInnerJSON_SoftFail(t *testing.T) {
	// Envelope is valid, but response field contains invalid JSON.
	envelope := `{"model":"m","response":"NOT JSON","done":true}`
	server := httptest.NewServer(ollamaHandler(200, envelope))
	defer server.Close()

	c := newTestClient(server)
	var result string
	stderr := captureStderr(func() {
		var err error
		result, err = c.Evaluate("model", "sys", "diff", DefaultGenerateOptions())
		if err != nil {
			t.Errorf("invalid inner JSON: want nil error, got %v", err)
		}
	})
	if result != "" {
		t.Errorf("invalid inner JSON: want empty result, got %q", result)
	}
	if !strings.Contains(stderr, "not valid JSON") {
		t.Errorf("invalid inner JSON: expected stderr mention, got %q", stderr)
	}
}

func TestEvaluate_ConnectionRefused_SoftFail(t *testing.T) {
	// Point at a port that is definitely not listening.
	c := NewClient("http://127.0.0.1:19999", 2*time.Second)

	var result string
	stderr := captureStderr(func() {
		var err error
		result, err = c.Evaluate("model", "sys", "diff", DefaultGenerateOptions())
		if err != nil {
			t.Errorf("connection refused: want nil error, got %v", err)
		}
	})
	if result != "" {
		t.Errorf("connection refused: want empty result, got %q", result)
	}
	_ = stderr
}

func TestEvaluate_Timeout_SoftFail(t *testing.T) {
	// Server that delays longer than the client's timeout.
	// We use a short time.Sleep instead of blocking on r.Context().Done()
	// so that httptest.Server.Close() is not blocked waiting for the goroutine.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
	}))
	defer server.Close()

	c := &Client{
		Host:    server.URL,
		Timeout: 30 * time.Millisecond, // client times out well before handler finishes
		httpClient: &http.Client{
			Timeout: 30 * time.Millisecond,
		},
	}

	var result string
	stderr := captureStderr(func() {
		var err error
		result, err = c.Evaluate("model", "sys", "diff", DefaultGenerateOptions())
		if err != nil {
			t.Errorf("timeout: want nil error, got %v", err)
		}
	})
	if result != "" {
		t.Errorf("timeout: want empty result, got %q", result)
	}
	_ = stderr
}

// -----------------------------------------------------------------------
// Inner JSON validation
// -----------------------------------------------------------------------

func TestEvaluate_InnerJSONReturned_Verbatim(t *testing.T) {
	inner := `{"approved":true,"errors":[],"warnings":["minor style"],"summary":"test"}`
	server := httptest.NewServer(ollamaHandler(200, validEnvelope(inner)))
	defer server.Close()

	c := newTestClient(server)
	result, err := c.Evaluate("m", "s", "d", DefaultGenerateOptions())
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if result != inner {
		t.Errorf("inner JSON mismatch:\ngot  %q\nwant %q", result, inner)
	}
}

// -----------------------------------------------------------------------
// Request prompt format
// -----------------------------------------------------------------------

func TestEvaluate_PromptContainsDiff(t *testing.T) {
	var captured generateRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&captured)
		w.Write([]byte(validEnvelope(`{"approved":true,"errors":[],"warnings":[],"summary":"x"}`)))
	}))
	defer server.Close()

	c := newTestClient(server)
	c.Evaluate("m", "s", "+func hello() {}", DefaultGenerateOptions())

	if !strings.Contains(captured.Prompt, "+func hello() {}") {
		t.Errorf("prompt: diff content not found in %q", captured.Prompt)
	}
	if !strings.Contains(captured.Prompt, "git diff") {
		t.Errorf("prompt: expected 'git diff' preamble in %q", captured.Prompt)
	}
}

// -----------------------------------------------------------------------
// Wire type JSON tags
// -----------------------------------------------------------------------

func TestGenerateRequest_JSONTags(t *testing.T) {
	req := generateRequest{
		Model:   "m",
		System:  "s",
		Prompt:  "p",
		Stream:  false,
		Format:  "json",
		Options: generateOptions{Temperature: 0.5, NumPredict: 256},
	}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	s := string(b)
	for _, field := range []string{`"model"`, `"system"`, `"prompt"`, `"stream"`, `"format"`, `"options"`, `"temperature"`, `"num_predict"`} {
		if !strings.Contains(s, field) {
			t.Errorf("JSON missing field %q in %s", field, s)
		}
	}
}
