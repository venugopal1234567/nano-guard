package hook

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

// fixtureBytes reads a file from testdata/payloads/ relative to the module root.
func fixtureBytes(t *testing.T, name string) []byte {
	t.Helper()
	// Walk up from the package directory to the module root.
	path := filepath.Join("..", "..", "testdata", "payloads", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	return data
}

// -----------------------------------------------------------------------
// IsWriteTool
// -----------------------------------------------------------------------

func TestIsWriteTool_KnownWriteTools(t *testing.T) {
	tools := []string{
		"write_to_file",
		"replace_file_content",
		"multi_replace_file_content",
		"Write",
		"Edit",
	}
	for _, name := range tools {
		if !IsWriteTool(name) {
			t.Errorf("IsWriteTool(%q): want true", name)
		}
	}
}

func TestIsWriteTool_NonWriteTools(t *testing.T) {
	tools := []string{"Read", "Bash", "read_file", "list_dir", ""}
	for _, name := range tools {
		if IsWriteTool(name) {
			t.Errorf("IsWriteTool(%q): want false", name)
		}
	}
}

func TestIsWriteTool_CaseSensitive(t *testing.T) {
	// Tool names are case-sensitive per the spec.
	if IsWriteTool("write_To_File") {
		t.Error("IsWriteTool should be case-sensitive")
	}
	if IsWriteTool("WRITE_TO_FILE") {
		t.Error("IsWriteTool should be case-sensitive")
	}
}

// -----------------------------------------------------------------------
// IsBashWrite
// -----------------------------------------------------------------------

func TestIsBashWrite_NotBash(t *testing.T) {
	if IsBashWrite("Write", map[string]interface{}{"command": "echo hello > out.txt"}) {
		t.Error("IsBashWrite: want false for non-Bash tool")
	}
}

func TestIsBashWrite_BashWithRedirection(t *testing.T) {
	cases := []string{
		"echo hello > out.txt",
		"cat file.go > /tmp/copy.go",
		"go build > build.log 2>&1",
	}
	for _, cmd := range cases {
		input := map[string]interface{}{"command": cmd}
		if !IsBashWrite("Bash", input) {
			t.Errorf("IsBashWrite(Bash, %q): want true", cmd)
		}
	}
}

func TestIsBashWrite_BashWithTee(t *testing.T) {
	cases := []string{
		"echo hello | tee out.txt",
		"cat file | tee -a log.txt",
	}
	for _, cmd := range cases {
		input := map[string]interface{}{"command": cmd}
		if !IsBashWrite("Bash", input) {
			t.Errorf("IsBashWrite(Bash, tee, %q): want true", cmd)
		}
	}
}

func TestIsBashWrite_BashReadOnly(t *testing.T) {
	cases := []string{
		"ls -la",
		"git status",
		"go test ./...",
		"cat file.go",
	}
	for _, cmd := range cases {
		input := map[string]interface{}{"command": cmd}
		if IsBashWrite("Bash", input) {
			t.Errorf("IsBashWrite(Bash, %q): want false for read-only command", cmd)
		}
	}
}

func TestIsBashWrite_BashMissingCommandKey(t *testing.T) {
	input := map[string]interface{}{}
	if IsBashWrite("Bash", input) {
		t.Error("IsBashWrite with missing command key: want false")
	}
}

func TestIsBashWrite_NilInput(t *testing.T) {
	if IsBashWrite("Bash", nil) {
		t.Error("IsBashWrite with nil input: want false")
	}
}

// -----------------------------------------------------------------------
// extractFilePath
// -----------------------------------------------------------------------

func TestExtractFilePath_TargetFile(t *testing.T) {
	ti := map[string]interface{}{"TargetFile": "/a/b/c.go"}
	tr := map[string]interface{}{}
	if got := extractFilePath(ti, tr); got != "/a/b/c.go" {
		t.Errorf("want /a/b/c.go, got %q", got)
	}
}

func TestExtractFilePath_AbsolutePath(t *testing.T) {
	ti := map[string]interface{}{"AbsolutePath": "/x/y/z.ts"}
	tr := map[string]interface{}{}
	if got := extractFilePath(ti, tr); got != "/x/y/z.ts" {
		t.Errorf("want /x/y/z.ts, got %q", got)
	}
}

func TestExtractFilePath_PathFallback(t *testing.T) {
	ti := map[string]interface{}{"path": "relative/file.py"}
	tr := map[string]interface{}{}
	if got := extractFilePath(ti, tr); got != "relative/file.py" {
		t.Errorf("want relative/file.py, got %q", got)
	}
}

func TestExtractFilePath_ResponseFilePath(t *testing.T) {
	ti := map[string]interface{}{}
	tr := map[string]interface{}{"filePath": "/resp/file.rs"}
	if got := extractFilePath(ti, tr); got != "/resp/file.rs" {
		t.Errorf("want /resp/file.rs, got %q", got)
	}
}

func TestExtractFilePath_Priority_TargetFileWins(t *testing.T) {
	// TargetFile must win over AbsolutePath and path
	ti := map[string]interface{}{
		"TargetFile":   "/wins.go",
		"AbsolutePath": "/loses.go",
		"path":         "/also-loses.go",
	}
	tr := map[string]interface{}{"filePath": "/response-loses.go"}
	if got := extractFilePath(ti, tr); got != "/wins.go" {
		t.Errorf("TargetFile priority: want /wins.go, got %q", got)
	}
}

func TestExtractFilePath_Priority_AbsolutePathBeforePath(t *testing.T) {
	ti := map[string]interface{}{
		"AbsolutePath": "/absolute.go",
		"path":         "/path.go",
	}
	tr := map[string]interface{}{}
	if got := extractFilePath(ti, tr); got != "/absolute.go" {
		t.Errorf("AbsolutePath priority: want /absolute.go, got %q", got)
	}
}

func TestExtractFilePath_NoneFound(t *testing.T) {
	ti := map[string]interface{}{"CodeContent": "package main"}
	tr := map[string]interface{}{"success": true}
	if got := extractFilePath(ti, tr); got != "" {
		t.Errorf("want empty string, got %q", got)
	}
}

func TestExtractFilePath_EmptyStringIgnored(t *testing.T) {
	// An empty TargetFile should not win — fall through to next key.
	ti := map[string]interface{}{
		"TargetFile":   "",
		"AbsolutePath": "/fallback.go",
	}
	tr := map[string]interface{}{}
	if got := extractFilePath(ti, tr); got != "/fallback.go" {
		t.Errorf("empty TargetFile should fall through: got %q", got)
	}
}

func TestExtractFilePath_NilMaps(t *testing.T) {
	// Must not panic with nil maps
	got := extractFilePath(nil, nil)
	if got != "" {
		t.Errorf("nil maps: want empty, got %q", got)
	}
}

// -----------------------------------------------------------------------
// ParseBytes — happy paths
// -----------------------------------------------------------------------

func TestParseBytes_ValidWrite(t *testing.T) {
	data := fixtureBytes(t, "valid_write.json")
	p, err := ParseBytes(data)
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	if !p.ShouldEvaluate {
		t.Error("ShouldEvaluate: want true")
	}
	if p.FilePath != "/home/user/project/src/server.go" {
		t.Errorf("FilePath: got %q", p.FilePath)
	}
	if p.Cwd != "/home/user/project" {
		t.Errorf("Cwd: got %q", p.Cwd)
	}
	if p.Raw.ToolName != "write_to_file" {
		t.Errorf("ToolName: got %q", p.Raw.ToolName)
	}
	if p.Raw.SessionID != "abc-123-def-456" {
		t.Errorf("SessionID: got %q", p.Raw.SessionID)
	}
}

func TestParseBytes_ValidEdit(t *testing.T) {
	data := fixtureBytes(t, "valid_edit.json")
	p, err := ParseBytes(data)
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	if !p.ShouldEvaluate {
		t.Error("ShouldEvaluate: want true for replace_file_content")
	}
	if p.FilePath != "/home/user/project/src/handler.go" {
		t.Errorf("FilePath: got %q", p.FilePath)
	}
}

// -----------------------------------------------------------------------
// ParseBytes — skip cases (ShouldEvaluate = false)
// -----------------------------------------------------------------------

func TestParseBytes_ReadTool_Skipped(t *testing.T) {
	data := fixtureBytes(t, "read_tool.json")
	p, err := ParseBytes(data)
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	if p.ShouldEvaluate {
		t.Error("Read tool: ShouldEvaluate must be false")
	}
}

func TestParseBytes_EmptyJSON_Skipped(t *testing.T) {
	data := fixtureBytes(t, "empty.json")
	p, err := ParseBytes(data)
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	if p.ShouldEvaluate {
		t.Error("Empty JSON: ShouldEvaluate must be false (no write tool)")
	}
}

// -----------------------------------------------------------------------
// ParseBytes — error / fail-open cases
// -----------------------------------------------------------------------

func TestParseBytes_Malformed_Error(t *testing.T) {
	data := fixtureBytes(t, "malformed.json")
	_, err := ParseBytes(data)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestParseBytes_EmptySlice_Error(t *testing.T) {
	_, err := ParseBytes([]byte{})
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

// -----------------------------------------------------------------------
// Parse (io.Reader) — smoke tests
// -----------------------------------------------------------------------

func TestParse_FromReader(t *testing.T) {
	raw := `{
		"hook_event_name": "PostToolUse",
		"tool_name": "Write",
		"tool_input": {"TargetFile": "/tmp/out.go"},
		"tool_response": {},
		"cwd": "/tmp"
	}`
	p, err := Parse(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !p.ShouldEvaluate {
		t.Error("ShouldEvaluate: want true for Write tool")
	}
	if p.FilePath != "/tmp/out.go" {
		t.Errorf("FilePath: got %q", p.FilePath)
	}
}

func TestParse_EmptyReader_Error(t *testing.T) {
	_, err := Parse(bytes.NewReader(nil))
	if err == nil {
		t.Fatal("empty reader: want error")
	}
}

func TestParse_MalformedReader_Error(t *testing.T) {
	_, err := Parse(strings.NewReader("not json"))
	if err == nil {
		t.Fatal("malformed reader: want error")
	}
}

// -----------------------------------------------------------------------
// CWD fallback
// -----------------------------------------------------------------------

func TestBuildParsed_CwdFallback(t *testing.T) {
	// When cwd field is absent, should fall back to os.Getwd().
	input := HookInput{
		ToolName:     "write_to_file",
		ToolInput:    map[string]interface{}{"TargetFile": "/some/file.go"},
		ToolResponse: map[string]interface{}{},
		Cwd:          "", // deliberately empty
	}
	p := buildParsed(input)
	if p.Cwd == "" {
		t.Error("Cwd fallback: want os.Getwd(), got empty string")
	}
}

func TestBuildParsed_CwdFromPayload(t *testing.T) {
	input := HookInput{
		ToolName:     "Edit",
		ToolInput:    map[string]interface{}{"AbsolutePath": "/proj/main.go"},
		ToolResponse: map[string]interface{}{},
		Cwd:          "/proj",
	}
	p := buildParsed(input)
	if p.Cwd != "/proj" {
		t.Errorf("Cwd: want /proj, got %q", p.Cwd)
	}
}

// -----------------------------------------------------------------------
// ShouldEvaluate edge cases
// -----------------------------------------------------------------------

func TestBuildParsed_WriteToolNoPath_NotEvaluated(t *testing.T) {
	// Write tool but no file path resolvable → skip
	input := HookInput{
		ToolName:     "write_to_file",
		ToolInput:    map[string]interface{}{"CodeContent": "package main"},
		ToolResponse: map[string]interface{}{"success": true},
		Cwd:          "/proj",
	}
	p := buildParsed(input)
	if p.ShouldEvaluate {
		t.Error("No file path → ShouldEvaluate must be false")
	}
}

func TestBuildParsed_MultiReplace_Evaluated(t *testing.T) {
	input := HookInput{
		ToolName: "multi_replace_file_content",
		ToolInput: map[string]interface{}{
			"TargetFile": "/src/app.ts",
		},
		ToolResponse: map[string]interface{}{},
		Cwd:          "/src",
	}
	p := buildParsed(input)
	if !p.ShouldEvaluate {
		t.Error("multi_replace_file_content: ShouldEvaluate must be true")
	}
	if p.FilePath != "/src/app.ts" {
		t.Errorf("FilePath: got %q", p.FilePath)
	}
}

func TestBuildParsed_BashWithRedirection_Evaluated(t *testing.T) {
	input := HookInput{
		ToolName: "Bash",
		ToolInput: map[string]interface{}{
			"command": "go build > build.log",
		},
		ToolResponse: map[string]interface{}{"filePath": "build.log"},
		Cwd:          "/proj",
	}
	p := buildParsed(input)
	if !p.ShouldEvaluate {
		t.Error("Bash with > redirect: ShouldEvaluate must be true")
	}
}

func TestBuildParsed_BashReadOnly_NotEvaluated(t *testing.T) {
	input := HookInput{
		ToolName:     "Bash",
		ToolInput:    map[string]interface{}{"command": "go test ./..."},
		ToolResponse: map[string]interface{}{},
		Cwd:          "/proj",
	}
	p := buildParsed(input)
	if p.ShouldEvaluate {
		t.Error("Bash read-only: ShouldEvaluate must be false")
	}
}

func TestBuildParsed_UnknownTool_NotEvaluated(t *testing.T) {
	input := HookInput{
		ToolName:     "list_dir",
		ToolInput:    map[string]interface{}{"path": "/some/dir"},
		ToolResponse: map[string]interface{}{},
		Cwd:          "/proj",
	}
	p := buildParsed(input)
	if p.ShouldEvaluate {
		t.Error("Unknown tool: ShouldEvaluate must be false")
	}
}

// -----------------------------------------------------------------------
// HookInput JSON round-trip
// -----------------------------------------------------------------------

func TestHookInput_JSONRoundTrip(t *testing.T) {
	raw := `{
		"hook_event_name": "PostToolUse",
		"session_id": "s1",
		"tool_name": "write_to_file",
		"tool_input": {"TargetFile": "/f.go", "CodeContent": "pkg main"},
		"tool_response": {"filePath": "/f.go", "success": true},
		"cwd": "/proj"
	}`
	p, err := Parse(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if p.Raw.HookEventName != "PostToolUse" {
		t.Errorf("HookEventName: %q", p.Raw.HookEventName)
	}
	if p.Raw.SessionID != "s1" {
		t.Errorf("SessionID: %q", p.Raw.SessionID)
	}
	ti := p.Raw.ToolInput
	if ti["TargetFile"] != "/f.go" {
		t.Errorf("ToolInput.TargetFile: %v", ti["TargetFile"])
	}
	tr := p.Raw.ToolResponse
	if tr["success"] != true {
		t.Errorf("ToolResponse.success: %v", tr["success"])
	}
}
