# Nano-Guard: Testing Plan

This document covers the complete test strategy for Nano-Guard — from unit tests on individual Go modules to end-to-end self-correction loop validation.

---

## 1. Testing Layers

```
┌─────────────────────────────────────────────────────┐
│  Layer 4 – E2E / Integration (real Claude Code IDE) │
├─────────────────────────────────────────────────────┤
│  Layer 3 – Integration (Ollama running locally)      │
├─────────────────────────────────────────────────────┤
│  Layer 2 – Unit Tests (Go, mocked HTTP)              │
├─────────────────────────────────────────────────────┤
│  Layer 1 – Static: go vet, golangci-lint, tsc --noEmit │
└─────────────────────────────────────────────────────┘
```

---

## 2. Test File Layout

```
nano-guard/
├── internal/
│   ├── hook/
│   │   ├── hook.go
│   │   └── hook_test.go          ← Unit: stdin parsing
│   ├── git/
│   │   ├── diff.go
│   │   └── diff_test.go          ← Unit: diff extraction + truncation
│   ├── ollama/
│   │   ├── client.go
│   │   └── client_test.go        ← Unit: HTTP client (mock server)
│   ├── evaluator/
│   │   ├── evaluator.go
│   │   └── evaluator_test.go     ← Unit: result parsing, exit decision
│   └── config/
│       ├── config.go
│       └── config_test.go        ← Unit: config load priority
│
├── testdata/
│   ├── payloads/
│   │   ├── valid_write.json       ← Sample PostToolUse stdin payload
│   │   ├── valid_edit.json
│   │   ├── empty.json
│   │   ├── read_tool.json         ← Should be skipped (non-write tool)
│   │   └── malformed.json
│   ├── diffs/
│   │   ├── approved_clean.diff
│   │   ├── rejected_unhandled_error.diff
│   │   ├── rejected_debug_log.diff
│   │   ├── rejected_placeholder.diff
│   │   ├── rejected_type_unsafe.diff
│   │   └── large_500_lines.diff   ← Test truncation
│   └── responses/
│       ├── approved.json          ← Mock Ollama response body
│       └── rejected_multi.json
│
├── e2e/
│   └── e2e_test.go               ← Full binary invocation tests
│
└── init/
    └── src/
        └── __tests__/
            └── settings.test.ts  ← Unit: settings.json patch logic
```

---

## 3. Unit Tests

### 3.1 `internal/hook` — stdin Parsing

**File**: `internal/hook/hook_test.go`

| Test Name | Input | Expected Result |
|:---|:---|:---|
| `TestParseValidWritePayload` | `testdata/payloads/valid_write.json` | Returns `HookInput` with correct `ToolName`, `Cwd`, `TargetFile` |
| `TestParseValidEditPayload` | `testdata/payloads/valid_edit.json` | Returns `HookInput` with `ToolName = "replace_file_content"` |
| `TestParseEmptyStdin` | Empty byte slice | Returns `ErrEmptyInput`, caller should `exit(0)` |
| `TestParseMalformedJSON` | `testdata/payloads/malformed.json` | Returns `ErrInvalidJSON`, caller should `exit(0)` |
| `TestExtractTargetFile_TargetFileKey` | `tool_input: {TargetFile: "/a/b.go"}` | Returns `/a/b.go` |
| `TestExtractTargetFile_AbsolutePathKey` | `tool_input: {AbsolutePath: "/a/b.go"}` | Returns `/a/b.go` |
| `TestExtractTargetFile_NoKey` | `tool_input: {}` | Returns `""` |
| `TestIsWriteTool` | `"write_to_file"`, `"Read"`, `"Bash"`, `"Edit"` | Returns `true`, `false`, `false`, `true` |

---

### 3.2 `internal/git` — Diff Extraction

**File**: `internal/git/diff_test.go`

| Test Name | Setup | Expected Result |
|:---|:---|:---|
| `TestExtractDiff_ValidRepo` | Temp git repo with uncommitted change | Returns non-empty unified diff string |
| `TestExtractDiff_CleanRepo` | Temp git repo, no changes | Returns `""` → caller exits `0` |
| `TestExtractDiff_NotARepo` | Non-git temp directory | Falls back to reading file content directly |
| `TestExtractDiff_FileNotFound` | Non-existent target file, no git | Returns `""` |
| `TestTruncateDiff_Under200Lines` | Diff with 50 lines | Returns diff unchanged |
| `TestTruncateDiff_Over200Lines` | Diff with 300 lines | Returns first 200 lines + truncation footer |
| `TestTruncateDiff_ExactBoundary` | Diff with exactly 200 lines | Returns all 200 lines, no footer |

---

### 3.3 `internal/ollama` — HTTP Client

