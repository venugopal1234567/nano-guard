package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

// initGitRepo creates a bare git repo in a temp dir, commits an initial file,
// and returns the repo path.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init")
	run("config", "user.email", "test@nano-guard.dev")
	run("config", "user.name", "Nano Guard Test")

	// Initial commit so HEAD exists
	initial := filepath.Join(dir, "README.md")
	os.WriteFile(initial, []byte("# test repo\n"), 0o644)
	run("add", "README.md")
	run("commit", "-m", "init")

	return dir
}

// writeFile writes content to path (creating parent dirs) and returns the path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// fixtureLines reads a testdata/diffs fixture and returns its line count.
func fixturePath(name string) string {
	return filepath.Join("..", "..", "testdata", "diffs", name)
}

// -----------------------------------------------------------------------
// truncateDiff
// -----------------------------------------------------------------------

func TestTruncateDiff_BelowLimit(t *testing.T) {
	diff := "line1\nline2\nline3"
	result := truncateDiff(diff, 10)
	if result != diff {
		t.Errorf("expected unchanged diff, got %q", result)
	}
}

func TestTruncateDiff_AtLimit(t *testing.T) {
	lines := make([]string, 5)
	for i := range lines {
		lines[i] = fmt.Sprintf("line%d", i+1)
	}
	diff := strings.Join(lines, "\n")
	result := truncateDiff(diff, 5)
	if result != diff {
		t.Errorf("at-limit: expected unchanged diff")
	}
}

func TestTruncateDiff_ExceedsLimit(t *testing.T) {
	lines := make([]string, 10)
	for i := range lines {
		lines[i] = fmt.Sprintf("line%d", i+1)
	}
	diff := strings.Join(lines, "\n")
	result := truncateDiff(diff, 5)

	resultLines := strings.Split(result, "\n")
	// 5 content lines + 1 footer line
	if len(resultLines) != 6 {
		t.Errorf("expected 6 lines (5 content + footer), got %d", len(resultLines))
	}
	if !strings.Contains(result, "[... diff truncated at 5 lines ...]") {
		t.Errorf("missing truncation footer in: %q", result)
	}
	if !strings.HasPrefix(result, "line1") {
		t.Errorf("should start with first line, got %q", result[:20])
	}
}

func TestTruncateDiff_ZeroLimit_NoTruncation(t *testing.T) {
	diff := strings.Repeat("line\n", 1000)
	result := truncateDiff(diff, 0)
	if result != diff {
		t.Error("zero maxLines: expected no truncation")
	}
}

func TestTruncateDiff_NegativeLimit_NoTruncation(t *testing.T) {
	diff := "a\nb\nc"
	result := truncateDiff(diff, -1)
	if result != diff {
		t.Error("negative maxLines: expected no truncation")
	}
}

func TestTruncateDiff_FooterFormat(t *testing.T) {
	diff := strings.Join([]string{"a", "b", "c", "d", "e", "f"}, "\n")
	result := truncateDiff(diff, 3)
	want := "a\nb\nc\n[... diff truncated at 3 lines ...]"
	if result != want {
		t.Errorf("footer format:\ngot  %q\nwant %q", result, want)
	}
}

// -----------------------------------------------------------------------
// truncateDiff — large fixture
// -----------------------------------------------------------------------

