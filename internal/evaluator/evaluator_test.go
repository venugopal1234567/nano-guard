package evaluator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

// buf returns a fresh buffer to capture output.
func buf() *bytes.Buffer { return &bytes.Buffer{} }

// toJSON marshals v to a JSON string (panics on error — test helper only).
func toJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// -----------------------------------------------------------------------
// Exit code constants
// -----------------------------------------------------------------------

func TestExitCodes_Values(t *testing.T) {
	if ExitApproved != 0 {
		t.Errorf("ExitApproved: want 0, got %d", ExitApproved)
	}
	if ExitRejected != 2 {
		t.Errorf("ExitRejected: want 2, got %d", ExitRejected)
	}
}

// -----------------------------------------------------------------------
// Evaluate — approved
// -----------------------------------------------------------------------

func TestEvaluate_Approved(t *testing.T) {
	raw := toJSON(LLMResult{
		Approved: true,
		Errors:   []string{},
		Warnings: []string{},
		Summary:  "Clean refactor.",
	})
	d := Evaluate(raw, buf())

	if d.ExitCode != ExitApproved {
		t.Errorf("ExitCode: want %d, got %d", ExitApproved, d.ExitCode)
	}
	if d.Result == nil {
		t.Fatal("Result must not be nil on approval")
	}
	if !d.Result.Approved {
		t.Error("Result.Approved: want true")
	}
	if d.FailOpenReason != "" {
		t.Errorf("FailOpenReason: want empty, got %q", d.FailOpenReason)
	}
}

func TestEvaluate_Approved_WithWarnings(t *testing.T) {
	// Warnings alone must not trigger rejection.
	raw := toJSON(LLMResult{
		Approved: true,
		Errors:   []string{},
		Warnings: []string{"minor naming issue"},
		Summary:  "Added helper.",
	})
	d := Evaluate(raw, buf())

	if d.ExitCode != ExitApproved {
		t.Errorf("warnings alone: want ExitApproved, got %d", d.ExitCode)
	}
}

// -----------------------------------------------------------------------
// Evaluate — rejected
// -----------------------------------------------------------------------

func TestEvaluate_Rejected_SingleError(t *testing.T) {
	raw := toJSON(LLMResult{
		Approved: false,
		Errors:   []string{"DEBUG_LOG: console.log() left in saveUser()"},
		Warnings: []string{},
		Summary:  "Added saveUser with a debug log.",
	})
	d := Evaluate(raw, buf())

	if d.ExitCode != ExitRejected {
		t.Errorf("ExitCode: want %d, got %d", ExitRejected, d.ExitCode)
	}
	if d.Result == nil {
		t.Fatal("Result must not be nil on rejection")
	}
	if d.Result.Approved {
		t.Error("Result.Approved: want false")
	}
	if len(d.Result.Errors) != 1 {
		t.Errorf("Errors: want 1, got %d", len(d.Result.Errors))
	}
}

func TestEvaluate_Rejected_MultipleErrors(t *testing.T) {
	raw := toJSON(LLMResult{
		Approved: false,
		Errors: []string{
			"UNHANDLED_ERROR: db.insert() return ignored",
			"DEBUG_LOG: console.log() in handler",
		},
		Warnings: []string{},
		Summary:  "Added handler.",
	})
	d := Evaluate(raw, buf())

	if d.ExitCode != ExitRejected {
		t.Errorf("ExitCode: want %d, got %d", ExitRejected, d.ExitCode)
	}
	if len(d.Result.Errors) != 2 {
		t.Errorf("Errors: want 2, got %d", len(d.Result.Errors))
	}
}

func TestEvaluate_Rejected_WithWarnings(t *testing.T) {
	raw := toJSON(LLMResult{
		Approved: false,
		Errors:   []string{"PLACEHOLDER: TODO in handler body"},
		Warnings: []string{"unused import"},
		Summary:  "Added stub.",
	})
	d := Evaluate(raw, buf())

	if d.ExitCode != ExitRejected {
		t.Errorf("ExitCode: want %d, got %d", ExitRejected, d.ExitCode)
	}
	if len(d.Result.Warnings) != 1 {
		t.Errorf("Warnings: want 1, got %d", len(d.Result.Warnings))
	}
}

