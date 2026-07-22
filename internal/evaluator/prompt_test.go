package evaluator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/venugopal1234567/nano-guard/internal/config"
)

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

// allRulesOn returns a config.Rules with every toggle set to true.
func allRulesOn() config.Rules {
	return config.Rules{
		UnhandledErrors:  true,
		DebugLogs:        true,
		TypeSafety:       true,
		PlaceholderStubs: true,
	}
}

// allRulesOff returns a config.Rules with every toggle set to false.
func allRulesOff() config.Rules {
	return config.Rules{}
}

// -----------------------------------------------------------------------
// BuildSystemPrompt — structure
// -----------------------------------------------------------------------

func TestBuildSystemPrompt_ContainsPreamble(t *testing.T) {
	prompt := BuildSystemPrompt(allRulesOn())

	checks := []string{
		"strict code reviewer",
		"Output ONLY valid JSON",
		"No explanation",
		"No prose",
	}
	for _, want := range checks {
		if !strings.Contains(prompt, want) {
			t.Errorf("preamble: missing %q", want)
		}
	}
}

func TestBuildSystemPrompt_ContainsJSONSchema(t *testing.T) {
	prompt := BuildSystemPrompt(allRulesOn())

	for _, field := range []string{`"approved"`, `"errors"`, `"warnings"`, `"summary"`} {
		if !strings.Contains(prompt, field) {
			t.Errorf("JSON schema: missing field %q", field)
		}
	}
	if !strings.Contains(prompt, "boolean") {
		t.Error("JSON schema: missing 'boolean' type")
	}
	if !strings.Contains(prompt, "[string]") {
		t.Error("JSON schema: missing '[string]' type")
	}
}

func TestBuildSystemPrompt_ContainsSuffix(t *testing.T) {
	prompt := BuildSystemPrompt(allRulesOn())

	if !strings.Contains(prompt, "Warnings (do NOT fail for these alone)") {
		t.Error("suffix: missing warnings note")
	}
	if !strings.Contains(prompt, "Analyze the diff below") {
		t.Error("suffix: missing 'Analyze the diff below'")
	}
}

// -----------------------------------------------------------------------
// BuildSystemPrompt — all rules on
// -----------------------------------------------------------------------

func TestBuildSystemPrompt_AllRulesOn_HasAllRuleLines(t *testing.T) {
	prompt := BuildSystemPrompt(allRulesOn())

	cases := []struct {
		name    string
		keyword string
	}{
		{"UNHANDLED_ERROR", "UNHANDLED_ERROR"},
		{"DEBUG_LOG", "DEBUG_LOG"},
		{"TYPE_UNSAFE", "TYPE_UNSAFE"},
		{"PLACEHOLDER", "PLACEHOLDER"},
	}
	for _, tc := range cases {
		if !strings.Contains(prompt, tc.keyword) {
			t.Errorf("all rules on: missing rule %q", tc.keyword)
		}
	}
}

func TestBuildSystemPrompt_AllRulesOn_RuleCount(t *testing.T) {
	prompt := BuildSystemPrompt(allRulesOn())
	// Each rule line starts with its number.
	for _, prefix := range []string{"1.", "2.", "3.", "4."} {
		if !strings.Contains(prompt, prefix) {
			t.Errorf("all rules on: missing rule numbered %q", prefix)
		}
	}
}

// -----------------------------------------------------------------------
// BuildSystemPrompt — selective rules
// -----------------------------------------------------------------------

func TestBuildSystemPrompt_OnlyUnhandledError(t *testing.T) {
	rules := config.Rules{UnhandledErrors: true}
	prompt := BuildSystemPrompt(rules)

	if !strings.Contains(prompt, "UNHANDLED_ERROR") {
		t.Error("expected UNHANDLED_ERROR in prompt")
	}
	for _, absent := range []string{"DEBUG_LOG", "TYPE_UNSAFE", "PLACEHOLDER"} {
		if strings.Contains(prompt, absent) {
			t.Errorf("unexpected rule %q in prompt with only UnhandledErrors on", absent)
		}
	}
}

func TestBuildSystemPrompt_OnlyDebugLogs(t *testing.T) {
	rules := config.Rules{DebugLogs: true}
	prompt := BuildSystemPrompt(rules)

	if !strings.Contains(prompt, "DEBUG_LOG") {
		t.Error("expected DEBUG_LOG in prompt")
	}
	for _, absent := range []string{"UNHANDLED_ERROR", "TYPE_UNSAFE", "PLACEHOLDER"} {
		if strings.Contains(prompt, absent) {
			t.Errorf("unexpected rule %q", absent)
		}
	}
}

