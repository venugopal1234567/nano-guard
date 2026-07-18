# Nano-Guard: Interceptor Implementation Specification

This specification contains reference implementations of the interceptor CLI in both Go and TypeScript (Node.js). The interceptor reads the tool invocation context from `stdin`, grabs the modified code context (via `git diff`), sends the payload to a local Ollama instance, and exits with code `0` or `2`.

---

## 1. Input Data Structure (stdin)

Claude Code pipes a JSON structure into `stdin` at `PostToolUse` event:
```json
{
  "hook_event_name": "PostToolUse",
  "tool_name": "replace_file_content",
  "tool_input": {
    "TargetFile": "/absolute/path/to/file.go",
    "ReplacementContent": "..."
  },
  "tool_response": {
    "stdout": "...",
    "stderr": ""
  },
  "cwd": "/home/user/project"
}
```

---

## 2. Go Reference Implementation

Go is highly recommended because it compiles to a single, zero-dependency binary that launches instantly (sub-millisecond start time).

Create a file named `main.go`:

```go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// HookInput matches the JSON schema piped into stdin by the IDE
type HookInput struct {
	HookEventName string                 `json:"hook_event_name"`
	ToolName      string                 `json:"tool_name"`
	ToolInput     map[string]interface{} `json:"tool_input"`
	ToolResponse  map[string]interface{} `json:"tool_response"`
	Cwd           string                 `json:"cwd"`
}

// OllamaRequest is the payload sent to /api/generate
type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	System string `json:"system"`
	Stream bool   `json:"stream"`
	Format string `json:"format"`
}

// OllamaResponse contains the response from Ollama API
type OllamaResponse struct {
	Response string `json:"response"`
}

// EvaluationResult represents the expected JSON response from the LLM
type EvaluationResult struct {
	Approved bool     `json:"approved"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
	Summary  string   `json:"summary"`
}

func main() {
	// 1. Read configuration or set defaults
	model := os.Getenv("NANO_GUARD_MODEL")
	if model == "" {
		model = "qwen2.5-coder:3b" // default model
	}
	ollamaURL := os.Getenv("OLLAMA_HOST")
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}

	// 2. Read stdin JSON
	var input HookInput
	stdinBytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Nano-Guard Error: failed to read stdin: %v\n", err)
		os.Exit(0) // Fail open to avoid blocking developer
	}

	if len(stdinBytes) == 0 {
		// No stdin context (possibly manual run). Default to success or test mode
		fmt.Println("Nano-Guard: No stdin provided. Skipping validation.")
		os.Exit(0)
	}

	if err := json.Unmarshal(stdinBytes, &input); err != nil {
		fmt.Fprintf(os.Stderr, "Nano-Guard Error: invalid JSON input: %v\n", err)
		os.Exit(0) // Fail open
	}

	// 3. Extract Git Diff
	diffCmd := exec.Command("git", "diff", "HEAD")
	if input.Cwd != "" {
		diffCmd.Dir = input.Cwd
	}
	diffOutput, err := diffCmd.Output()
	if err != nil {
		// Fallback: If git diff fails or not a repo, get diff of target file if known
		targetFile, ok := input.ToolInput["TargetFile"].(string)
		if !ok {
			targetFile, ok = input.ToolInput["AbsolutePath"].(string)
		}
		if ok && targetFile != "" {
			// Read the file directly as fallback
			fileContent, err := os.ReadFile(targetFile)
			if err == nil {
				diffOutput = []byte(fmt.Sprintf("--- File Content Fallback (%s) ---\n%s", targetFile, string(fileContent)))
			}
		}
	}

	if len(bytes.TrimSpace(diffOutput)) == 0 {
		// No changes detected to analyze
		os.Exit(0)
	}

	// 4. Construct Prompt
	systemPrompt := getSystemPrompt()
	userPrompt := fmt.Sprintf("Analyze this git diff and verify code quality metrics:\n\n%s", string(diffOutput))

	// 5. Query Ollama
	ollamaPayload := OllamaRequest{
		Model:  model,
		Prompt: userPrompt,
		System: systemPrompt,
		Stream: false,
		Format: "json", // Force JSON output
	}

	payloadBytes, err := json.Marshal(ollamaPayload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Nano-Guard Error: failed to marshal request: %v\n", err)
		os.Exit(0)
	}

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Post(ollamaURL+"/api/generate", "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Nano-Guard Error: failed to connect to Ollama: %v\n", err)
		os.Exit(0)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Nano-Guard Error: failed to read response body: %v\n", err)
		os.Exit(0)
	}

	var ollamaResponse OllamaResponse
	if err := json.Unmarshal(bodyBytes, &ollamaResponse); err != nil {
		fmt.Fprintf(os.Stderr, "Nano-Guard Error: failed to parse Ollama wrapper response: %v\n", err)
		os.Exit(0)
	}

	// 6. Parse LLM JSON Evaluation
	var result EvaluationResult
	if err := json.Unmarshal([]byte(ollamaResponse.Response), &result); err != nil {
		fmt.Fprintf(os.Stderr, "Nano-Guard Error: failed to parse LLM evaluation JSON: %v\nResponse was:\n%s\n", err, ollamaResponse.Response)
		os.Exit(0)
	}

	// 7. Handle Validation Outcome
	if result.Approved {
		// Exit code 0 tells the agent it is approved to proceed
		os.Exit(0)
	} else {
		// Exit code 2 blocks the agent and pipes feedback to stderr
		formattedErrors := strings.Join(result.Errors, "\n- ")
		fmt.Fprintf(os.Stderr, "\n🚨 Nano-Guard Code Verification Failed! 🚨\n")
		fmt.Fprintf(os.Stderr, "Summary: %s\n", result.Summary)
		fmt.Fprintf(os.Stderr, "Errors:\n- %s\n\n", formattedErrors)
		if len(result.Warnings) > 0 {
			fmt.Fprintf(os.Stderr, "Warnings:\n- %s\n\n", strings.Join(result.Warnings, "\n- "))
		}
		os.Exit(2)
	}
}

