# Nano-Guard: `init` CLI Bootstrapper

The `npx nano-guard@latest init` command is the user's single entry point. It handles everything — dependency checks, model pulling, binary install, and IDE config patching.

---

## 1. What `init` Does (in order)

```
npx nano-guard@latest init
│
├── 1. Check prerequisites
│   ├── Is Ollama installed? → if not, print install instructions + exit
│   └── Is git installed?   → if not, print warning (non-fatal)
│
├── 2. Pull local LLM model
│   └── ollama pull qwen2.5-coder:3b  (if not already pulled)
│
├── 3. Install the nano-guard binary
│   ├── Check if pre-built binary exists for this OS/arch
│   │     (downloads from GitHub Releases if available)
│   └── Otherwise: go build ./cmd/nano-guard -o ~/.local/bin/nano-guard
│
├── 4. Write nano-guard.config.json to project root
│   └── Copies default config template (user can edit after)
│
├── 5. Patch .claude/settings.json
│   ├── Create .claude/ directory if missing
│   ├── Read existing settings.json (if any)
│   ├── Merge nano-guard PostToolUse hook block into hooks section
│   └── Write back to disk (preserving existing settings)
│
└── 6. Print success summary
    ├── ✅ Model ready: qwen2.5-coder:3b
    ├── ✅ Binary installed: ~/.local/bin/nano-guard
    ├── ✅ Config written: ./nano-guard.config.json
    └── ✅ Hook registered: .claude/settings.json
```

---

## 2. Hook Block Injected into `.claude/settings.json`

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "write_to_file|replace_file_content|multi_replace_file_content|Write|Edit",
        "hooks": [
          {
            "type": "command",
            "command": "$HOME/.local/bin/nano-guard",
            "timeout": 30
          }
        ]
      }
    ]
  }
}
```

The `init` script merges this block into any existing `PostToolUse` array rather than overwriting it, so existing hooks are preserved.

---

## 3. TypeScript Module Structure

```
init/
├── package.json          ← name: "nano-guard", bin: { "nano-guard": "dist/index.js" }
├── tsconfig.json
└── src/
    ├── index.ts          ← CLI entrypoint, runs the init steps
    ├── prereqs.ts        ← Check Ollama + git installed
    ├── model.ts          ← ollama pull <model>
    ├── binary.ts         ← Download or build Go binary
    ├── config.ts         ← Write nano-guard.config.json
    └── settings.ts       ← Patch .claude/settings.json
```

---

## 4. `package.json` Spec

```json
{
  "name": "nano-guard",
  "version": "0.1.0",
  "description": "Local LLM-powered PostToolUse code verification hook for agentic IDEs",
  "bin": {
    "nano-guard": "./dist/index.js"
  },
  "scripts": {
    "build": "tsc",
    "start": "node dist/index.js"
  },
  "dependencies": {
    "chalk": "^5.0.0",
    "execa": "^8.0.0",
    "glob": "^10.0.0"
  },
  "devDependencies": {
    "typescript": "^5.0.0",
    "@types/node": "^20.0.0"
  },
  "engines": {
    "node": ">=18.0.0"
  }
}
```

---

## 5. CLI Output (Success)

```
🛡️  Nano-Guard Init

  Checking prerequisites...
  ✅ Ollama found (v0.6.2)
  ✅ git found

  Pulling model: qwen2.5-coder:3b
  ✅ Model ready

  Installing nano-guard binary...
  ✅ Installed at ~/.local/bin/nano-guard

  Writing config...
  ✅ nano-guard.config.json created

  Patching .claude/settings.json...
  ✅ PostToolUse hook registered

──────────────────────────────────────
  Nano-Guard is active. Every file edit
  will now be verified locally by
  qwen2.5-coder:3b before your agent
  continues. Zero cloud tokens used.
──────────────────────────────────────
```
