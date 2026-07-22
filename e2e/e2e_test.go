//go:build integration

package e2e

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/venugopal1234567/nano-guard/internal/hook"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "nano-guard")

	cmd := exec.Command("go", "build", "-o", binPath, "../cmd/nano-guard")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build binary: %v\n%s", err, out)
	}

	return binPath
}

func runNanoGuardWithStdin(binPath, targetFile, cwd string) (int, string, string) {
	hookInput := hook.HookInput{
		HookEventName: "PostToolUse",
		SessionID:     "integration-test-session",
		ToolName:      "write_to_file",
		ToolInput: map[string]interface{}{
			"TargetFile": targetFile,
		},
		ToolResponse: map[string]interface{}{
			"filePath": targetFile,
			"success":  true,
		},
		Cwd: cwd,
	}

	inputBytes, _ := json.Marshal(hookInput)

	cmd := exec.Command(binPath)
	cmd.Dir = cwd
	cmd.Stdin = bytes.NewReader(inputBytes)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	}

	return exitCode, stdout.String(), stderr.String()
}

func initGitRepoWithFile(t *testing.T, filename, diffContent string) string {
	t.Helper()
	dir := t.TempDir()

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	runGit("init")
	runGit("config", "user.email", "e2e@nano-guard.dev")
	runGit("config", "user.name", "Nano Guard E2E")

	filePath := filepath.Join(dir, filename)
	os.WriteFile(filePath, []byte("// initial content\n"), 0644)
	runGit("add", filename)
	runGit("commit", "-m", "initial commit")

	// Apply change to produce diff
	os.WriteFile(filePath, []byte(diffContent), 0644)

	return dir
}

func TestIntegration_CleanDiff_Approved(t *testing.T) {
	binPath := buildBinary(t)
	cleanContent := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello World\")\n}\n"
	repoDir := initGitRepoWithFile(t, "main.go", cleanContent)

	exitCode, _, stderr := runNanoGuardWithStdin(binPath, "main.go", repoDir)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0 for clean diff, got %d. Stderr: %s", exitCode, stderr)
	}
}

func TestIntegration_UnhandledError_Rejected(t *testing.T) {
	binPath := buildBinary(t)
	badContent := "package main\n\nimport \"os\"\n\nfunc main() {\n\t// Error ignored\n\tos.Open(\"missing.txt\")\n}\n"
	repoDir := initGitRepoWithFile(t, "main.go", badContent)

	exitCode, _, stderr := runNanoGuardWithStdin(binPath, "main.go", repoDir)

	if exitCode != 2 {
		t.Fatalf("expected exit code 2 for unhandled error, got %d. Stderr: %s", exitCode, stderr)
	}

	if !strings.Contains(stderr, "UNHANDLED_ERROR") && !strings.Contains(stderr, "Code verification failed") {
		t.Fatalf("expected stderr to contain UNHANDLED_ERROR report, got: %s", stderr)
	}
}

func TestIntegration_DebugLog_Rejected(t *testing.T) {
	binPath := buildBinary(t)
	badContent := "package main\n\nimport \"fmt\"\n\nfunc process() {\n\tfmt.Println(\"DEBUG: processing\")\n}\n"
	repoDir := initGitRepoWithFile(t, "main.go", badContent)

	exitCode, _, stderr := runNanoGuardWithStdin(binPath, "main.go", repoDir)

	if exitCode != 2 {
		t.Fatalf("expected exit code 2 for debug log, got %d. Stderr: %s", exitCode, stderr)
	}

	if !strings.Contains(stderr, "DEBUG_LOG") && !strings.Contains(stderr, "Code verification failed") {
		t.Fatalf("expected stderr to contain DEBUG_LOG report, got: %s", stderr)
	}
}
