# Nano-Guard: IDE stdin Contract (PostToolUse Payload)

This document defines the exact JSON schema that Claude Code and Antigravity pipe into `stdin` when a `PostToolUse` event fires. Nano-Guard must parse this reliably.

---

## 1. Claude Code — PostToolUse Payload

```json
{
  "hook_event_name": "PostToolUse",
  "session_id": "abc-123-def-456",
  "tool_name": "write_to_file",
  "tool_input": {
    "TargetFile": "/home/user/project/src/server.go",
    "CodeContent": "package main\n\nfunc main() {...}"
  },
  "tool_response": {
    "filePath": "/home/user/project/src/server.go",
    "success": true
  },
  "cwd": "/home/user/project"
}
```

### Known `tool_name` Values to Match

| Tool Name | Trigger? | Notes |
|:---|:---|:---|
| `write_to_file` | ✅ Yes | New file creation |
| `replace_file_content` | ✅ Yes | Full file replacement |
| `multi_replace_file_content` | ✅ Yes | Partial/multi-block edit |
| `Write` | ✅ Yes | Alias used in some Claude Code versions |
| `Edit` | ✅ Yes | Alias used in some Claude Code versions |
| `Bash` | ⚠️ Optional | Only trigger if command contains `>` redirection or `tee` |
| `Read` | ❌ No | Read-only, skip |

---

## 2. Field Extraction Map (Go struct)

```go
type HookInput struct {
    HookEventName string                 `json:"hook_event_name"`
    SessionID     string                 `json:"session_id"`
    ToolName      string                 `json:"tool_name"`
    ToolInput     map[string]interface{} `json:"tool_input"`
    ToolResponse  map[string]interface{} `json:"tool_response"`
    Cwd           string                 `json:"cwd"`
}
```

### File Path Extraction Priority (from `tool_input`)

```
1. tool_input["TargetFile"]       → write_to_file / replace_file_content
2. tool_input["AbsolutePath"]     → some Edit tool variants
3. tool_input["path"]             → generic fallback
4. Derive from tool_response["filePath"] if available
5. If none found → skip evaluation, exit(0)
```

---

## 3. Edge Cases

| Scenario | Nano-Guard Behavior |
|:---|:---|
| `tool_name` is not a write tool | Exit `0` immediately — no evaluation needed |
| File path not resolvable | Exit `0` — cannot evaluate without context |
| File matches `ignore_paths` glob | Exit `0` silently |
| `cwd` field missing | Use `os.Getwd()` as fallback |
| stdin is empty or malformed JSON | Exit `0` (fail-open) |