**File**: `internal/ollama/client_test.go`  
Uses `httptest.NewServer()` to mock the Ollama server — no real Ollama needed.

| Test Name | Mock Server Behavior | Expected Result |
|:---|:---|:---|
| `TestEvaluate_Success` | Returns valid JSON `response` field | Returns parsed JSON string |
| `TestEvaluate_ConnectionRefused` | Server not started | Returns `""`, `nil` (fail-open) |
| `TestEvaluate_HTTP500` | Returns HTTP 500 | Returns `""`, `nil` (fail-open) |
| `TestEvaluate_MalformedResponse` | Returns `{"response": "not-json"}` | Returns `""`, error logged |
| `TestEvaluate_Timeout` | Server sleeps 60s | Cancels after `timeout_seconds`, returns `""`, `nil` |
| `TestEvaluate_EmptyResponse` | Returns `{"response": ""}` | Returns `""`, caller exits `0` |
| `TestBuildPayload_TemperatureZero` | — | Verifies `options.temperature == 0.0` in request |
| `TestBuildPayload_StreamFalse` | — | Verifies `stream == false` in request |
| `TestBuildPayload_FormatJSON` | — | Verifies `format == "json"` in request |

---

### 3.4 `internal/evaluator` — Result Parsing & Decision

**File**: `internal/evaluator/evaluator_test.go`

