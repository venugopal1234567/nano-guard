# Nano-Guard: Ollama API Integration

This document specifies the exact HTTP API call contract between the Nano-Guard Go binary and the local Ollama instance.

---

## 1. Endpoint

```
POST http://localhost:11434/api/generate
Content-Type: application/json
```

The host is configurable via `ollama_host` in the config or the `OLLAMA_HOST` environment variable.

---

## 2. Request Payload

```json
{
  "model": "qwen2.5-coder:3b",
  "system": "<system prompt — see spec 07>",
  "prompt": "Analyze this git diff:\n\n<diff content here>",
  "stream": false,
  "format": "json",
  "options": {
    "temperature": 0.0,
    "num_predict": 512
  }
}
```

### Key Fields

| Field | Value | Reason |
|:---|:---|:---|
| `stream` | `false` | Wait for complete response before parsing |
| `format` | `"json"` | **Critical**: Forces Ollama to constrain token logits to valid JSON only |
| `temperature` | `0.0` | Deterministic output — no randomness in code validation |
| `num_predict` | `512` | Caps response length to prevent runaway generation |

---

## 3. Response Schema

```json
{
  "model": "qwen2.5-coder:3b",
  "created_at": "2025-01-01T00:00:00Z",
  "response": "{\"approved\":true,\"errors\":[],\"warnings\":[],\"summary\":\"...\"}",
  "done": true,
  "total_duration": 1234567890,
  "eval_count": 87
}
```

The LLM evaluation JSON lives inside the `response` field as a **string** — it must be unmarshalled a second time.

---

## 4. Error Handling

| Failure Mode | Nano-Guard Action |
|:---|:---|
| Ollama not running (connection refused) | Log warning to stderr → `exit(0)` fail-open |
| HTTP status >= 400 | Log status code to stderr → `exit(0)` fail-open |
| Response `done: false` | Treat as incomplete → `exit(0)` fail-open |
| `response` field is not valid JSON | Log raw response for debugging → `exit(0)` fail-open |
| Timeout exceeded (`timeout_seconds`) | Cancel request → `exit(0)` fail-open |

> [!IMPORTANT]
> Nano-Guard must **never** block development. All failure paths exit with `0` and log a warning to stderr so the developer is aware but not stalled.

---

## 5. Go Module Interface

```go
// internal/ollama/client.go

package ollama

type Client struct {
    Host    string
    Timeout time.Duration
}

// Evaluate sends the diff and system prompt to Ollama.
// Returns the raw JSON string from the "response" field.
// Returns ("", nil) on soft failures (network, timeout) — caller should exit(0).
func (c *Client) Evaluate(model, systemPrompt, diff string, opts GenerateOptions) (string, error)

type GenerateOptions struct {
    Temperature float64
    MaxTokens   int
}
```

---

## 6. Recommended Models (by hardware)

| Machine | Recommended Model | RAM Needed | Notes |
|:---|:---|:---|:---|
| Any laptop | `qwen2.5-coder:3b` | ~2.5 GB | Best code quality for size |
| Low-end / fast machine | `gemma2:2b` | ~1.8 GB | Fastest inference |
| High-end / accuracy | `qwen2.5-coder:7b` | ~5 GB | Best accuracy |