func getSystemPrompt() string {
	return `You are Nano-Guard, an automated code quality checker.
Analyze the provided code diff and output a strict JSON schema containing:
{
  "approved": boolean,
  "errors": ["list of structural issues, syntax mistakes, or unhandled errors"],
  "warnings": ["minor issues, console.logs, todo remarks"],
  "summary": "one line summary of changes"
}
Set "approved": false only if there are critical errors, type safety violations, or obvious logic bugs.`
}
```

---

## 3. TypeScript (Node.js) Reference Implementation

Create a file named `index.ts`:

```typescript
import * as fs from 'fs';
import { execSync } from 'child_process';

interface HookInput {
  hook_event_name: string;
  tool_name: string;
  tool_input: Record<string, any>;
  tool_response: Record<string, any>;
  cwd: string;
}

interface EvaluationResult {
  approved: boolean;
  errors: string[];
  warnings: string[];
  summary: string;
}

async function run() {
  const model = process.env.NANO_GUARD_MODEL || 'qwen2.5-coder:3b';
  const ollamaURL = process.env.OLLAMA_HOST || 'http://localhost:11434';

  try {
    const inputData = fs.readFileSync(0, 'utf-8');
    if (!inputData.trim()) {
      process.exit(0);
    }
    const input: HookInput = JSON.parse(inputData);

    // Get git diff
    let diff = '';
    try {
      diff = execSync('git diff HEAD', { cwd: input.cwd, encoding: 'utf8' });
    } catch {
      // Fallback
      const targetFile = input.tool_input?.TargetFile || input.tool_input?.AbsolutePath;
      if (targetFile && fs.existsSync(targetFile)) {
        diff = `--- File Content Fallback (${targetFile}) ---\n` + fs.readFileSync(targetFile, 'utf8');
      }
    }

    if (!diff.trim()) {
      process.exit(0);
    }

    // Call Ollama
    const systemPrompt = `You are Nano-Guard, an automated code quality checker.
Analyze the provided code diff and output a strict JSON schema containing:
{
  "approved": boolean,
  "errors": string[],
  "warnings": string[],
  "summary": string
}
Set "approved": false only if there are critical errors, type safety violations, or obvious logic bugs.`;

    const response = await fetch(`${ollamaURL}/api/generate`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        model,
        prompt: `Analyze this git diff and verify code quality metrics:\n\n${diff}`,
        system: systemPrompt,
        stream: false,
        format: 'json'
      })
    });

    if (!response.ok) {
      console.error(`Nano-Guard: Ollama returned status ${response.status}`);
      process.exit(0);
    }

    const data: any = await response.json();
    const result: EvaluationResult = JSON.parse(data.response);

    if (result.approved) {
      process.exit(0);
    } else {
      console.error(`\n🚨 Nano-Guard Code Verification Failed! 🚨`);
      console.error(`Summary: ${result.summary}`);
      console.error(`Errors:\n- ${result.errors.join('\n- ')}\n`);
      if (result.warnings.length > 0) {
        console.error(`Warnings:\n- ${result.warnings.join('\n- ')}\n`);
      }
      process.exit(2);
    }
  } catch (err) {
    console.error(`Nano-Guard error: ${err}`);
    process.exit(0); // Fail open
  }
}

run();
```
