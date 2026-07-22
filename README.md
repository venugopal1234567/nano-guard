# Nano-Guard 🛡️

Nano-Guard is an autonomous, lightweight **Post-Tool-Use** code verification hook designed for agentic IDEs (**Google Antigravity 2.0** and **Claude Code**). Powered by small local LLMs running via Ollama, it intercepts edits, evaluates quality, and pipes feedback to correct code.

### 💰 Why Nano-Guard? (Token & Cost Optimization)
* **Zero-Cost Gatekeeping**: Code reviews run on your local CPU/GPU via Ollama, saving expensive cloud model API tokens.
* **Minimal Context Bloat**: 
  * On success, the hook exits silently, adding **zero tokens** to your active cloud session.
  * On failure, it returns a hyper-focused error report (<150 tokens) rather than raw logs or massive files, saving memory and keeping sessions fast.


---

## ⚡ Quick Start (One-Step Setup)

To install Nano-Guard and automatically configure it for **Google Antigravity 2.0** or **Claude Code**, run inside your project directory:

```bash
# Register init CLI locally (until published on npm)
cd /path/to/Nano-Guard/init && npm link

# Run init inside any project workspace
npx nano-guard init
```

*This command automatically pulls the recommended local model (`qwen2.5-coder:7b`), builds the Go binary to `~/.local/bin/nano-guard`, and writes both `.claude/settings.json` (for Claude Code) and `.agents/AGENTS.md` (for Antigravity 2.0).*

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

## 🧪 How to Test Nano-Guard

### 1. Unit Tests (Go & TypeScript)
Nano-Guard includes unit test suites for all internal components (stdin payload parsing, Ollama HTTP communication, code evaluation formatting, git diff extraction, and init CLI generators):

```bash
# Run all Go unit tests
go test ./...

# Generate Go unit test coverage report
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out

# Run TypeScript init package tests
cd init && npm test
```

### 2. Live Integration Tests (Requires local Ollama)
Run full end-to-end integration tests that compile the Go binary and verify evaluation results against a live local Ollama model (`qwen2.5-coder:7b` or `3b`):

```bash
# 1. Ensure Ollama is running and has model pulled
ollama pull qwen2.5-coder:7b

# 2. Run live integration test suite
go test ./e2e/... -tags integration -timeout 120s
```

---

## 🪐 How to Integrate with Google Antigravity 2.0

Nano-Guard fully supports **Google Antigravity 2.0** through Workspace Rules (`.agents/AGENTS.md`) and command-line execution.

### Step 1: Install Binary
Build the Go binary and place it in your local binary path:
```bash
mkdir -p ~/.local/bin
go build -o ~/.local/bin/nano-guard ./cmd/nano-guard
```

### Step 2: Configure Workspace Rule in Antigravity 2.0
In your target project directory, create or update `.agents/AGENTS.md`:

```markdown
## Nano-Guard Post-Tool-Use Verification
After executing any file modification tool (write_to_file, replace_file_content, multi_replace_file_content), you MUST verify the code change by running this terminal command:
```bash
echo '{"hook_event_name":"PostToolUse","tool_name":"replace_file_content","tool_input":{"TargetFile":"<edited_file_path>"},"cwd":"/path/to/your/project"}' | ~/.local/bin/nano-guard
```
If the command outputs errors (Exit Code 2), you MUST fix the reported issues immediately.
```

### Step 3: Test in Antigravity 2.0
1. Ensure Ollama service is active (`ollama serve`).
2. Ask Antigravity to make an edit (e.g. *"Add a debug logger to main.go without handling errors"*).
3. Antigravity will apply the edit, execute `nano-guard` via terminal, catch the error report from `stderr`, and automatically self-correct the code!

---

## 🚀 How to Use Nano-Guard on Any New Project

Setting up Nano-Guard on any new project takes less than 30 seconds.

### Option A: One-Step Automatic Setup (Recommended)

1. Register the local `nano-guard` package once on your machine:
   ```bash
   cd /path/to/Nano-Guard/init && npm run build && npm link
   ```

2. Inside **any new or existing project workspace**, run:
   ```bash
   npx nano-guard init
   ```

   **What `npx nano-guard init` does automatically**:
   * Checks for local **Ollama** installation.
   * Pulls the recommended local LLM (`qwen2.5-coder:7b`).
   * Compiles & installs `~/.local/bin/nano-guard`.
   * Generates a default `nano-guard.config.json` config.
   * Configures `.claude/settings.json` (for Claude Code CLI).
   * Configures `.agents/AGENTS.md` (for Google Antigravity 2.0).

---

### Option B: Manual Setup for New Projects

1. Build & place `nano-guard` in your `$PATH`:
   ```bash
   go build -o ~/.local/bin/nano-guard ./cmd/nano-guard
   ```

2. In your new project root, create `.agents/AGENTS.md` (for Antigravity 2.0) and `.claude/settings.json` (for Claude Code).

3. *(Optional)* Create a `nano-guard.config.json` to customize rules or ignored path globs:
   ```json
   {
     "model": "qwen2.5-coder:7b",
     "ollama_host": "http://localhost:11434",
     "timeout_seconds": 30,
     "ignore_paths": ["**/*.md", "**/dist/**", "**/vendor/**"]
   }
   ```

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


