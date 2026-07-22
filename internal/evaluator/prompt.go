// Package evaluator contains the prompt-building logic for Nano-Guard.
//
// Spec reference: specs/07-prompt-engineering.md
//
// Design goals (§1):
//   - System prompt < 200 tokens
//   - Imperative language (commands, not suggestions)
//   - Schema shown before rule descriptions
//   - Rules are config-driven: disabled rules are removed from the prompt
package evaluator

import (
	"fmt"
	"os"
	"strings"

	"github.com/venugopal1234567/nano-guard/internal/config"
)

// -----------------------------------------------------------------------
// Prompt building blocks
// -----------------------------------------------------------------------

// promptPreamble is the fixed header: role declaration + JSON schema.
// It is always included, regardless of which rules are enabled.
const promptPreamble = `You are a strict code reviewer. Output ONLY valid JSON. No explanation. No prose.

JSON schema:
{
  "approved": boolean,
  "errors": [string],
  "warnings": [string],
  "summary": string
}

Rules — if any rule below is violated, set approved=false AND put the violation string in the errors array:`

// promptSuffix is the fixed footer appended after the active rule lines.
const promptSuffix = `
Warnings (do NOT fail for these alone): style issues, minor naming, dead imports.

Analyze the diff below and return the JSON object.`

// Individual rule lines — each maps exactly to a config.Rules toggle.
const (
	ruleUnhandledError  = "1. UNHANDLED_ERROR: function returns error/promise but caller ignores it"
	ruleDebugLog        = "2. DEBUG_LOG: console.log / fmt.Println / print() left in code"
	ruleTypeUnsafe      = "3. TYPE_UNSAFE: use of `any`, unsafe casts, or missing types on public API"
	rulePlaceholderStub = "4. PLACEHOLDER: TODO, FIXME, panic(\"implement me\"), empty stub body"
)

// -----------------------------------------------------------------------
// BuildSystemPrompt
// -----------------------------------------------------------------------

// BuildSystemPrompt assembles the system prompt by starting from the
// preamble and conditionally appending each rule line based on the
// supplied config.Rules toggles.
//
// Disabled rules are omitted entirely — this keeps the prompt lean and
// prevents the model from checking things the user has turned off.
//
// If no rules are enabled the rules section is omitted and only the
// preamble + suffix are returned (the model will always approve).
func BuildSystemPrompt(rules config.Rules) string {
	var sb strings.Builder
	sb.WriteString(promptPreamble)

	// Collect active rule lines in spec-defined order.
	activeRules := []string{}
	if rules.UnhandledErrors {
		activeRules = append(activeRules, ruleUnhandledError)
	}
	if rules.DebugLogs {
		activeRules = append(activeRules, ruleDebugLog)
	}
	if rules.TypeSafety {
		activeRules = append(activeRules, ruleTypeUnsafe)
	}
	if rules.PlaceholderStubs {
		activeRules = append(activeRules, rulePlaceholderStub)
	}

	if len(activeRules) > 0 {
		sb.WriteString("\n")
		for _, rule := range activeRules {
			sb.WriteString(rule)
			sb.WriteString("\n")
		}
	} else {
		// No rules enabled — still syntactically valid; model will approve everything.
		sb.WriteString("\n(no rules enabled — approve all changes)\n")
	}

	sb.WriteString(promptSuffix)
	return sb.String()
}

// -----------------------------------------------------------------------
// BuildUserPrompt
// -----------------------------------------------------------------------

// BuildUserPrompt constructs the user-facing prompt that wraps the diff
// content, as specified in spec §3.
//
// The system prompt is sent in the Ollama `system` field; this function
// produces the value for the `prompt` field.
func BuildUserPrompt(diff string) string {
	return fmt.Sprintf("Git diff to review:\n\n%s", diff)
}

// -----------------------------------------------------------------------
// LoadSystemPromptFile
// -----------------------------------------------------------------------

// LoadSystemPromptFile reads the system prompt base file from disk.
// It returns the raw file contents so callers can use it as-is or pass
// it to BuildSystemPrompt for rule injection.
//
// Falls back to the hardcoded default (all rules on) if the file is not
// found, keeping the fail-open philosophy consistent.
func LoadSystemPromptFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// Silently fall back to the full default prompt.
		return BuildSystemPrompt(config.Rules{
			UnhandledErrors:  true,
			DebugLogs:        true,
			TypeSafety:       true,
			PlaceholderStubs: true,
		}), nil
	}
	if err != nil {
		return "", fmt.Errorf("prompt: read %s: %w", path, err)
	}
	return strings.TrimRight(string(data), "\n"), nil
}
