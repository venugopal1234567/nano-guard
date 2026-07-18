# Nano-Guard: Unit Test Code

This document contains the complete, runnable source code for all unit tests across every Go module and the TypeScript init CLI. These files map 1:1 to the source files defined in [01-project-structure.md](./01-project-structure.md).

Run all Go unit tests (no Ollama needed):
```bash
go test ./internal/... -v -race -cover
```

Run TypeScript init tests:
```bash
cd init && npm test
```

---

## 1. `internal/hook/hook_test.go`

```go
package hook_test

import (
	"strings"
	"testing"

	"github.com/your-org/nano-guard/internal/hook"
)

// --- Fixtures ---

const validWritePayload = `{
	"hook_event_name": "PostToolUse",
	"session_id": "test-001",
	"tool_name": "write_to_file",
	"tool_input": {
		"TargetFile": "/tmp/project/src/server.go",
		"CodeContent": "package main\nfunc main() {}"
	},
	"tool_response": { "success": true },
	"cwd": "/tmp/project"
}`

const validEditPayload = `{
	"hook_event_name": "PostToolUse",
	"session_id": "test-002",
	"tool_name": "replace_file_content",
	"tool_input": {
		"TargetFile": "/tmp/project/src/handler.go"
	},
	"tool_response": {},
	"cwd": "/tmp/project"
}`

const readToolPayload = `{
	"hook_event_name": "PostToolUse",
	"tool_name": "Read",
	"tool_input": { "AbsolutePath": "/tmp/project/src/server.go" },
	"cwd": "/tmp/project"
}`

const absolutePathPayload = `{
	"hook_event_name": "PostToolUse",
	"tool_name": "Edit",
	"tool_input": { "AbsolutePath": "/tmp/project/src/util.go" },
	"cwd": "/tmp/project"
}`

const genericPathPayload = `{
	"hook_event_name": "PostToolUse",
	"tool_name": "Write",
	"tool_input": { "path": "/tmp/project/src/misc.go" },
	"cwd": "/tmp/project"
}`

const noFilePathPayload = `{
	"hook_event_name": "PostToolUse",
	"tool_name": "write_to_file",
	"tool_input": {},
	"cwd": "/tmp/project"
}`

const missingCwdPayload = `{
	"hook_event_name": "PostToolUse",
	"tool_name": "write_to_file",
	"tool_input": { "TargetFile": "/tmp/project/src/main.go" }
}`

// --- ParseStdin Tests ---

func TestParseValidWritePayload(t *testing.T) {
	input, err := hook.ParseStdin(strings.NewReader(validWritePayload))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if input.ToolName != "write_to_file" {
		t.Errorf("expected tool_name 'write_to_file', got '%s'", input.ToolName)
	}
	if input.Cwd != "/tmp/project" {
		t.Errorf("expected cwd '/tmp/project', got '%s'", input.Cwd)
	}
}

func TestParseValidEditPayload(t *testing.T) {
	input, err := hook.ParseStdin(strings.NewReader(validEditPayload))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if input.ToolName != "replace_file_content" {
		t.Errorf("expected tool_name 'replace_file_content', got '%s'", input.ToolName)
	}
}

func TestParseEmptyStdin(t *testing.T) {
	_, err := hook.ParseStdin(strings.NewReader(""))
	if err == nil {
		t.Fatal("expected error for empty stdin, got nil")
	}
	if err != hook.ErrEmptyInput {
		t.Errorf("expected ErrEmptyInput, got: %v", err)
	}
}

func TestParseMalformedJSON(t *testing.T) {
	_, err := hook.ParseStdin(strings.NewReader("{not valid json"))
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
	if err != hook.ErrInvalidJSON {
		t.Errorf("expected ErrInvalidJSON, got: %v", err)
	}
}

// --- ExtractTargetFile Tests ---

func TestExtractTargetFile_TargetFileKey(t *testing.T) {
	input, _ := hook.ParseStdin(strings.NewReader(validWritePayload))
	path := hook.ExtractTargetFile(input)
	if path != "/tmp/project/src/server.go" {
		t.Errorf("expected '/tmp/project/src/server.go', got '%s'", path)
	}
}

func TestExtractTargetFile_AbsolutePathKey(t *testing.T) {
	input, _ := hook.ParseStdin(strings.NewReader(absolutePathPayload))
	path := hook.ExtractTargetFile(input)
	if path != "/tmp/project/src/util.go" {
		t.Errorf("expected '/tmp/project/src/util.go', got '%s'", path)
	}
}

func TestExtractTargetFile_GenericPathKey(t *testing.T) {
	input, _ := hook.ParseStdin(strings.NewReader(genericPathPayload))
	path := hook.ExtractTargetFile(input)
	if path != "/tmp/project/src/misc.go" {
		t.Errorf("expected '/tmp/project/src/misc.go', got '%s'", path)
	}
}

func TestExtractTargetFile_NoKey(t *testing.T) {
	input, _ := hook.ParseStdin(strings.NewReader(noFilePathPayload))
	path := hook.ExtractTargetFile(input)
	if path != "" {
		t.Errorf("expected empty string, got '%s'", path)
	}
}

// --- IsWriteTool Tests ---

func TestIsWriteTool_WriteToFile(t *testing.T) {
	if !hook.IsWriteTool("write_to_file") {
		t.Error("expected write_to_file to be a write tool")
	}
}

func TestIsWriteTool_ReplaceFileContent(t *testing.T) {
	if !hook.IsWriteTool("replace_file_content") {
		t.Error("expected replace_file_content to be a write tool")
	}
}

func TestIsWriteTool_MultiReplace(t *testing.T) {
	if !hook.IsWriteTool("multi_replace_file_content") {
		t.Error("expected multi_replace_file_content to be a write tool")
	}
}

func TestIsWriteTool_Edit(t *testing.T) {
	if !hook.IsWriteTool("Edit") {
		t.Error("expected Edit to be a write tool")
	}
}

func TestIsWriteTool_Write(t *testing.T) {
	if !hook.IsWriteTool("Write") {
		t.Error("expected Write to be a write tool")
	}
}

func TestIsWriteTool_Read(t *testing.T) {
	if hook.IsWriteTool("Read") {
		t.Error("expected Read NOT to be a write tool")
	}
}

func TestIsWriteTool_Bash(t *testing.T) {
	if hook.IsWriteTool("Bash") {
		t.Error("expected Bash NOT to be a write tool by default")
	}
}

// --- MissingCwd Fallback ---

func TestParseMissingCwd_FallsBackToEmpty(t *testing.T) {
	input, err := hook.ParseStdin(strings.NewReader(missingCwdPayload))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	// cwd should be empty string — caller will use os.Getwd()
	if input.Cwd != "" {
		t.Errorf("expected empty cwd when field is absent, got '%s'", input.Cwd)
	}
}
```