func TestTruncateDiff_LargeFixture(t *testing.T) {
	data, err := os.ReadFile(fixturePath("large_500_lines.diff"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	diff := string(data)
	result := truncateDiff(diff, 200)

	lines := strings.Split(result, "\n")
	// 200 content lines + 1 footer line
	if len(lines) != 201 {
		t.Errorf("expected 201 lines, got %d", len(lines))
	}
	if !strings.Contains(result, "[... diff truncated at 200 lines ...]") {
		t.Errorf("missing truncation footer")
	}
}

// -----------------------------------------------------------------------
// buildPseudoDiff
// -----------------------------------------------------------------------

func TestBuildPseudoDiff_SingleLine(t *testing.T) {
	result := buildPseudoDiff("/tmp/foo.go", "package main")
	if !strings.Contains(result, "--- /dev/null") {
		t.Error("missing '--- /dev/null' header")
	}
	if !strings.Contains(result, "+++ /tmp/foo.go") {
		t.Error("missing '+++ <file>' header")
	}
	if !strings.Contains(result, "@@ -0,0 +1,1 @@") {
		t.Error("missing hunk header for 1 line")
	}
	if !strings.Contains(result, "+package main") {
		t.Error("missing +package main line")
	}
}

func TestBuildPseudoDiff_MultiLine(t *testing.T) {
	content := "package main\n\nfunc main() {}\n"
	result := buildPseudoDiff("/proj/main.go", content)

	if !strings.Contains(result, "@@ -0,0 +1,3 @@") {
		t.Errorf("hunk header: expected +1,3 for 3 non-empty lines\nresult: %s", result)
	}
	if !strings.Contains(result, "+package main") {
		t.Error("missing +package main")
	}
	if !strings.Contains(result, "+func main() {}") {
		t.Error("missing +func main")
	}
}

func TestBuildPseudoDiff_AllLinesHavePlusPrefix(t *testing.T) {
	content := "line1\nline2\nline3"
	result := buildPseudoDiff("/f.go", content)
	// Skip the 3 header lines and check every remaining line starts with +
	lines := strings.Split(result, "\n")
	for _, line := range lines[3:] {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "+") {
			t.Errorf("non-plus line in pseudo-diff body: %q", line)
		}
	}
}

func TestBuildPseudoDiff_FilePath(t *testing.T) {
	result := buildPseudoDiff("relative/path/file.ts", "const x = 1")
	if !strings.Contains(result, "+++ relative/path/file.ts") {
		t.Errorf("file path not in header: %q", result)
	}
}

// -----------------------------------------------------------------------
// runGitDiff
// -----------------------------------------------------------------------

func TestRunGitDiff_NotARepo(t *testing.T) {
	// Use a temp dir that is not a git repo
	dir := t.TempDir()
	_, err := runGitDiff(dir, "file.go", false)
	if err == nil {
		t.Error("expected error for non-git directory")
	}
}

func TestRunGitDiff_HeadDiff(t *testing.T) {
	dir := initGitRepo(t)

	// Write a file and stage + commit it, then modify it (unstaged)
	writeFile(t, dir, "main.go", "package main\n\nfunc a() {}\n")
	exec.Command("git", "-C", dir, "add", "main.go").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "add main").Run()

	// Now modify it without staging — creates HEAD diff
	writeFile(t, dir, "main.go", "package main\n\nfunc a() {}\n\nfunc b() {}\n")

	diff, err := runGitDiff(dir, "main.go", false)
	if err != nil {
		t.Fatalf("runGitDiff HEAD: %v", err)
	}
	if !strings.Contains(diff, "+func b()") {
		t.Errorf("expected +func b() in diff, got:\n%s", diff)
	}
}

func TestRunGitDiff_CachedDiff(t *testing.T) {
	dir := initGitRepo(t)

	// Write and stage (but don't commit) — creates cached diff
	writeFile(t, dir, "server.go", "package main\n\nfunc serve() {}\n")
	exec.Command("git", "-C", dir, "add", "server.go").Run()

	diff, err := runGitDiff(dir, "server.go", true)
	if err != nil {
		t.Fatalf("runGitDiff cached: %v", err)
	}
	if !strings.Contains(diff, "+func serve()") {
		t.Errorf("expected +func serve() in staged diff, got:\n%s", diff)
	}
}

func TestRunGitDiff_EmptyWhenNoChanges(t *testing.T) {
	dir := initGitRepo(t)

	// Committed clean file — no changes
	writeFile(t, dir, "clean.go", "package main\n")
	exec.Command("git", "-C", dir, "add", "clean.go").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "add clean").Run()

	diff, err := runGitDiff(dir, "clean.go", false)
	if err != nil {
		t.Fatalf("runGitDiff: %v", err)
	}
	if diff != "" {
		t.Errorf("expected empty diff for committed clean file, got %q", diff)
	}
}

// -----------------------------------------------------------------------
// ExtractDiff — strategy 1: git diff HEAD
// -----------------------------------------------------------------------

func TestExtractDiff_Strategy1_GitDiffHEAD(t *testing.T) {
	dir := initGitRepo(t)

	// Commit a file then modify it (unstaged)
	writeFile(t, dir, "app.go", "package main\n\nfunc old() {}\n")
	exec.Command("git", "-C", dir, "add", "app.go").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "add app").Run()
	writeFile(t, dir, "app.go", "package main\n\nfunc old() {}\n\nfunc new() {}\n")

	diff, err := ExtractDiff(dir, "app.go", "", 200)
	if err != nil {
		t.Fatalf("ExtractDiff: %v", err)
	}
	if !strings.Contains(diff, "+func new()") {
		t.Errorf("strategy 1: expected +func new() in diff:\n%s", diff)
	}
}

// -----------------------------------------------------------------------
// ExtractDiff — strategy 2: git diff --cached
// -----------------------------------------------------------------------

func TestExtractDiff_Strategy2_GitDiffCached(t *testing.T) {
	dir := initGitRepo(t)

	// Stage a new file without committing (no HEAD diff, but cached diff exists)
	writeFile(t, dir, "worker.go", "package main\n\nfunc work() {}\n")
	exec.Command("git", "-C", dir, "add", "worker.go").Run()

	diff, err := ExtractDiff(dir, "worker.go", "", 200)
	if err != nil {
		t.Fatalf("ExtractDiff strategy 2: %v", err)
	}
	if diff == "" {
		t.Error("strategy 2: expected non-empty diff for staged file")
	}
	if !strings.Contains(diff, "+func work()") {
		t.Errorf("strategy 2: expected +func work(), got:\n%s", diff)
	}
}

