# Nano-Guard: Config Schema

The user-facing config file is `nano-guard.config.json`, placed at the project root or `~/.config/nano-guard/config.json` for global use.

---

## Full Schema

```json
{
  "$schema": "https://nano-guard.dev/schema/config.json",

  "model": "qwen2.5-coder:3b",
  "ollama_host": "http://localhost:11434",
  "timeout_seconds": 30,
  "max_diff_lines": 200,
  "fail_open": true,

  "rules": {
    "unhandled_errors": true,
    "debug_logs": true,
    "type_safety": true,
    "placeholder_stubs": true
  },

  "ignore_paths": [
    "**/*.test.ts",
    "**/*.spec.go",
    "**/vendor/**",
    "**/node_modules/**"
  ]
}
```

---

## Field Reference

| Field | Type | Default | Description |
|:---|:---|:---|:---|
| `model` | string | `"qwen2.5-coder:3b"` | Ollama model tag to use for evaluation |
| `ollama_host` | string | `"http://localhost:11434"` | Base URL of the local Ollama server |
| `timeout_seconds` | int | `30` | Max seconds to wait for Ollama response before fail-open |
| `max_diff_lines` | int | `200` | Max lines of git diff to send to the LLM (truncated beyond this) |
| `fail_open` | bool | `true` | If `true`, any hook error (network, parse) exits with `0` silently |
| `rules.unhandled_errors` | bool | `true` | Check for ignored error return values or missing try/catch |
| `rules.debug_logs` | bool | `true` | Check for leftover `console.log`, `fmt.Println`, `print()` etc. |
| `rules.type_safety` | bool | `true` | Check for `as any`, unsafe casts, or missing type annotations |
| `rules.placeholder_stubs` | bool | `true` | Check for `TODO`, `FIXME`, `panic("implement me")` |
| `ignore_paths` | string[] | `[]` | Glob patterns. Files matching these are skipped entirely. |

---

## Config Load Priority (highest → lowest)

1. Environment variables (e.g., `NANO_GUARD_MODEL`, `OLLAMA_HOST`)
2. Project-level `./nano-guard.config.json`
3. Global `~/.config/nano-guard/config.json`
4. Built-in defaults (table above)