| Test Name | Input JSON | Expected Exit | Expected stderr |
|:---|:---|:---|:---|
| `TestDecide_Approved` | `{"approved":true,"errors":[],...}` | `0` | None |
| `TestDecide_Rejected_OneError` | `{"approved":false,"errors":["UNHANDLED_ERROR: ..."],...}` | `2` | Contains error string |
| `TestDecide_Rejected_MultipleErrors` | `{"approved":false,"errors":["A","B"],...}` | `2` | Contains both errors, numbered |
| `TestDecide_ApprovedWithWarnings` | `{"approved":true,"warnings":["minor style"]}` | `0` | None (warnings don't block) |
| `TestDecide_InvalidJSON` | `"not json"` | `0` | Warning logged |
| `TestDecide_MissingApprovedField` | `{"errors":[]}` | `0` | Warning logged (fail-open) |
| `TestFormatStderr_Structure` | Rejected result | `2` | Output contains `🚨`, `Summary:`, `Errors:` headers |

---

### 3.5 `internal/config` — Config Loading

**File**: `internal/config/config_test.go`

| Test Name | Setup | Expected Result |
|:---|:---|:---|
| `TestLoadDefaults` | No config files present | Returns all defaults (model, timeout, etc.) |
| `TestLoadProjectConfig` | `./nano-guard.config.json` present | Overrides defaults with file values |
| `TestLoadGlobalConfig` | `~/.config/nano-guard/config.json` present | Overrides defaults |
| `TestEnvVarOverride_Model` | `NANO_GUARD_MODEL=gemma2:2b` | Returns `gemma2:2b` regardless of file |
| `TestEnvVarOverride_Host` | `OLLAMA_HOST=http://custom:11434` | Uses custom host |
| `TestIgnorePathsGlob_Match` | `ignore_paths: ["**/*.test.ts"]`, file `foo.test.ts` | Returns `shouldSkip = true` |
| `TestIgnorePathsGlob_NoMatch` | `ignore_paths: ["**/*.test.ts"]`, file `foo.go` | Returns `shouldSkip = false` |
| `TestInvalidConfigJSON` | Malformed JSON in config file | Falls back to defaults, logs warning |

---

## 4. Integration Tests (Ollama Running)

These tests require a live local Ollama instance with `qwen2.5-coder:3b` pulled. Run with build tag `//go:build integration`.

**File**: `e2e/e2e_test.go`

```bash
# Run integration tests only
go test ./e2e/... -tags integration -timeout 120s
```

| Test Name | Input | Expected Outcome |
|:---|:---|:---|
| `TestIntegration_CleanDiff_Approved` | `testdata/diffs/approved_clean.diff` | LLM returns `approved: true`, exit `0` |
| `TestIntegration_UnhandledError_Rejected` | `testdata/diffs/rejected_unhandled_error.diff` | LLM returns `approved: false`, exit `2`, stderr contains `UNHANDLED_ERROR` |
| `TestIntegration_DebugLog_Rejected` | `testdata/diffs/rejected_debug_log.diff` | exit `2`, stderr contains `DEBUG_LOG` |
| `TestIntegration_Placeholder_Rejected` | `testdata/diffs/rejected_placeholder.diff` | exit `2`, stderr contains `PLACEHOLDER` |
| `TestIntegration_TypeUnsafe_Rejected` | `testdata/diffs/rejected_type_unsafe.diff` | exit `2`, stderr contains `TYPE_UNSAFE` |
| `TestIntegration_LargeDiff_Truncated` | `testdata/diffs/large_500_lines.diff` | Sends only 200 lines to Ollama, returns valid JSON |
| `TestIntegration_OllamaDown_FailOpen` | Ollama stopped manually | exit `0`, stderr contains `[nano-guard warn] Ollama unreachable` |

---

## 5. End-to-End Test (Real IDE Hook)

These tests validate the full feedback loop with a real Claude Code or Antigravity session. They are **manual** and serve as the final acceptance criteria.

### Test Scenario A: Happy Path (No Errors)
1. Ask the agent to write a clean, well-typed Go function with error handling.
2. Agent executes `write_to_file`.
3. **Expected**: Nano-Guard exits `0` silently. Agent proceeds without interruption. IDE terminal shows nothing from nano-guard.

### Test Scenario B: Self-Correction Loop (Unhandled Error)
1. Deliberately instruct the agent to write code that ignores an error return value.
2. Agent executes `write_to_file`.
3. **Expected**: Nano-Guard exits `2`. Agent's next message acknowledges the error from stderr and rewrites the function with proper error handling.

### Test Scenario C: Debug Log Cleanup
1. Instruct the agent to add a feature with a `console.log` debug statement.
2. **Expected**: Nano-Guard exits `2`. Agent removes `console.log` in the next edit. Hook passes on the second attempt with exit `0`.

### Test Scenario D: Ollama Offline (Fail-Open)
1. Stop Ollama (`ollama stop` or kill the process).
2. Ask the agent to write any file.
3. **Expected**: Nano-Guard exits `0` silently (or with a soft warning). Agent is **not** blocked. Development continues normally.

### Test Scenario E: Ignored Path
1. Add `"**/*.test.ts"` to `ignore_paths` in `nano-guard.config.json`.
2. Ask the agent to write a test file `foo.test.ts` with intentional issues.
3. **Expected**: Nano-Guard exits `0` immediately without evaluating the file.

---

## 6. Init CLI Tests (TypeScript)

**File**: `init/src/__tests__/settings.test.ts`  
Run with: `cd init && npm test`

| Test Name | Setup | Expected Outcome |
|:---|:---|:---|
| `patchSettings_NoExistingFile` | No `.claude/` directory | Creates `.claude/settings.json` with hook block |
| `patchSettings_EmptyExistingFile` | `.claude/settings.json` = `{}` | Writes hook block into empty object |
| `patchSettings_ExistingHooks` | File has existing `PostToolUse` hooks | Appends nano-guard entry, preserves existing hooks |
| `patchSettings_AlreadyPatched` | nano-guard hook already present | Idempotent — does not add duplicate |
| `checkPrereqs_OllamaFound` | `ollama` on PATH | Returns `true` |
| `checkPrereqs_OllamaMissing` | `ollama` not on PATH | Returns `false`, logs install instructions |

---

## 7. Static Analysis & Linting

Run these on every PR/commit:

```bash
# Go static analysis
go vet ./...
golangci-lint run

# TypeScript type check
cd init && npx tsc --noEmit

# Go test with race detector
go test -race ./...

# Go test with coverage report
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

**Target coverage**: ≥ 80% on `internal/` packages.

---

## 8. Test Data Fixtures

### `testdata/payloads/valid_write.json`
```json
{
  "hook_event_name": "PostToolUse",
  "session_id": "test-session-001",
  "tool_name": "write_to_file",
  "tool_input": {
    "TargetFile": "/tmp/test-project/src/server.go",
    "CodeContent": "package main\n\nfunc main() {}"
  },
  "tool_response": { "success": true },
  "cwd": "/tmp/test-project"
}
```

### `testdata/responses/approved.json`
```json
{
  "model": "qwen2.5-coder:3b",
  "response": "{\"approved\":true,\"errors\":[],\"warnings\":[],\"summary\":\"Added main function.\"}",
  "done": true
}
```

### `testdata/responses/rejected_multi.json`
```json
{
  "model": "qwen2.5-coder:3b",
  "response": "{\"approved\":false,\"errors\":[\"UNHANDLED_ERROR: db.insert() return value ignored\",\"DEBUG_LOG: console.log found in saveUser()\"],\"warnings\":[],\"summary\":\"Added saveUser function.\"}",
  "done": true
}
```

---

## 9. CI Pipeline Definition

```yaml
# .github/workflows/ci.yml

name: CI
on: [push, pull_request]

jobs:
  test-go:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: go vet ./...
      - run: go test -race -coverprofile=coverage.out ./...
      - run: go tool cover -func=coverage.out

  lint-go:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: golangci/golangci-lint-action@v6

  test-init:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '20'
      - run: cd init && npm ci && npm test
      - run: cd init && npx tsc --noEmit
```
