# Nano-Guard: Context Extraction

This document specifies how Nano-Guard extracts the code diff/context to send to the local LLM. The primary strategy is `git diff`. Multiple fallbacks handle non-git environments.

---

## 1. Strategy Waterfall

```
┌──────────────────────────────────────────┐
│ 1. git diff HEAD (unstaged changes)      │ ← Primary: shows exactly what changed
├──────────────────────────────────────────┤
│ 2. git diff --cached (staged changes)    │ ← Fallback if HEAD diff is empty
├──────────────────────────────────────────┤
│ 3. Read file + extract written chunk     │ ← Fallback if not a git repo
│    (from tool_input["CodeContent"])       │
├──────────────────────────────────────────┤
│ 4. Read full target file from disk       │ ← Last resort
└──────────────────────────────────────────┘
         │
         ▼
   If still empty → exit(0) — nothing to evaluate
```

---

## 2. Git Diff Command Spec

```bash
# Primary command
git diff HEAD --unified=3 -- <target_file>

# Flags:
# --unified=3   : show 3 lines of context around each change (balanced token cost vs. context)
# -- <file>     : scope diff to only the file that was just written
```

Using `-- <target_file>` is critical. Without it, the full repo diff is sent to Ollama, wasting tokens and overwhelming small models.

---

## 3. Diff Truncation

Before sending to Ollama, apply the following truncation rules based on `max_diff_lines` config:

```
if len(diff_lines) > max_diff_lines:
    send = diff_lines[:max_diff_lines]
    append footer: "\n[... diff truncated at {max_diff_lines} lines ...]"
```

**Default**: `max_diff_lines = 200`

This keeps the local LLM's context focused and prevents token overflow on large files.

---

## 4. Non-Git Fallback

If `git` is not available or the project is not a git repository:

1. Extract written content directly from `tool_input["CodeContent"]` (available for `write_to_file` calls).
2. Format it as a pseudo-diff:
   ```
   --- /dev/null
   +++ <TargetFile>
   @@ -0,0 +1,N @@
   +<line 1 of written content>
   +<line 2 of written content>
   ...
   ```
3. Apply the same `max_diff_lines` truncation.

---

## 5. Go Module Interface

```go
// internal/git/diff.go

package git

// ExtractDiff returns the unified diff for the given file within the cwd.
// Falls back to reading the file directly if git is unavailable.
func ExtractDiff(cwd, filePath string, maxLines int) (string, error)

// truncateDiff limits diff to maxLines, appending a truncation notice.
func truncateDiff(diff string, maxLines int) string
```
