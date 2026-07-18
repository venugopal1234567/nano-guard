# Nano-Guard: Architecture & System Flow

## 1. High-Level Sequence

```mermaid
sequenceDiagram
    autonumber
    participant IDE as Agentic IDE (Claude Code / Antigravity)
    participant NG  as nano-guard (Go Binary)
    participant GIT as git CLI
    participant OL  as Ollama (Local LLM)

    IDE->>IDE: Agent executes Write/Edit tool
    IDE->>NG: PostToolUse fires — pipes JSON to stdin
    NG->>NG: Parse stdin → extract file path + cwd
    NG->>GIT: git diff HEAD (in cwd)
    GIT-->>NG: Unified diff output
    NG->>OL: POST /api/generate (system prompt + diff, format=json)
    OL-->>NG: Structured JSON evaluation
    alt approved = true
        NG-->>IDE: exit(0) — silent success
    else approved = false
        NG-->>IDE: exit(2) + JSON error block → stderr
        IDE->>IDE: Agent reads stderr, self-corrects code
    end
```

---

## 2. Internal Module Flow (Go)

```
main()
  │
  ├── config.Load()              ← reads nano-guard.config.json
  │
  ├── hook.ParseStdin()          ← reads + unmarshals IDE JSON from stdin
  │
  ├── git.ExtractDiff(cwd, file) ← runs git diff, falls back to file read
  │     └─ if diff empty → exit(0) early
  │
  ├── ollama.Evaluate(diff, cfg) ← POST /api/generate
  │     └─ on network error      → exit(0) [fail open]
  │
  └── evaluator.Decide(result)
        ├─ approved = true  → exit(0)
        └─ approved = false → write JSON to stderr → exit(2)
```

---

## 3. Token Budget Design

The core design goal is **minimizing cloud token consumption**:

| Scenario | Cloud Tokens Added | Cost |
|:---|:---|:---|
| Clean edit (approved) | **0** — hook exits silently | $0.00 |
| Rejected edit (1-2 errors) | **~80–150 tokens** (structured JSON error) | ~$0.0001 |
| Hook internal error (Ollama down) | **0** — fail-open, silent exit | $0.00 |
| Large diff (>500 lines) | Diff is **truncated to first 200 lines** before sending to Ollama | $0.00 (local LLM) |

---

## 4. Component Boundaries

```
┌─────────────────────────────────────────────────┐
│                   nano-guard binary              │
│                                                  │
│  ┌──────────┐  ┌──────────┐  ┌────────────────┐ │
│  │  hook/   │  │   git/   │  │    config/     │ │
│  │ (stdin)  │  │  (diff)  │  │  (load JSON)   │ │
│  └────┬─────┘  └────┬─────┘  └───────┬────────┘ │
│       └─────────────┴────────────────┘           │
│                      │                           │
│              ┌────────────────┐                  │
│              │   ollama/      │                  │
│              │ (HTTP client)  │                  │
│              └───────┬────────┘                  │
│                      │                           │
│              ┌────────────────┐                  │
│              │  evaluator/    │                  │
│              │ (exit logic)   │                  │
│              └────────────────┘                  │
└─────────────────────────────────────────────────┘
              ▲                    ▼
         IDE stdin            IDE stderr / exit code
```
