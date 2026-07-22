// Package evaluator contains the result-parsing and exit-code decision logic
// for Nano-Guard.
//
// Spec reference: specs/08-exit-codes.md
//
// Exit code contract:
//
//	0 — Approved or fail-open (agent continues, no tokens consumed)
//	2 — Rejected (IDE injects stderr into agent context; agent self-corrects)
//	1 — Reserved / never emitted
package evaluator

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// -----------------------------------------------------------------------
// Exit codes
// -----------------------------------------------------------------------

const (
	// ExitApproved signals success or fail-open — agent continues normally.
	ExitApproved = 0
	// ExitRejected signals that the LLM found a violation — IDE injects
	// stderr into agent context for self-correction.
	ExitRejected = 2
)

// -----------------------------------------------------------------------
// LLM response schema
// -----------------------------------------------------------------------

// LLMResult is the JSON object returned by the Ollama model inside the
// response field.  Defined in specs/07-prompt-engineering.md §4.
type LLMResult struct {
	Approved bool     `json:"approved"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
	Summary  string   `json:"summary"`
}

// -----------------------------------------------------------------------
// Decision
// -----------------------------------------------------------------------

// Decision is the evaluated outcome produced by Evaluate().
type Decision struct {
	// ExitCode is ExitApproved (0) or ExitRejected (2).
	ExitCode int
	// Result is the parsed LLM response. May be nil on fail-open paths.
	Result *LLMResult
	// FailOpenReason is a human-readable explanation when the exit code is
	// ExitApproved but NOT because the LLM approved — e.g. "Ollama unreachable".
	// Empty when the LLM genuinely approved.
	FailOpenReason string
}

// -----------------------------------------------------------------------
// Evaluate
// -----------------------------------------------------------------------

// Evaluate parses the raw JSON string returned by the Ollama client and
// returns a Decision.
//
// rawJSON is the string from ollama.Client.Evaluate().  When rawJSON is
// empty (any soft-failure in the Ollama layer) the function returns a
// fail-open Decision with ExitCode 0.
//
// w is the writer for diagnostic warnings (typically os.Stderr).
func Evaluate(rawJSON string, w io.Writer) Decision {
	if w == nil {
		w = os.Stderr
	}

	// Empty rawJSON → Ollama soft-failure already logged by the client.
	// We fail-open silently at this layer.
	if strings.TrimSpace(rawJSON) == "" {
		return Decision{ExitCode: ExitApproved, FailOpenReason: "empty LLM response"}
	}

	var result LLMResult
	if err := json.Unmarshal([]byte(rawJSON), &result); err != nil {
		fmt.Fprintf(w, "[nano-guard warn] LLM returned non-JSON output\n")
		return Decision{ExitCode: ExitApproved, FailOpenReason: "non-JSON LLM output"}
	}

	if result.Approved {
		return Decision{ExitCode: ExitApproved, Result: &result}
	}

	return Decision{ExitCode: ExitRejected, Result: &result}
}

// -----------------------------------------------------------------------
// Rejection output formatter
// -----------------------------------------------------------------------

// FormatRejection writes the structured stderr block defined in spec §2
// to w.  Call this before os.Exit(ExitRejected).
func FormatRejection(result *LLMResult, w io.Writer) {
	if w == nil {
		w = os.Stderr
	}

	fmt.Fprintf(w, "🚨 Nano-Guard: Code verification failed.\n\n")
	fmt.Fprintf(w, "Summary: %s\n\n", result.Summary)

	fmt.Fprintf(w, "Errors (must fix before proceeding):\n")
	if len(result.Errors) == 0 {
		fmt.Fprintf(w, "  (none)\n")
	} else {
		for i, e := range result.Errors {
			fmt.Fprintf(w, "  [%d] %s\n", i+1, e)
		}
	}

	fmt.Fprintf(w, "\nWarnings (optional):\n")
	if len(result.Warnings) == 0 {
		fmt.Fprintf(w, "  (none)\n")
	} else {
		for _, warn := range result.Warnings {
			fmt.Fprintf(w, "  %s\n", warn)
		}
	}

	fmt.Fprintf(w, "\nFix the issues above and re-run your edit.\n")
}

// -----------------------------------------------------------------------
// Fail-open warn helpers  (spec §3 warn messages)
// -----------------------------------------------------------------------

// WarnOllamaUnreachable writes the spec-defined warning for connection errors.
func WarnOllamaUnreachable(w io.Writer) {
	if w == nil {
		w = os.Stderr
	}
	fmt.Fprintf(w, "[nano-guard warn] Ollama unreachable, skipping validation\n")
}

// WarnOllamaHTTPError writes the spec-defined warning for HTTP error responses.
func WarnOllamaHTTPError(status int, w io.Writer) {
	if w == nil {
		w = os.Stderr
	}
	fmt.Fprintf(w, "[nano-guard warn] Ollama returned HTTP %d\n", status)
}

// WarnOllamaTimeout writes the spec-defined warning when a request times out.
func WarnOllamaTimeout(seconds int, w io.Writer) {
	if w == nil {
		w = os.Stderr
	}
	fmt.Fprintf(w, "[nano-guard warn] Ollama timed out after %ds\n", seconds)
}

// WarnNonJSONResponse writes the spec-defined warning for unparseable LLM output.
func WarnNonJSONResponse(w io.Writer) {
	if w == nil {
		w = os.Stderr
	}
	fmt.Fprintf(w, "[nano-guard warn] LLM returned non-JSON output\n")
}