func TestBuildSystemPrompt_OnlyTypeSafety(t *testing.T) {
	rules := config.Rules{TypeSafety: true}
	prompt := BuildSystemPrompt(rules)

	if !strings.Contains(prompt, "TYPE_UNSAFE") {
		t.Error("expected TYPE_UNSAFE in prompt")
	}
}

func TestBuildSystemPrompt_OnlyPlaceholder(t *testing.T) {
	rules := config.Rules{PlaceholderStubs: true}
	prompt := BuildSystemPrompt(rules)

	if !strings.Contains(prompt, "PLACEHOLDER") {
		t.Error("expected PLACEHOLDER in prompt")
	}
}

func TestBuildSystemPrompt_TwoRules(t *testing.T) {
	rules := config.Rules{UnhandledErrors: true, DebugLogs: true}
	prompt := BuildSystemPrompt(rules)

	if !strings.Contains(prompt, "UNHANDLED_ERROR") {
		t.Error("missing UNHANDLED_ERROR")
	}
	if !strings.Contains(prompt, "DEBUG_LOG") {
		t.Error("missing DEBUG_LOG")
	}
	if strings.Contains(prompt, "TYPE_UNSAFE") {
		t.Error("unexpected TYPE_UNSAFE")
	}
	if strings.Contains(prompt, "PLACEHOLDER") {
		t.Error("unexpected PLACEHOLDER")
	}
}

// -----------------------------------------------------------------------
// BuildSystemPrompt — no rules
// -----------------------------------------------------------------------

func TestBuildSystemPrompt_NoRules_StillHasPreamble(t *testing.T) {
	prompt := BuildSystemPrompt(allRulesOff())

	if !strings.Contains(prompt, "strict code reviewer") {
		t.Error("no-rules: preamble must still be present")
	}
	if !strings.Contains(prompt, "Output ONLY valid JSON") {
		t.Error("no-rules: schema instruction must still be present")
	}
}

func TestBuildSystemPrompt_NoRules_NoRuleKeywords(t *testing.T) {
	prompt := BuildSystemPrompt(allRulesOff())

	for _, kw := range []string{"UNHANDLED_ERROR", "DEBUG_LOG", "TYPE_UNSAFE", "PLACEHOLDER"} {
		if strings.Contains(prompt, kw) {
			t.Errorf("no-rules: unexpected keyword %q in prompt", kw)
		}
	}
}

func TestBuildSystemPrompt_NoRules_HasNoRulesNotice(t *testing.T) {
	prompt := BuildSystemPrompt(allRulesOff())
	if !strings.Contains(prompt, "no rules enabled") {
		t.Error("no-rules: expected 'no rules enabled' notice")
	}
}

// -----------------------------------------------------------------------
// BuildSystemPrompt — token budget
// -----------------------------------------------------------------------

func TestBuildSystemPrompt_AllRules_Under200Tokens_Approx(t *testing.T) {
	prompt := BuildSystemPrompt(allRulesOn())
	// Rough token estimate: ~4 chars per token for English text.
	// The spec says < 200 tokens.
	charCount := utf8.RuneCountInString(prompt)
	approxTokens := charCount / 4
	if approxTokens >= 200 {
		t.Errorf("prompt may exceed 200 tokens: ~%d tokens (%d chars)", approxTokens, charCount)
	}
}

// -----------------------------------------------------------------------
// BuildSystemPrompt — rule ordering
// -----------------------------------------------------------------------

func TestBuildSystemPrompt_RulesInSpecOrder(t *testing.T) {
	prompt := BuildSystemPrompt(allRulesOn())

	posUnhandled := strings.Index(prompt, "UNHANDLED_ERROR")
	posDebug := strings.Index(prompt, "DEBUG_LOG")
	posType := strings.Index(prompt, "TYPE_UNSAFE")
	posPlace := strings.Index(prompt, "PLACEHOLDER")

	if posUnhandled >= posDebug {
		t.Error("UNHANDLED_ERROR must appear before DEBUG_LOG")
	}
	if posDebug >= posType {
		t.Error("DEBUG_LOG must appear before TYPE_UNSAFE")
	}
	if posType >= posPlace {
		t.Error("TYPE_UNSAFE must appear before PLACEHOLDER")
	}
}

// -----------------------------------------------------------------------
// BuildUserPrompt
// -----------------------------------------------------------------------

func TestBuildUserPrompt_ContainsDiff(t *testing.T) {
	diff := "+func hello() {}\n-func old() {}"
	result := BuildUserPrompt(diff)

	if !strings.Contains(result, diff) {
		t.Errorf("user prompt missing diff content: %q", result)
	}
}

