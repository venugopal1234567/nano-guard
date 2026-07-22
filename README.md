# Nano-Guard 🛡️

Nano-Guard is an autonomous, lightweight **Post-Tool-Use** code verification hook designed for agentic IDEs (like Claude Code and Google Antigravity). Powered by small local LLMs running via Ollama, it intercepts edits, evaluates quality, and pipes feedback to correct code.

### 💰 Why Nano-Guard? (Token & Cost Optimization)
* **Zero-Cost Gatekeeping**: Code reviews run on your local CPU/GPU via Ollama, saving expensive cloud model API tokens.
* **Minimal Context Bloat**: 
  * On success, the hook exits silently, adding **zero tokens** to your active cloud session.
  * On failure, it returns a hyper-focused error report (<150 tokens) rather than raw logs or massive files, saving memory and keeping sessions fast.


---

## ⚡ Quick Start (One-Step Setup)

To install Nano-Guard and automatically configure it for your local workspace, run:

```bash
# Install and run auto-setup
npx nano-guard@latest init
```

*This command automatically pulls the recommended local model (`qwen2.5-coder:7b`), builds the script, and writes the `.claude/settings.json` hook block for your current project.*

---

## 🔨 Manual Build & Verification

If you prefer building from source manually:

```bash
# 1. Navigate to project root
cd /home/venu/Documents/projects/ai/Nano-Guard

# 2. Build the binary directly to ~/.local/bin/nano-guard
mkdir -p ~/.local/bin
go build -o ~/.local/bin/nano-guard ./cmd/nano-guard

# 3. Verify installation and version
~/.local/bin/nano-guard --version
# Output: nano-guard v0.1.0
```

## 💡 Usage Examples

### Example 1: Enforcing Error Handling (Self-Correction Loop)

1. The primary IDE agent writes code with an unhandled async call:
   ```javascript
   // Written by agent
   function saveUser(data) {
     db.users.insert(data); // Missing await!
   }
   ```
2. Nano-Guard intercepts the write and exits with **Code 2**, returning this to the agent:
   ```
   🚨 Nano-Guard Code Verification Failed!
   Summary: Added saveUser function.
   Errors:
   - db.users.insert is asynchronous but called without 'await'. This returns a Promise immediately and may run out-of-order.
   ```
3. The primary agent reads the `stderr` error, understands the issue, and automatically rewrites the code correctly:
   ```javascript
   // Corrected by agent
   async function saveUser(data) {
     await db.users.insert(data);
   }
   ```

---

### Example 2: Chaining with Other Tools (e.g., Graphify)

You can chain multiple hooks in your `.claude/settings.json` to trigger multiple actions (such as updating codebase graphs via `graphify`) after edits:

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "write_to_file|replace_file_content|multi_replace_file_content|Write|Edit",
        "hooks": [
          {
            "type": "command",
            "command": "graphify update --file \"$CLAUDE_TOOL_INPUT_FILE_PATH\""
          },
          {
            "type": "command",
            "command": "nano-guard-interceptor"
          }
        ]
      }
    ]
  }
}
```

---

## 🛠️ How It Works

```
[Agentic IDE] ──(Executes Edit Tool)──> [PostToolUse Hook]
                                                │
                                       (Extracts Git Diff)
                                                │
                                                ▼
[Primary Agent] <───(Exit 2: stderr)─── [Ollama Local LLM]
      │
(Corrects Code)
```

Once installed, it runs transparently in the background:
* **Approved (Exit 0)**: Code changes are clean; the primary agent proceeds.
* **Rejected (Exit 2)**: Issues found; validation errors are piped to `stderr`. The primary agent reads them and automatically self-corrects the code.

---

## 📂 Specifications & Customization

Full build-ready specifications are in the [`specs/`](file:///home/venu/Documents/projects/ai/Nano-Guard/specs/) folder:

| # | Document | What it covers |
|:--|:--|:--|
| 00 | [Overview & Feasibility](file:///home/venu/Documents/projects/ai/Nano-Guard/specs/00-overview-feasibility.md) | Feasibility analysis, design decisions |
| 01 | [Project Structure](file:///home/venu/Documents/projects/ai/Nano-Guard/specs/01-project-structure.md) | Directory layout, file ownership |
| 02 | [Architecture](file:///home/venu/Documents/projects/ai/Nano-Guard/specs/02-architecture.md) | Sequence flow, module boundaries, token budget |
| 03 | [Config Schema](file:///home/venu/Documents/projects/ai/Nano-Guard/specs/03-config-schema.md) | `nano-guard.config.json` fields & defaults |
| 04 | [stdin Contract](file:///home/venu/Documents/projects/ai/Nano-Guard/specs/04-stdin-contract.md) | IDE PostToolUse JSON payload schema |
| 05 | [Context Extraction](file:///home/venu/Documents/projects/ai/Nano-Guard/specs/05-context-extraction.md) | Git diff strategy & fallback logic |
| 06 | [Ollama API](file:///home/venu/Documents/projects/ai/Nano-Guard/specs/06-ollama-api.md) | HTTP request/response contract, error handling |
| 07 | [Prompt Engineering](file:///home/venu/Documents/projects/ai/Nano-Guard/specs/07-prompt-engineering.md) | System prompt, few-shot examples, JSON schema |
| 08 | [Exit Codes](file:///home/venu/Documents/projects/ai/Nano-Guard/specs/08-exit-codes.md) | Exit code contract, stderr format, fail-open rules |
| 09 | [Init CLI](file:///home/venu/Documents/projects/ai/Nano-Guard/specs/09-init-cli.md) | `npx nano-guard init` bootstrapper spec |
| 10 | [Testing Plan](file:///home/venu/Documents/projects/ai/Nano-Guard/specs/10-testing-plan.md) | Unit, integration, E2E, and CI strategy |
| 11 | [Unit Test Code](file:///home/venu/Documents/projects/ai/Nano-Guard/specs/11-unit-tests.md) | Full runnable test source for every module |

