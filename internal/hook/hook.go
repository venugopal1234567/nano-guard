// Package hook parses the PostToolUse JSON payload that IDE/Claude Code
// pipes into Nano-Guard via stdin.
//
// Spec reference: specs/04-stdin-contract.md
package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// -----------------------------------------------------------------------
// Write-tool registry
// -----------------------------------------------------------------------

// writeTool enumerates all tool_name values that write or modify files and
// therefore require Nano-Guard evaluation.
type writeTool string

const (
	ToolWriteToFile           writeTool = "write_to_file"
	ToolReplaceFileContent    writeTool = "replace_file_content"
	ToolMultiReplaceFile      writeTool = "multi_replace_file_content"
	ToolWrite                 writeTool = "Write" // Claude Code alias
	ToolEdit                  writeTool = "Edit"  // Claude Code alias
)

// writeTools is the set of tool names that unconditionally trigger evaluation.
var writeTools = map[string]bool{
	string(ToolWriteToFile):        true,
	string(ToolReplaceFileContent):  true,
	string(ToolMultiReplaceFile):    true,
	string(ToolWrite):               true,
	string(ToolEdit):                true,
}

// IsWriteTool returns true if the given tool_name should trigger evaluation.
// The Bash tool is handled separately (see IsBashWrite).
func IsWriteTool(name string) bool {
	return writeTools[name]
}

// IsBashWrite returns true when the tool is "Bash" and the command contains
// output-redirection operators that could write to a file (> or tee).
func IsBashWrite(name string, toolInput map[string]interface{}) bool {
	if name != "Bash" {
		return false
	}
	cmd, _ := toolInput["command"].(string)
	for i := 0; i < len(cmd); i++ {
		if cmd[i] == '>' {
			return true
		}
		// tee anywhere in the command
		if i+3 <= len(cmd) && cmd[i:i+3] == "tee" {
			return true
		}
	}
	return false
}

// -----------------------------------------------------------------------
// Payload schema
// -----------------------------------------------------------------------

// HookInput is the exact JSON schema piped into stdin by the IDE hook runner.
// Defined in specs/04-stdin-contract.md §2.
type HookInput struct {
	HookEventName string                 `json:"hook_event_name"`
	SessionID     string                 `json:"session_id"`
	ToolName      string                 `json:"tool_name"`
	ToolInput     map[string]interface{} `json:"tool_input"`
	ToolResponse  map[string]interface{} `json:"tool_response"`
	Cwd           string                 `json:"cwd"`
}

// -----------------------------------------------------------------------
// Parsed result
// -----------------------------------------------------------------------

// ParsedHook is the cleaned-up, decision-ready representation derived from
// HookInput.  Callers should check ShouldEvaluate before doing any further
// work.
type ParsedHook struct {
	// Raw is the original decoded payload.
	Raw HookInput

	// ShouldEvaluate is true when the hook event is a write tool and a
	// file path could be resolved.  If false, Nano-Guard exits 0 immediately.
	ShouldEvaluate bool

	// FilePath is the absolute (or CWD-relative) path of the file that was
	// written, derived from ToolInput / ToolResponse per the spec priority.
	FilePath string

	// Cwd is the working directory to use when resolving relative paths.
	// Falls back to os.Getwd() when the payload field is absent.
	Cwd string
}

// -----------------------------------------------------------------------
// File-path extraction
// -----------------------------------------------------------------------

// extractFilePath applies the spec §2 priority chain to derive a file path:
//
//  1. tool_input["TargetFile"]
//  2. tool_input["AbsolutePath"]
//  3. tool_input["path"]
//  4. tool_response["filePath"]
//  5. "" (not found)
func extractFilePath(toolInput, toolResponse map[string]interface{}) string {
	keys := []string{"TargetFile", "AbsolutePath", "path"}
	for _, k := range keys {
		if v, ok := toolInput[k].(string); ok && v != "" {
			return v
		}
	}
	if v, ok := toolResponse["filePath"].(string); ok && v != "" {
		return v
	}
	return ""
}

// -----------------------------------------------------------------------
// Public parsing API
// -----------------------------------------------------------------------

// Parse decodes a HookInput from r (typically os.Stdin) and returns a
// ParsedHook.
//
// Errors are intentionally swallowed per the fail-open design: if the JSON
// cannot be decoded, Parse returns a ParsedHook with ShouldEvaluate=false
// and a non-nil error so callers can log it when desired.
func Parse(r io.Reader) (ParsedHook, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return ParsedHook{}, fmt.Errorf("hook: read stdin: %w", err)
	}

	if len(data) == 0 {
		return ParsedHook{}, fmt.Errorf("hook: empty stdin")
	}

	var input HookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return ParsedHook{}, fmt.Errorf("hook: malformed JSON: %w", err)
	}

	return buildParsed(input), nil
}

// ParseBytes is a convenience wrapper around Parse that reads from a byte
// slice instead of an io.Reader. Useful in tests.
func ParseBytes(data []byte) (ParsedHook, error) {
	var input HookInput
	if len(data) == 0 {
		return ParsedHook{}, fmt.Errorf("hook: empty input")
	}
	if err := json.Unmarshal(data, &input); err != nil {
		return ParsedHook{}, fmt.Errorf("hook: malformed JSON: %w", err)
	}
	return buildParsed(input), nil
}

// buildParsed applies all the decision logic described in the spec to
// produce a ParsedHook from a decoded HookInput.
func buildParsed(input HookInput) ParsedHook {
	p := ParsedHook{Raw: input}

	// --- Resolve CWD -------------------------------------------------
	if input.Cwd != "" {
		p.Cwd = input.Cwd
	} else {
		cwd, err := os.Getwd()
		if err == nil {
			p.Cwd = cwd
		}
	}

	// --- Decide whether to evaluate ----------------------------------
	isWrite := IsWriteTool(input.ToolName)
	isBashW := IsBashWrite(input.ToolName, input.ToolInput)

	if !isWrite && !isBashW {
		// Not a write tool → exit 0 (ShouldEvaluate stays false)
		return p
	}

	// --- Extract file path -------------------------------------------
	filePath := extractFilePath(input.ToolInput, input.ToolResponse)
	if filePath == "" {
		// Cannot evaluate without a path → exit 0
		return p
	}

	p.FilePath = filePath
	p.ShouldEvaluate = true
	return p
}