func TestBuildUserPrompt_HasGitDiffLabel(t *testing.T) {
	result := BuildUserPrompt("some diff")
	if !strings.Contains(result, "Git diff to review") {
		t.Errorf("user prompt missing label, got: %q", result)
	}
}

func TestBuildUserPrompt_EmptyDiff(t *testing.T) {
	result := BuildUserPrompt("")
	// Should not panic; label should still be there.
	if !strings.Contains(result, "Git diff to review") {
		t.Error("user prompt label missing for empty diff")
	}
}

func TestBuildUserPrompt_Format(t *testing.T) {
	diff := "+line1\n+line2"
	result := BuildUserPrompt(diff)
	want := "Git diff to review:\n\n" + diff
	if result != want {
		t.Errorf("user prompt format:\ngot  %q\nwant %q", result, want)
	}
}

// -----------------------------------------------------------------------
// LoadSystemPromptFile
// -----------------------------------------------------------------------

func TestLoadSystemPromptFile_ReadsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "system.txt")
	os.WriteFile(path, []byte("custom prompt content"), 0o644)

	content, err := LoadSystemPromptFile(path)
	if err != nil {
		t.Fatalf("LoadSystemPromptFile: %v", err)
	}
	if content != "custom prompt content" {
		t.Errorf("got %q", content)
	}
}

func TestLoadSystemPromptFile_TrimsTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "system.txt")
	os.WriteFile(path, []byte("prompt\n\n"), 0o644)

	content, err := LoadSystemPromptFile(path)
	if err != nil {
		t.Fatalf("LoadSystemPromptFile: %v", err)
	}
	if strings.HasSuffix(content, "\n") {
		t.Errorf("expected trailing newlines trimmed, got %q", content)
	}
}

func TestLoadSystemPromptFile_NotExist_FallsBackToDefault(t *testing.T) {
	content, err := LoadSystemPromptFile("/does/not/exist/system.txt")
	if err != nil {
		t.Fatalf("expected fallback, got error: %v", err)
	}
	if content == "" {
		t.Fatal("fallback content must not be empty")
	}
	// Fallback should have all 4 rule keywords.
	for _, kw := range []string{"UNHANDLED_ERROR", "DEBUG_LOG", "TYPE_UNSAFE", "PLACEHOLDER"} {
		if !strings.Contains(content, kw) {
			t.Errorf("fallback missing rule keyword %q", kw)
		}
	}
}

func TestLoadSystemPromptFile_CanonicalFile(t *testing.T) {
	// Read the actual prompts/system.txt shipped with the repo.
	path := filepath.Join("..", "..", "prompts", "system.txt")
	content, err := LoadSystemPromptFile(path)
	if err != nil {
		t.Fatalf("canonical system.txt: %v", err)
	}
	if !strings.Contains(content, "approved") {
		t.Error("canonical system.txt: missing 'approved' schema field")
	}
	if !strings.Contains(content, "UNHANDLED_ERROR") {
		t.Error("canonical system.txt: missing UNHANDLED_ERROR rule")
	}
}

// -----------------------------------------------------------------------
// Integration — BuildSystemPrompt matches canonical file (all rules on)
// -----------------------------------------------------------------------

func TestBuildSystemPrompt_MatchesCanonicalFile(t *testing.T) {
	path := filepath.Join("..", "..", "prompts", "system.txt")
	fileContent, err := LoadSystemPromptFile(path)
	if err != nil {
		t.Fatalf("read canonical: %v", err)
	}
	built := BuildSystemPrompt(allRulesOn())

	// They don't need to be byte-identical (the file may have different
	// whitespace) but both must contain the same key landmarks.
	landmarks := []string{
		"strict code reviewer",
		"Output ONLY valid JSON",
		`"approved"`,
		`"errors"`,
		"UNHANDLED_ERROR",
		"DEBUG_LOG",
		"TYPE_UNSAFE",
		"PLACEHOLDER",
		"Warnings (do NOT fail for these alone)",
		"Analyze the diff below",
	}
	for _, lm := range landmarks {
		if !strings.Contains(fileContent, lm) {
			t.Errorf("canonical file missing: %q", lm)
		}
		if !strings.Contains(built, lm) {
			t.Errorf("BuildSystemPrompt missing: %q", lm)
		}
	}
}

// -----------------------------------------------------------------------
// Idempotency
// -----------------------------------------------------------------------

func TestBuildSystemPrompt_Idempotent(t *testing.T) {
	rules := allRulesOn()
	p1 := BuildSystemPrompt(rules)
	p2 := BuildSystemPrompt(rules)
	if p1 != p2 {
		t.Error("BuildSystemPrompt must be deterministic")
	}
}
