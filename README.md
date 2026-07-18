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

*This command automatically pulls the recommended local model (`qwen2.5-coder:3b`), builds the script, and writes the `.claude/settings.json` hook block for your current project.*

---

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

Detailed configurations, source code architectures, and prompt guides:
* 📊 [Architecture](file:///home/venu/Documents/projects/ai/Nano-Guard/specs/architecture.md): System design and exit codes.
* ⚙️ [Configuration](file:///home/venu/Documents/projects/ai/Nano-Guard/specs/configuration.md): IDE-specific hook integration.
* 💻 [Interceptor Code](file:///home/venu/Documents/projects/ai/Nano-Guard/specs/interceptor.md): Source implementations.
* 🧠 [Prompt Design](file:///home/venu/Documents/projects/ai/Nano-Guard/specs/prompting.md): Validation rules and LLM templates.