---

## 2. `internal/git/diff_test.go`

```go
package git_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/your-org/nano-guard/internal/git"
)

// initTempRepo creates a temp git repo, writes a file, and commits it.
// Returns the repo dir and a cleanup function.
func initTempRepo(t *testing.T) (string, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "nano-guard-git-test-*")
	if err != nil {
		t.Fatal(err)
	}

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %s", args, out)
		}
	}

	run("init")
	run("config", "user.email", "test@nano-guard.dev")
	run("config", "user.name", "Nano-Guard Test")

	// Write and commit an initial file
	filePath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(filePath, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "initial")

	cleanup := func() { os.RemoveAll(dir) }
	return dir, cleanup
}

// --- ExtractDiff Tests ---

func TestExtractDiff_ValidRepo_WithChange(t *testing.T) {
	dir, cleanup := initTempRepo(t)
	defer cleanup()

	// Modify the committed file
	filePath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(filePath, []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	diff, err := git.ExtractDiff(dir, filePath, 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff == "" {
		t.Fatal("expected non-empty diff, got empty string")
	}
	if !strings.Contains(diff, "+func main()") {
		t.Errorf("expected diff to contain added function, got:\n%s", diff)
	}
}

func TestExtractDiff_CleanRepo_NoChanges(t *testing.T) {
	dir, cleanup := initTempRepo(t)
	defer cleanup()

	filePath := filepath.Join(dir, "main.go")

	diff, err := git.ExtractDiff(dir, filePath, 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff != "" {
		t.Errorf("expected empty diff for clean repo, got:\n%s", diff)
	}
}

func TestExtractDiff_NotARepo_FallsBackToFileContent(t *testing.T) {
	dir, err := os.MkdirTemp("", "nano-guard-nogit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	filePath := filepath.Join(dir, "app.ts")
	content := "export function hello() { return 'world'; }\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	diff, err := git.ExtractDiff(dir, filePath, 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(diff, "hello") {
		t.Errorf("expected fallback to include file content, got:\n%s", diff)
	}
}

func TestExtractDiff_FileNotFound_ReturnsEmpty(t *testing.T) {
	dir, err := os.MkdirTemp("", "nano-guard-nf-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	diff, err := git.ExtractDiff(dir, filepath.Join(dir, "nonexistent.go"), 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff != "" {
		t.Errorf("expected empty diff for missing file, got:\n%s", diff)
	}
}

// --- TruncateDiff Tests ---

func TestTruncateDiff_UnderLimit(t *testing.T) {
	lines := strings.Repeat("+line\n", 50)
	result := git.TruncateDiff(lines, 200)
	if strings.Contains(result, "truncated") {
		t.Error("expected no truncation notice for 50 lines with limit 200")
	}
	if result != lines {
		t.Error("expected diff to be unchanged")
	}
}

func TestTruncateDiff_OverLimit(t *testing.T) {
	lines := strings.Repeat("+line\n", 300)
	result := git.TruncateDiff(lines, 200)
	if !strings.Contains(result, "truncated") {
		t.Error("expected truncation notice in output")
	}
	resultLines := strings.Split(result, "\n")
	// 200 lines + 1 truncation footer + possible empty trailing line
	if len(resultLines) > 203 {
		t.Errorf("expected ~201 lines after truncation, got %d", len(resultLines))
	}
}

func TestTruncateDiff_ExactBoundary(t *testing.T) {
	lines := strings.Repeat("+line\n", 200)
	result := git.TruncateDiff(lines, 200)
	if strings.Contains(result, "truncated") {
		t.Error("expected no truncation for exactly 200 lines")
	}
}
```