// -----------------------------------------------------------------------
// ExtractDiff — strategy 3: CodeContent pseudo-diff
// -----------------------------------------------------------------------

func TestExtractDiff_Strategy3_CodeContent(t *testing.T) {
	// Use a non-git temp dir
	dir := t.TempDir()
	content := "package main\n\nfunc handler() {}\n"

	diff, err := ExtractDiff(dir, "handler.go", content, 200)
	if err != nil {
		t.Fatalf("ExtractDiff strategy 3: %v", err)
	}
	if !strings.Contains(diff, "--- /dev/null") {
		t.Error("strategy 3: expected pseudo-diff header")
	}
	if !strings.Contains(diff, "+func handler()") {
		t.Errorf("strategy 3: expected +func handler(), got:\n%s", diff)
	}
}

// -----------------------------------------------------------------------
// ExtractDiff — strategy 4: full file read
// -----------------------------------------------------------------------

func TestExtractDiff_Strategy4_ReadFile(t *testing.T) {
	dir := t.TempDir()
	// Write file to disk — no git, no CodeContent
	writeFile(t, dir, "util.go", "package util\n\nfunc Helper() {}\n")

	diff, err := ExtractDiff(dir, "util.go", "", 200)
	if err != nil {
		t.Fatalf("ExtractDiff strategy 4: %v", err)
	}
	if !strings.Contains(diff, "--- /dev/null") {
		t.Error("strategy 4: expected pseudo-diff header from file read")
	}
	if !strings.Contains(diff, "+func Helper()") {
		t.Errorf("strategy 4: expected +func Helper(), got:\n%s", diff)
	}
}

func TestExtractDiff_Strategy4_AbsolutePath(t *testing.T) {
	dir := t.TempDir()
	absPath := writeFile(t, dir, "abs.go", "package abs\n\nvar X = 1\n")

	diff, err := ExtractDiff(dir, absPath, "", 200)
	if err != nil {
		t.Fatalf("ExtractDiff abs path: %v", err)
	}
	if !strings.Contains(diff, "+var X = 1") {
		t.Errorf("absolute path: expected +var X = 1, got:\n%s", diff)
	}
}

// -----------------------------------------------------------------------
// ExtractDiff — empty result
// -----------------------------------------------------------------------

func TestExtractDiff_NothingFound_Empty(t *testing.T) {
	dir := t.TempDir()
	// File doesn't exist, no git, no CodeContent
	diff, err := ExtractDiff(dir, "nonexistent.go", "", 200)
	if err != nil {
		t.Fatalf("ExtractDiff nothing: %v", err)
	}
	if diff != "" {
		t.Errorf("expected empty diff, got %q", diff)
	}
}

// -----------------------------------------------------------------------
// ExtractDiff — truncation applied end-to-end
// -----------------------------------------------------------------------

func TestExtractDiff_Truncation_Applied(t *testing.T) {
	dir := t.TempDir()
	// Build content with 50 lines
	var lines []string
	for i := 1; i <= 50; i++ {
		lines = append(lines, fmt.Sprintf("func fn%d() {}", i))
	}
	content := strings.Join(lines, "\n")

	diff, err := ExtractDiff(dir, "big.go", content, 10)
	if err != nil {
		t.Fatalf("ExtractDiff truncation: %v", err)
	}
	if !strings.Contains(diff, "[... diff truncated at 10 lines ...]") {
		t.Errorf("expected truncation footer, got:\n%s", diff)
	}
	resultLines := strings.Split(diff, "\n")
	if len(resultLines) != 11 { // 10 content + 1 footer
		t.Errorf("expected 11 lines after truncation, got %d", len(resultLines))
	}
}

func TestExtractDiff_NoTruncation_WhenUnderLimit(t *testing.T) {
	dir := t.TempDir()
	content := "package main\nfunc a() {}"

	diff, err := ExtractDiff(dir, "small.go", content, 200)
	if err != nil {
		t.Fatalf("ExtractDiff: %v", err)
	}
	if strings.Contains(diff, "truncated") {
		t.Errorf("unexpected truncation for small diff: %s", diff)
	}
}

// -----------------------------------------------------------------------
// Fixture smoke tests (verify fixture files are well-formed)
// -----------------------------------------------------------------------

func TestFixture_ApprovedClean(t *testing.T) {
	data, err := os.ReadFile(fixturePath("approved_clean.diff"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if !strings.Contains(string(data), "diff --git") {
		t.Error("approved_clean.diff: missing 'diff --git' header")
	}
}

func TestFixture_Large500Lines(t *testing.T) {
	data, err := os.ReadFile(fixturePath("large_500_lines.diff"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) < 500 {
		t.Errorf("large fixture: expected >= 500 lines, got %d", len(lines))
	}
}