// -----------------------------------------------------------------------
// Evaluate — fail-open paths (all must return ExitApproved)
// -----------------------------------------------------------------------

func TestEvaluate_EmptyString_FailOpen(t *testing.T) {
	d := Evaluate("", buf())
	if d.ExitCode != ExitApproved {
		t.Errorf("empty: want ExitApproved, got %d", d.ExitCode)
	}
	if d.FailOpenReason == "" {
		t.Error("empty: expected non-empty FailOpenReason")
	}
}

func TestEvaluate_WhitespaceOnly_FailOpen(t *testing.T) {
	d := Evaluate("   \n\t  ", buf())
	if d.ExitCode != ExitApproved {
		t.Errorf("whitespace: want ExitApproved, got %d", d.ExitCode)
	}
}

func TestEvaluate_MalformedJSON_FailOpen(t *testing.T) {
	w := buf()
	d := Evaluate("{not valid json", w)

	if d.ExitCode != ExitApproved {
		t.Errorf("malformed: want ExitApproved, got %d", d.ExitCode)
	}
	if d.FailOpenReason == "" {
		t.Error("malformed: expected non-empty FailOpenReason")
	}
	if !strings.Contains(w.String(), "[nano-guard warn]") {
		t.Errorf("malformed: expected warn in stderr, got %q", w.String())
	}
	if !strings.Contains(w.String(), "non-JSON") {
		t.Errorf("malformed: expected 'non-JSON' in stderr, got %q", w.String())
	}
}

func TestEvaluate_NilWriter_UsesStderr(t *testing.T) {
	// Must not panic when w is nil.
	d := Evaluate("{not json", nil)
	if d.ExitCode != ExitApproved {
		t.Errorf("nil writer: want ExitApproved, got %d", d.ExitCode)
	}
}

func TestEvaluate_WrongJSONShape_FailOpen(t *testing.T) {
	// Valid JSON but not the expected schema shape — approved defaults to false!
	// This should still be parseable (zero-value bool = false), so it will
	// be treated as rejected if approved is false. Let's verify the actual
	// Go zero-value behaviour.
	raw := `{"some_other_field": "value"}`
	d := Evaluate(raw, buf())
	// approved is false by default → rejected
	if d.ExitCode != ExitRejected {
		t.Errorf("wrong shape: want ExitRejected (approved=false default), got %d", d.ExitCode)
	}
}

// -----------------------------------------------------------------------
// Evaluate — result fields preserved
// -----------------------------------------------------------------------

func TestEvaluate_ResultFieldsPreserved(t *testing.T) {
	raw := toJSON(LLMResult{
		Approved: false,
		Errors:   []string{"TYPE_UNSAFE: as any in parseData()"},
		Warnings: []string{"dead import"},
		Summary:  "Refactored parser.",
	})
	d := Evaluate(raw, buf())

	if d.Result.Summary != "Refactored parser." {
		t.Errorf("Summary: got %q", d.Result.Summary)
	}
	if d.Result.Errors[0] != "TYPE_UNSAFE: as any in parseData()" {
		t.Errorf("Errors[0]: got %q", d.Result.Errors[0])
	}
	if d.Result.Warnings[0] != "dead import" {
		t.Errorf("Warnings[0]: got %q", d.Result.Warnings[0])
	}
}

// -----------------------------------------------------------------------
// FormatRejection — output structure
// -----------------------------------------------------------------------

func TestFormatRejection_Header(t *testing.T) {
	w := buf()
	FormatRejection(&LLMResult{
		Approved: false,
		Errors:   []string{"DEBUG_LOG: console.log in main()"},
		Summary:  "test summary",
	}, w)

	out := w.String()
	if !strings.Contains(out, "🚨 Nano-Guard: Code verification failed.") {
		t.Errorf("header missing: %q", out)
	}
}