---

## 3. `internal/ollama/client_test.go`

```go
package ollama_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/your-org/nano-guard/internal/ollama"
)

// buildMockServer returns a test HTTP server that responds with the given body and status.
func buildMockServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
}

// buildApprovedResponse wraps an inner JSON string the way Ollama wraps it.
func buildApprovedResponse(innerJSON string) string {
	wrapper := map[string]interface{}{
		"model":    "qwen2.5-coder:3b",
		"response": innerJSON,
		"done":     true,
	}
	b, _ := json.Marshal(wrapper)
	return string(b)
}

const approvedInnerJSON = `{"approved":true,"errors":[],"warnings":[],"summary":"Clean change."}`
const rejectedInnerJSON = `{"approved":false,"errors":["UNHANDLED_ERROR: db.insert ignored"],"warnings":[],"summary":"Added saveUser."}`

// --- Evaluate Tests ---

func TestEvaluate_Success_Approved(t *testing.T) {
	srv := buildMockServer(t, 200, buildApprovedResponse(approvedInnerJSON))
	defer srv.Close()

	client := ollama.NewClient(srv.URL, 10*time.Second)
	result, err := client.Evaluate("qwen2.5-coder:3b", "system prompt", "+func main(){}", ollama.GenerateOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != approvedInnerJSON {
		t.Errorf("expected inner JSON '%s', got '%s'", approvedInnerJSON, result)
	}
}

func TestEvaluate_Success_Rejected(t *testing.T) {
	srv := buildMockServer(t, 200, buildApprovedResponse(rejectedInnerJSON))
	defer srv.Close()

	client := ollama.NewClient(srv.URL, 10*time.Second)
	result, err := client.Evaluate("qwen2.5-coder:3b", "system", "+bad code", ollama.GenerateOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != rejectedInnerJSON {
		t.Errorf("expected rejected inner JSON, got '%s'", result)
	}
}

func TestEvaluate_HTTP500_FailOpen(t *testing.T) {
	srv := buildMockServer(t, 500, `{"error":"internal"}`)
	defer srv.Close()

	client := ollama.NewClient(srv.URL, 10*time.Second)
	result, err := client.Evaluate("model", "system", "diff", ollama.GenerateOptions{})
	if err != nil {
		t.Fatalf("expected nil error for fail-open on HTTP 500, got: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result string for HTTP 500, got '%s'", result)
	}
}

func TestEvaluate_ConnectionRefused_FailOpen(t *testing.T) {
	// Point client at a port with nothing listening
	client := ollama.NewClient("http://127.0.0.1:19999", 2*time.Second)
	result, err := client.Evaluate("model", "system", "diff", ollama.GenerateOptions{})
	if err != nil {
		t.Fatalf("expected nil error on connection refused (fail-open), got: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result on connection refused, got '%s'", result)
	}
}

func TestEvaluate_Timeout_FailOpen(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // Simulate hung Ollama
	}))
	defer srv.Close()

	client := ollama.NewClient(srv.URL, 1*time.Second) // 1s timeout
	result, err := client.Evaluate("model", "system", "diff", ollama.GenerateOptions{})
	if err != nil {
		t.Fatalf("expected nil error on timeout (fail-open), got: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result on timeout, got '%s'", result)
	}
}

func TestEvaluate_EmptyResponseField_FailOpen(t *testing.T) {
	body := `{"model":"qwen2.5-coder:3b","response":"","done":true}`
	srv := buildMockServer(t, 200, body)
	defer srv.Close()

	client := ollama.NewClient(srv.URL, 10*time.Second)
	result, err := client.Evaluate("model", "system", "diff", ollama.GenerateOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result for empty response field, got '%s'", result)
	}
}

func TestEvaluate_MalformedOuterJSON_FailOpen(t *testing.T) {
	srv := buildMockServer(t, 200, `{not valid json`)
	defer srv.Close()

	client := ollama.NewClient(srv.URL, 10*time.Second)
	result, err := client.Evaluate("model", "system", "diff", ollama.GenerateOptions{})
	if err != nil {
		t.Fatalf("expected nil error for malformed wrapper (fail-open), got: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result for malformed outer JSON, got '%s'", result)
	}
}

// --- Request Shape Verification Tests ---

func TestEvaluate_RequestPayload_StreamIsFalse(t *testing.T) {
	var captured map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&captured)
		w.Write([]byte(buildApprovedResponse(approvedInnerJSON)))
	}))
	defer srv.Close()

	client := ollama.NewClient(srv.URL, 10*time.Second)
	client.Evaluate("model", "system", "diff", ollama.GenerateOptions{})

	if stream, ok := captured["stream"].(bool); !ok || stream {
		t.Errorf("expected stream=false in request payload, got: %v", captured["stream"])
	}
}

func TestEvaluate_RequestPayload_FormatIsJSON(t *testing.T) {
	var captured map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&captured)
		w.Write([]byte(buildApprovedResponse(approvedInnerJSON)))
	}))
	defer srv.Close()

	client := ollama.NewClient(srv.URL, 10*time.Second)
	client.Evaluate("model", "system", "diff", ollama.GenerateOptions{})

	if captured["format"] != "json" {
		t.Errorf("expected format='json' in request payload, got: %v", captured["format"])
	}
}

func TestEvaluate_RequestPayload_TemperatureIsZero(t *testing.T) {
	var captured map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&captured)
		w.Write([]byte(buildApprovedResponse(approvedInnerJSON)))
	}))
	defer srv.Close()

	client := ollama.NewClient(srv.URL, 10*time.Second)
	client.Evaluate("model", "system", "diff", ollama.GenerateOptions{Temperature: 0.0})

	options, _ := captured["options"].(map[string]interface{})
	if options["temperature"] != float64(0) {
		t.Errorf("expected temperature=0, got: %v", options["temperature"])
	}
}
```

---

## 4. `internal/evaluator/evaluator_test.go`

```go
package evaluator_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/your-org/nano-guard/internal/evaluator"
)

// --- ParseResult Tests ---

func TestParseResult_Approved(t *testing.T) {
	input := `{"approved":true,"errors":[],"warnings":[],"summary":"Clean change."}`
	result, err := evaluator.ParseResult(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Approved {
		t.Error("expected approved=true")
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got: %v", result.Errors)
	}
}

func TestParseResult_Rejected(t *testing.T) {
	input := `{"approved":false,"errors":["UNHANDLED_ERROR: db.insert ignored"],"warnings":[],"summary":"Bad code."}`
	result, err := evaluator.ParseResult(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Approved {
		t.Error("expected approved=false")
	}
	if len(result.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(result.Errors))
	}
}

func TestParseResult_MultipleErrors(t *testing.T) {
	input := `{"approved":false,"errors":["UNHANDLED_ERROR: foo","DEBUG_LOG: bar"],"warnings":[],"summary":"Two issues."}`
	result, err := evaluator.ParseResult(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(result.Errors))
	}
}

func TestParseResult_InvalidJSON(t *testing.T) {
	_, err := evaluator.ParseResult("not json")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestParseResult_EmptyString(t *testing.T) {
	_, err := evaluator.ParseResult("")
	if err == nil {
		t.Fatal("expected error for empty string, got nil")
	}
}

func TestParseResult_MissingApprovedField(t *testing.T) {
	// Missing "approved" key — should default to false (conservative)
	input := `{"errors":[],"warnings":[],"summary":"unknown"}`
	result, err := evaluator.ParseResult(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Approved {
		t.Error("expected approved=false when field missing (conservative default)")
	}
}

// --- FormatStderr Tests ---

func TestFormatStderr_ContainsRequiredSections(t *testing.T) {
	result := &evaluator.EvaluationResult{
		Approved: false,
		Errors:   []string{"UNHANDLED_ERROR: db.insert ignored in saveUser()"},
		Warnings: []string{},
		Summary:  "Added saveUser function.",
	}

	var buf bytes.Buffer
	evaluator.FormatStderr(&buf, result)
	output := buf.String()

	checks := []string{"🚨", "Summary:", "Errors:", "UNHANDLED_ERROR"}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("expected stderr to contain '%s', got:\n%s", check, output)
		}
	}
}

func TestFormatStderr_NumberedErrors(t *testing.T) {
	result := &evaluator.EvaluationResult{
		Approved: false,
		Errors:   []string{"UNHANDLED_ERROR: foo", "DEBUG_LOG: bar"},
		Warnings: []string{},
		Summary:  "Two issues.",
	}

	var buf bytes.Buffer
	evaluator.FormatStderr(&buf, result)
	output := buf.String()

	if !strings.Contains(output, "[1]") || !strings.Contains(output, "[2]") {
		t.Errorf("expected numbered errors [1] and [2] in stderr, got:\n%s", output)
	}
}

func TestFormatStderr_WarningsShownWhenPresent(t *testing.T) {
	result := &evaluator.EvaluationResult{
		Approved: false,
		Errors:   []string{"UNHANDLED_ERROR: foo"},
		Warnings: []string{"minor style issue"},
		Summary:  "Change with warnings.",
	}

	var buf bytes.Buffer
	evaluator.FormatStderr(&buf, result)
	output := buf.String()

	if !strings.Contains(output, "Warnings:") {
		t.Errorf("expected 'Warnings:' section in stderr, got:\n%s", output)
	}
	if !strings.Contains(output, "minor style issue") {
		t.Errorf("expected warning text in stderr, got:\n%s", output)
	}
}

func TestFormatStderr_NoWarningsSectionWhenEmpty(t *testing.T) {
	result := &evaluator.EvaluationResult{
		Approved: false,
		Errors:   []string{"UNHANDLED_ERROR: foo"},
		Warnings: []string{},
		Summary:  "Clean errors, no warnings.",
	}

	var buf bytes.Buffer
	evaluator.FormatStderr(&buf, result)
	output := buf.String()

	if strings.Contains(output, "Warnings:") {
		t.Errorf("expected NO 'Warnings:' section when warnings are empty, got:\n%s", output)
	}
}
```

---

## 5. `internal/config/config_test.go`

```go
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/your-org/nano-guard/internal/config"
)

// writeConfigFile writes a JSON config to a temp file and returns its path.
func writeConfigFile(t *testing.T, dir, filename, content string) string {
	t.Helper()
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// --- Default Values ---

func TestLoadDefaults_AllFieldsPresent(t *testing.T) {
	// No config files on disk, no env vars set
	cfg := config.LoadDefaults()

	if cfg.Model != "qwen2.5-coder:3b" {
		t.Errorf("expected default model 'qwen2.5-coder:3b', got '%s'", cfg.Model)
	}
	if cfg.OllamaHost != "http://localhost:11434" {
		t.Errorf("expected default ollama_host, got '%s'", cfg.OllamaHost)
	}
	if cfg.TimeoutSeconds != 30 {
		t.Errorf("expected default timeout 30, got %d", cfg.TimeoutSeconds)
	}
	if cfg.MaxDiffLines != 200 {
		t.Errorf("expected default max_diff_lines 200, got %d", cfg.MaxDiffLines)
	}
	if !cfg.FailOpen {
		t.Error("expected fail_open=true by default")
	}
	if !cfg.Rules.UnhandledErrors {
		t.Error("expected rules.unhandled_errors=true by default")
	}
	if !cfg.Rules.DebugLogs {
		t.Error("expected rules.debug_logs=true by default")
	}
}

// --- Project Config Override ---

func TestLoadProjectConfig_OverridesDefaults(t *testing.T) {
	dir, err := os.MkdirTemp("", "nano-guard-cfg-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	writeConfigFile(t, dir, "nano-guard.config.json", `{
		"model": "gemma2:2b",
		"timeout_seconds": 15,
		"max_diff_lines": 100
	}`)

	cfg, err := config.Load(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Model != "gemma2:2b" {
		t.Errorf("expected model 'gemma2:2b', got '%s'", cfg.Model)
	}
	if cfg.TimeoutSeconds != 15 {
		t.Errorf("expected timeout 15, got %d", cfg.TimeoutSeconds)
	}
	if cfg.MaxDiffLines != 100 {
		t.Errorf("expected max_diff_lines 100, got %d", cfg.MaxDiffLines)
	}
	// Non-overridden field should retain default
	if cfg.OllamaHost != "http://localhost:11434" {
		t.Errorf("expected default ollama_host, got '%s'", cfg.OllamaHost)
	}
}

// --- Environment Variable Overrides ---

func TestEnvVarOverride_Model(t *testing.T) {
	t.Setenv("NANO_GUARD_MODEL", "llama3.2:3b")
	cfg, _ := config.Load("", "")
	if cfg.Model != "llama3.2:3b" {
		t.Errorf("expected model from env 'llama3.2:3b', got '%s'", cfg.Model)
	}
}

func TestEnvVarOverride_OllamaHost(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "http://custom-host:11434")
	cfg, _ := config.Load("", "")
	if cfg.OllamaHost != "http://custom-host:11434" {
		t.Errorf("expected ollama_host from env, got '%s'", cfg.OllamaHost)
	}
}

// --- IgnorePaths Glob Matching ---

func TestIgnorePathsGlob_Match(t *testing.T) {
	cfg := &config.Config{
		IgnorePaths: []string{"**/*.test.ts", "**/vendor/**"},
	}
	if !cfg.ShouldIgnore("src/utils.test.ts") {
		t.Error("expected 'src/utils.test.ts' to be ignored")
	}
	if !cfg.ShouldIgnore("vendor/some/package/main.go") {
		t.Error("expected 'vendor/...' path to be ignored")
	}
}

func TestIgnorePathsGlob_NoMatch(t *testing.T) {
	cfg := &config.Config{
		IgnorePaths: []string{"**/*.test.ts"},
	}
	if cfg.ShouldIgnore("src/server.go") {
		t.Error("expected 'src/server.go' NOT to be ignored")
	}
	if cfg.ShouldIgnore("src/utils.ts") {
		t.Error("expected 'src/utils.ts' NOT to be ignored (no .test. in name)")
	}
}

func TestIgnorePathsGlob_EmptyList(t *testing.T) {
	cfg := &config.Config{IgnorePaths: []string{}}
	if cfg.ShouldIgnore("anything.go") {
		t.Error("expected no files to be ignored when ignore_paths is empty")
	}
}

// --- Malformed Config ---

func TestInvalidConfigJSON_FallsBackToDefaults(t *testing.T) {
	dir, err := os.MkdirTemp("", "nano-guard-badcfg-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	writeConfigFile(t, dir, "nano-guard.config.json", `{this is not valid json`)

	// Should not return an error — falls back to defaults
	cfg, err := config.Load(dir, "")
	if err != nil {
		t.Fatalf("expected fallback to defaults on bad JSON, got error: %v", err)
	}
	if cfg.Model != "qwen2.5-coder:3b" {
		t.Errorf("expected default model after bad config, got '%s'", cfg.Model)
	}
}
```

---

## 6. `init/src/__tests__/settings.test.ts`

```typescript
import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';
import { patchSettings } from '../settings';

// Helper: create a temp .claude directory and return its path
function makeTempDir(): string {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'nano-guard-test-'));
}

const EXPECTED_HOOK = {
  type: 'command',
  command: `${os.homedir()}/.local/bin/nano-guard`,
  timeout: 30,
};

const EXPECTED_MATCHER = 'write_to_file|replace_file_content|multi_replace_file_content|Write|Edit';

describe('patchSettings', () => {

  test('creates .claude/settings.json when file does not exist', () => {
    const dir = makeTempDir();
    const claudeDir = path.join(dir, '.claude');

    patchSettings(claudeDir);

    const settings = JSON.parse(fs.readFileSync(path.join(claudeDir, 'settings.json'), 'utf-8'));
    expect(settings.hooks.PostToolUse).toBeDefined();
    expect(settings.hooks.PostToolUse[0].matcher).toBe(EXPECTED_MATCHER);
    expect(settings.hooks.PostToolUse[0].hooks[0]).toMatchObject(EXPECTED_HOOK);

    fs.rmSync(dir, { recursive: true });
  });

  test('patches empty settings.json correctly', () => {
    const dir = makeTempDir();
    const claudeDir = path.join(dir, '.claude');
    fs.mkdirSync(claudeDir, { recursive: true });
    fs.writeFileSync(path.join(claudeDir, 'settings.json'), '{}');

    patchSettings(claudeDir);

    const settings = JSON.parse(fs.readFileSync(path.join(claudeDir, 'settings.json'), 'utf-8'));
    expect(settings.hooks.PostToolUse).toHaveLength(1);

    fs.rmSync(dir, { recursive: true });
  });

  test('appends to existing PostToolUse hooks without overwriting them', () => {
    const dir = makeTempDir();
    const claudeDir = path.join(dir, '.claude');
    fs.mkdirSync(claudeDir, { recursive: true });

    const existing = {
      hooks: {
        PostToolUse: [
          { matcher: 'Bash', hooks: [{ type: 'command', command: 'some-other-tool' }] },
        ],
      },
    };
    fs.writeFileSync(path.join(claudeDir, 'settings.json'), JSON.stringify(existing, null, 2));

    patchSettings(claudeDir);

    const settings = JSON.parse(fs.readFileSync(path.join(claudeDir, 'settings.json'), 'utf-8'));
    expect(settings.hooks.PostToolUse).toHaveLength(2);
    // Original hook still present
    expect(settings.hooks.PostToolUse[0].matcher).toBe('Bash');
    // Nano-Guard hook appended
    expect(settings.hooks.PostToolUse[1].matcher).toBe(EXPECTED_MATCHER);

    fs.rmSync(dir, { recursive: true });
  });

  test('is idempotent — does not add duplicate hook if already present', () => {
    const dir = makeTempDir();
    const claudeDir = path.join(dir, '.claude');
    fs.mkdirSync(claudeDir, { recursive: true });
    fs.writeFileSync(path.join(claudeDir, 'settings.json'), '{}');

    // Run twice
    patchSettings(claudeDir);
    patchSettings(claudeDir);

    const settings = JSON.parse(fs.readFileSync(path.join(claudeDir, 'settings.json'), 'utf-8'));
    expect(settings.hooks.PostToolUse).toHaveLength(1);

    fs.rmSync(dir, { recursive: true });
  });

  test('preserves all other top-level settings keys', () => {
    const dir = makeTempDir();
    const claudeDir = path.join(dir, '.claude');
    fs.mkdirSync(claudeDir, { recursive: true });

    const existing = { model: 'claude-sonnet-4', permissions: { allow: ['*'] } };
    fs.writeFileSync(path.join(claudeDir, 'settings.json'), JSON.stringify(existing, null, 2));

    patchSettings(claudeDir);

    const settings = JSON.parse(fs.readFileSync(path.join(claudeDir, 'settings.json'), 'utf-8'));
    expect(settings.model).toBe('claude-sonnet-4');
    expect(settings.permissions.allow).toContain('*');

    fs.rmSync(dir, { recursive: true });
  });

});
```

---

## 7. Running All Tests

```bash
# All Go unit tests (no Ollama required)
go test ./internal/... -v -race -cover

# Generate HTML coverage report
go test ./internal/... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html

# Integration tests only (requires Ollama running with qwen2.5-coder:3b)
go test ./e2e/... -tags integration -v -timeout 120s

# TypeScript init unit tests
cd init && npm ci && npm test

# All linting
go vet ./...
cd init && npx tsc --noEmit
```
