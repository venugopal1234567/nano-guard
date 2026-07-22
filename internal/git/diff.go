// Package git implements context extraction for Nano-Guard.
//
// It runs `git diff` to obtain the code change that was just written, with a
// four-level fallback waterfall as specified in specs/05-context-extraction.md.
//
// Strategy waterfall:
//
//  1. git diff HEAD --unified=3 -- <file>   (unstaged changes)
//  2. git diff --cached --unified=3 -- <file> (staged changes)
//  3. Pseudo-diff built from CodeContent    (non-git: write_to_file content)
//  4. Read full file from disk              (last resort)
//
// If all strategies return empty, ExtractDiff returns ("", nil) and callers
// should exit 0 — nothing to evaluate.
package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// -----------------------------------------------------------------------
// Public API
// -----------------------------------------------------------------------

// ExtractDiff returns the unified diff for filePath within cwd, falling back
// through the strategy waterfall defined in spec §1.
//
// codeContent is the value of tool_input["CodeContent"] when available
// (write_to_file calls). Pass an empty string when not available.
//
// maxLines is the max_diff_lines config value; diff is truncated when exceeded.
func ExtractDiff(cwd, filePath, codeContent string, maxLines int) (string, error) {
	// --- Strategy 1: git diff HEAD (unstaged) -------------------------
	if diff, err := runGitDiff(cwd, filePath, false); err == nil && diff != "" {
		return truncateDiff(diff, maxLines), nil
	}

	// --- Strategy 2: git diff --cached (staged) -----------------------
	if diff, err := runGitDiff(cwd, filePath, true); err == nil && diff != "" {
		return truncateDiff(diff, maxLines), nil
	}

	// --- Strategy 3: pseudo-diff from CodeContent ---------------------
	if codeContent != "" {
		diff := buildPseudoDiff(filePath, codeContent)
		return truncateDiff(diff, maxLines), nil
	}

	// --- Strategy 4: read full file from disk -------------------------
	absPath := filePath
	if !filepath.IsAbs(filePath) {
		absPath = filepath.Join(cwd, filePath)
	}
	data, err := os.ReadFile(absPath)
	if err == nil && len(data) > 0 {
		diff := buildPseudoDiff(filePath, string(data))
		return truncateDiff(diff, maxLines), nil
	}

	// Nothing found — caller should exit 0.
	return "", nil
}

// -----------------------------------------------------------------------
// Git diff runner
// -----------------------------------------------------------------------

// runGitDiff executes the git diff command for a single file.
// When cached is true it runs `git diff --cached`, otherwise `git diff HEAD`.
// Returns ("", err) for any error (git not installed, not a repo, etc.).
func runGitDiff(cwd, filePath string, cached bool) (string, error) {
	args := []string{"diff", "--unified=3"}
	if cached {
		args = append(args, "--cached")
	} else {
		args = append(args, "HEAD")
	}
	args = append(args, "--", filePath)

	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// -----------------------------------------------------------------------
// Pseudo-diff builder (non-git fallback)
// -----------------------------------------------------------------------

// buildPseudoDiff formats content as a unified diff from /dev/null, mimicking
// a brand-new file addition. Matches the format specified in spec §4.
func buildPseudoDiff(filePath, content string) string {
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	var sb strings.Builder

	fmt.Fprintf(&sb, "--- /dev/null\n")
	fmt.Fprintf(&sb, "+++ %s\n", filePath)
	fmt.Fprintf(&sb, "@@ -0,0 +1,%d @@\n", len(lines))
	for _, line := range lines {
		sb.WriteString("+")
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

// -----------------------------------------------------------------------
// Truncation
// -----------------------------------------------------------------------

// truncateDiff limits the diff to maxLines lines.
// When truncation occurs a footer notice is appended per spec §3.
// If maxLines <= 0 the full diff is returned unchanged.
func truncateDiff(diff string, maxLines int) string {
	if maxLines <= 0 {
		return diff
	}
	lines := strings.Split(diff, "\n")
	if len(lines) <= maxLines {
		return diff
	}
	truncated := strings.Join(lines[:maxLines], "\n")
	return truncated + fmt.Sprintf("\n[... diff truncated at %d lines ...]", maxLines)
}