func TestFormatRejection_Summary(t *testing.T) {
	w := buf()
	FormatRejection(&LLMResult{
		Summary: "Added saveUser function.",
		Errors:  []string{"DEBUG_LOG: console.log left in code"},
	}, w)

	if !strings.Contains(w.String(), "Summary: Added saveUser function.") {
		t.Errorf("summary missing: %q", w.String())
	}
}

func TestFormatRejection_ErrorsNumbered(t *testing.T) {
	w := buf()
	FormatRejection(&LLMResult{
		Errors: []string{
			"UNHANDLED_ERROR: db.insert() ignored",
			"DEBUG_LOG: console.log() left",
		},
		Summary: "s",
	}, w)

	out := w.String()
	if !strings.Contains(out, "[1] UNHANDLED_ERROR: db.insert() ignored") {
		t.Errorf("error [1] missing: %q", out)
	}
	if !strings.Contains(out, "[2] DEBUG_LOG: console.log() left") {
		t.Errorf("error [2] missing: %q", out)
	}
}

func TestFormatRejection_NoErrors_ShowsNone(t *testing.T) {
	w := buf()
	FormatRejection(&LLMResult{Summary: "s"}, w)

	if !strings.Contains(w.String(), "(none)") {
		t.Errorf("empty errors: expected '(none)', got %q", w.String())
	}
}

func TestFormatRejection_Warnings(t *testing.T) {
	w := buf()
	FormatRejection(&LLMResult{
		Errors:   []string{"PLACEHOLDER: TODO found"},
		Warnings: []string{"unused import", "minor naming"},
		Summary:  "s",
	}, w)

	out := w.String()
	if !strings.Contains(out, "unused import") {
		t.Errorf("warning 1 missing: %q", out)
	}
	if !strings.Contains(out, "minor naming") {
		t.Errorf("warning 2 missing: %q", out)
	}
}

func TestFormatRejection_NoWarnings_ShowsNone(t *testing.T) {
	w := buf()
	FormatRejection(&LLMResult{
		Errors:   []string{"TYPE_UNSAFE: as any"},
		Warnings: []string{},
		Summary:  "s",
	}, w)

	// The warnings section should still exist with "(none)"
	out := w.String()
	if !strings.Contains(out, "Warnings (optional):") {
		t.Errorf("warnings section missing: %q", out)
	}
	// "(none)" appears in warnings section (not errors)
	warningSection := out[strings.Index(out, "Warnings (optional):"):]
	if !strings.Contains(warningSection, "(none)") {
		t.Errorf("empty warnings: expected '(none)', got %q", warningSection)
	}
}

func TestFormatRejection_Footer(t *testing.T) {
	w := buf()
	FormatRejection(&LLMResult{
		Errors:  []string{"err"},
		Summary: "s",
	}, w)

	if !strings.Contains(w.String(), "Fix the issues above and re-run your edit.") {
		t.Errorf("footer missing: %q", w.String())
	}
}

func TestFormatRejection_NilWriter_UsesStderr(t *testing.T) {
	// Must not panic.
	FormatRejection(&LLMResult{Summary: "s"}, nil)
}

func TestFormatRejection_MatchesSpecExample(t *testing.T) {
	// Reproduces the exact example from spec §2.
	w := buf()
	FormatRejection(&LLMResult{
		Approved: false,
		Errors: []string{
			"UNHANDLED_ERROR: db.users.insert() is async but called without await in saveUser()",
			"DEBUG_LOG: console.log() left in saveUser()",
		},
		Warnings: []string{},
		Summary:  "Added saveUser function that inserts user into the database.",
	}, w)

	out := w.String()
	if !strings.Contains(out, "🚨 Nano-Guard") {
		t.Error("spec example: missing emoji header")
	}
	if !strings.Contains(out, "[1] UNHANDLED_ERROR") {
		t.Error("spec example: missing [1] UNHANDLED_ERROR")
	}
	if !strings.Contains(out, "[2] DEBUG_LOG") {
		t.Error("spec example: missing [2] DEBUG_LOG")
	}
	if !strings.Contains(out, "Fix the issues above") {
		t.Error("spec example: missing footer")
	}
}

