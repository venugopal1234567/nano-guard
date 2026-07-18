# Nano-Guard: Exit Codes & Error Contract

This is the definitive contract between Nano-Guard and the IDE agent.

---

## 1. Exit Code Table

| Exit Code | Name | Condition | IDE Effect |
|:---|:---|:---|:---|
| `0` | **Success / Approved** | LLM approved the diff, OR fail-open triggered | Agent continues normally. No tokens consumed. |
| `2` | **Rejected** | LLM returned `approved: false` | IDE injects stderr into agent context. Agent self-corrects. |
| `1` | **Reserved / Unused** | Not emitted by Nano-Guard | N/A |

> [!NOTE]
> Exit code `1` is intentionally avoided. Claude Code treats non-zero, non-2 exit codes as non-blocking informational errors. We only want `0` (proceed) and `2` (block + correct).

---

## 2. stderr Output Format on Rejection

When exiting with code `2`, Nano-Guard writes a structured human+machine readable block to `stderr`:

```
🚨 Nano-Guard: Code verification failed.

Summary: Added saveUser function that inserts user into the database.

Errors (must fix before proceeding):
  [1] UNHANDLED_ERROR: db.users.insert() is async but called without await in saveUser()
  [2] DEBUG_LOG: console.log() left in saveUser()

Warnings (optional):
  (none)

Fix the issues above and re-run your edit.
```

### Why this format?
- **Machine-parseable prefix**: Each error is numbered and rule-prefixed (`UNHANDLED_ERROR:`), making it trivial for the agent to reference a specific issue.
- **Human-readable**: A developer watching the terminal can understand immediately what went wrong.
- **Token-efficient**: The entire block is typically 80–150 tokens — far cheaper than another full agent turn.

---

## 3. Fail-Open Scenarios (always exit 0)

All of the following are silent exits with code `0`:

| Scenario | Log to stderr? |
|:---|:---|
| stdin empty or not valid JSON | No (silent) |
| `tool_name` is not a write tool | No (silent) |
| File matches `ignore_paths` | No (silent) |
| Diff is empty (no changes detected) | No (silent) |
| Ollama connection refused | Yes — `[nano-guard warn] Ollama unreachable, skipping validation` |
| Ollama HTTP error (4xx/5xx) | Yes — `[nano-guard warn] Ollama returned HTTP {status}` |
| Ollama response timeout | Yes — `[nano-guard warn] Ollama timed out after {N}s` |
| LLM response is not valid JSON | Yes — `[nano-guard warn] LLM returned non-JSON output` |

> [!IMPORTANT]
> Warnings go to stderr so they appear in the IDE's tool output, alerting the developer that Nano-Guard ran but could not validate. They do NOT block the agent.
