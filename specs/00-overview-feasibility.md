# Nano-Guard: Project Overview & Feasibility

## Core Concept

Nano-Guard is a **PostToolUse lifecycle hook** — a CLI binary that the IDE spawns after every file write/edit. It costs $0 to run, adds zero tokens on success, and returns a compact JSON error block (<150 tokens) on failure. The primary cloud agent reads that error and self-corrects.

## Feasibility Verdict: ✅ Fully Buildable

All underlying primitives are stable and available today:

| Dependency | Status | Notes |
|:---|:---|:---|
| Claude Code `PostToolUse` hook | ✅ Live | Documented, works in production |
| Antigravity lifecycle hooks | ✅ Live | Rule-based sidecar interception |
| Ollama `/api/generate` + `format: json` | ✅ Stable | Enforces strict JSON output from local LLM |
| `git diff HEAD` context extraction | ✅ Reliable | Cheap, surgical — only sends changed lines |
| Exit code `2` → stderr feedback loop | ✅ Confirmed | Claude/Antigravity read stderr and inject it back into agent context |
| Go single binary distribution | ✅ Ideal | Zero cold start, no runtime deps, ships as one file |
| `npx` init bootstrapper | ✅ Standard | Standard Node.js tooling pattern |

## Key Design Decisions

1. **Go for the core binary**: Sub-millisecond startup. No node_modules. Single compiled artifact.
2. **TypeScript for the `init` CLI**: Node.js / npx ecosystem is the standard for developer tooling bootstrappers.
3. **Fail-open by default**: Any internal error (Ollama down, network timeout, malformed JSON) exits with `0` to never block development.
4. **Configurable via `nano-guard.config.json`**: Model selection, timeout, rule toggles, and custom checks are all configurable without touching code.
5. **Strict JSON output enforced at API level**: `"format": "json"` in the Ollama request payload forces the model's output decoder to only emit valid JSON tokens.