// -----------------------------------------------------------------------
// Warn helpers — spec §3
// -----------------------------------------------------------------------

func TestWarnOllamaUnreachable(t *testing.T) {
	w := buf()
	WarnOllamaUnreachable(w)
	out := w.String()
	if !strings.Contains(out, "[nano-guard warn]") {
		t.Errorf("unreachable: missing warn prefix: %q", out)
	}
	if !strings.Contains(out, "Ollama unreachable") {
		t.Errorf("unreachable: missing message: %q", out)
	}
}

func TestWarnOllamaHTTPError(t *testing.T) {
	cases := []int{400, 404, 500, 503}
	for _, status := range cases {
		w := buf()
		WarnOllamaHTTPError(status, w)
		out := w.String()
		if !strings.Contains(out, "[nano-guard warn]") {
			t.Errorf("HTTP %d: missing warn prefix", status)
		}
		if !strings.Contains(out, "Ollama returned HTTP") {
			t.Errorf("HTTP %d: missing message", status)
		}
		// Status code must appear in the output.
		statusStr := fmt.Sprintf("%d", status)
		if !strings.Contains(out, statusStr) {
			t.Errorf("HTTP %d: status code not in output: %q", status, out)
		}
	}
}

func TestWarnOllamaTimeout(t *testing.T) {
	w := buf()
	WarnOllamaTimeout(30, w)
	out := w.String()
	if !strings.Contains(out, "[nano-guard warn]") {
		t.Errorf("timeout: missing warn prefix: %q", out)
	}
	if !strings.Contains(out, "timed out after 30s") {
		t.Errorf("timeout: missing message: %q", out)
	}
}

func TestWarnNonJSONResponse(t *testing.T) {
	w := buf()
	WarnNonJSONResponse(w)
	out := w.String()
	if !strings.Contains(out, "[nano-guard warn]") {
		t.Errorf("non-JSON: missing warn prefix: %q", out)
	}
	if !strings.Contains(out, "non-JSON") {
		t.Errorf("non-JSON: missing message: %q", out)
	}
}

func TestWarnHelpers_NilWriter_NoPanic(t *testing.T) {
	WarnOllamaUnreachable(nil)
	WarnOllamaHTTPError(500, nil)
	WarnOllamaTimeout(30, nil)
	WarnNonJSONResponse(nil)
}

// -----------------------------------------------------------------------
// LLMResult JSON tags
// -----------------------------------------------------------------------

func TestLLMResult_JSONRoundTrip(t *testing.T) {
	original := LLMResult{
		Approved: false,
		Errors:   []string{"err1", "err2"},
		Warnings: []string{"warn1"},
		Summary:  "test",
	}
	data, _ := json.Marshal(original)

	var decoded LLMResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Approved != original.Approved {
		t.Errorf("Approved: got %v", decoded.Approved)
	}
	if len(decoded.Errors) != 2 {
		t.Errorf("Errors: got %v", decoded.Errors)
	}
	if decoded.Summary != "test" {
		t.Errorf("Summary: got %q", decoded.Summary)
	}
}

func TestLLMResult_JSONFieldNames(t *testing.T) {
	r := LLMResult{Approved: true, Summary: "x"}
	b, _ := json.Marshal(r)
	s := string(b)
	for _, field := range []string{`"approved"`, `"errors"`, `"warnings"`, `"summary"`} {
		if !strings.Contains(s, field) {
			t.Errorf("JSON missing field %q in %s", field, s)
		}
	}
}


