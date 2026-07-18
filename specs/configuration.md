# Nano-Guard: Configuration Specification

This document details the configuration blocks required to register Nano-Guard with agentic IDEs, ensuring that tool executions are intercepted and evaluated before proceeding.

---

## 1. Claude Code Configuration

Claude Code reads settings from a project-level file at `.claude/settings.json` or globally at `~/.claude/settings.json`.

To intercept file modifications, we target the `PostToolUse` event with a matcher that matches any write or edit tool.

### `.claude/settings.json`
```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "write_to_file|replace_file_content|multi_replace_file_content|Write|Edit|EditFile|Bash",
        "hooks": [
          {
            "type": "command",
            "command": "$HOME/.local/bin/nano-guard-interceptor",
            "timeout": 30
          }
        ]
      }
    ]
  }
}
```

### Explanations of Key Fields:
* **`PostToolUse`**: Hook event type that fires immediately after a tool execution completes successfully.
* **`matcher`**: A regular expression pipe-separated list of tool names to intercept. This catches standard file editing tools.
* **`command`**: The path to the compiled Nano-Guard binary or execution script.
* **`timeout`**: Timeout in seconds (default is 30s) to prevent a hung Ollama call from locking up the editor.

---

## 2. Google Antigravity Configuration

Google Antigravity leverages Workspace Customizations (`.agents/AGENTS.md` and custom scripts/MCP tools) and agent rules.

To intercept operations in Antigravity, we can configure rules or trigger hooks defined in the project settings or customize via an MCP server integration.

### MCP-based Hook Configuration
If integrated as an MCP-compliant lifecycle interceptor, we register it as a tool or lifecycle sidecar:

```json
{
  "mcpServers": {
    "nano-guard": {
      "command": "/usr/local/bin/nano-guard",
      "args": ["--mcp"],
      "env": {
        "OLLAMA_HOST": "http://localhost:11434"
      }
    }
  }
}
```
In this mode, Nano-Guard is registered as an MCP server. The primary agent is instructed to call the evaluation tool on modified files before wrapping up its task (or the environment runs it as a post-step validation sidecar).
