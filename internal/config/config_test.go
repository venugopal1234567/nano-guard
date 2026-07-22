package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

// writeJSON writes v as a JSON file at path, creating parent dirs as needed.
func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("writeJSON MkdirAll: %v", err)
	}
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("writeJSON Marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writeJSON WriteFile: %v", err)
	}
}

// tmpFile creates a named temp file with JSON content and returns its path.
func tmpJSON(t *testing.T, v any) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "nano-guard-*.json")
	if err != nil {
		t.Fatalf("tmpJSON CreateTemp: %v", err)
	}
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("tmpJSON Marshal: %v", err)
	}
	if _, err := f.Write(data); err != nil {
		t.Fatalf("tmpJSON Write: %v", err)
	}
	f.Close()
	return f.Name()
}

// -----------------------------------------------------------------------
// defaults()
// -----------------------------------------------------------------------

func TestDefaults(t *testing.T) {
	cfg := defaults()

	if cfg.Model != "qwen2.5-coder:7b" {
		t.Errorf("Model: got %q, want %q", cfg.Model, "qwen2.5-coder:7b")
	}
	if cfg.OllamaHost != "http://localhost:11434" {
		t.Errorf("OllamaHost: got %q", cfg.OllamaHost)
	}
	if cfg.TimeoutSeconds != 30 {
		t.Errorf("TimeoutSeconds: got %d, want 30", cfg.TimeoutSeconds)
	}
	if cfg.MaxDiffLines != 200 {
		t.Errorf("MaxDiffLines: got %d, want 200", cfg.MaxDiffLines)
	}
	if !cfg.FailOpen {
		t.Error("FailOpen: want true")
	}
	if !cfg.Rules.UnhandledErrors {
		t.Error("Rules.UnhandledErrors: want true")
	}
	if !cfg.Rules.DebugLogs {
		t.Error("Rules.DebugLogs: want true")
	}
	if !cfg.Rules.TypeSafety {
		t.Error("Rules.TypeSafety: want true")
	}
	if !cfg.Rules.PlaceholderStubs {
		t.Error("Rules.PlaceholderStubs: want true")
	}
	if len(cfg.IgnorePaths) != 0 {
		t.Errorf("IgnorePaths: want empty, got %v", cfg.IgnorePaths)
	}
}

// -----------------------------------------------------------------------
// loadFile()
// -----------------------------------------------------------------------

func TestLoadFile_NotExist(t *testing.T) {
	_, ok, err := loadFile("/does/not/exist/config.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false for missing file")
	}
}

func TestLoadFile_Malformed(t *testing.T) {
	f, _ := os.CreateTemp(t.TempDir(), "bad-*.json")
	f.WriteString("{not valid json")
	f.Close()

	_, _, err := loadFile(f.Name())
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestLoadFile_ValidPartial(t *testing.T) {
	path := tmpJSON(t, map[string]any{
		"model": "llama3:8b",
	})
	fc, ok, err := loadFile(path)
	if err != nil || !ok {
		t.Fatalf("loadFile: ok=%v err=%v", ok, err)
	}
	if fc.Model == nil || *fc.Model != "llama3:8b" {
		t.Errorf("Model: got %v", fc.Model)
	}
	// Fields not in the JSON must be nil (not-set)
	if fc.TimeoutSeconds != nil {
		t.Errorf("TimeoutSeconds should be nil, got %v", *fc.TimeoutSeconds)
	}
}

// -----------------------------------------------------------------------
// applyFile()
// -----------------------------------------------------------------------

func TestApplyFile_PartialOverride(t *testing.T) {
	cfg := defaults()
	timeout := 60
	fc := fileConfig{
		TimeoutSeconds: &timeout,
	}
	applyFile(&cfg, fc)

	if cfg.TimeoutSeconds != 60 {
		t.Errorf("TimeoutSeconds: got %d, want 60", cfg.TimeoutSeconds)
	}
	// Untouched fields should still be defaults
	if cfg.Model != "qwen2.5-coder:7b" {
		t.Errorf("Model should be default, got %q", cfg.Model)
	}
}

func TestApplyFile_RulesPartialOverride(t *testing.T) {
	cfg := defaults()
	f := false
	fc := fileConfig{
		Rules: &fileRules{DebugLogs: &f},
	}
	applyFile(&cfg, fc)

	if cfg.Rules.DebugLogs {
		t.Error("DebugLogs: want false after override")
	}
	// Other rules must be unchanged
	if !cfg.Rules.UnhandledErrors {
		t.Error("UnhandledErrors: want true (unchanged)")
	}
}

func TestApplyFile_IgnorePathsReplaced(t *testing.T) {
	cfg := defaults()
	fc := fileConfig{
		IgnorePaths: []string{"**/*.test.ts"},
	}
	applyFile(&cfg, fc)

	if len(cfg.IgnorePaths) != 1 || cfg.IgnorePaths[0] != "**/*.test.ts" {
		t.Errorf("IgnorePaths: got %v", cfg.IgnorePaths)
	}
}

// -----------------------------------------------------------------------
// applyEnv()
// -----------------------------------------------------------------------

func TestApplyEnv_Model(t *testing.T) {
	t.Setenv("NANO_GUARD_MODEL", "mistral:7b")
	cfg := defaults()
	applyEnv(&cfg)
	if cfg.Model != "mistral:7b" {
		t.Errorf("Model: got %q, want %q", cfg.Model, "mistral:7b")
	}
}

func TestApplyEnv_OllamaHost(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "http://remote:11434")
	cfg := defaults()
	applyEnv(&cfg)
	if cfg.OllamaHost != "http://remote:11434" {
		t.Errorf("OllamaHost: got %q", cfg.OllamaHost)
	}
}

func TestApplyEnv_Timeout(t *testing.T) {
	t.Setenv("NANO_GUARD_TIMEOUT", "120")
	cfg := defaults()
	applyEnv(&cfg)
	if cfg.TimeoutSeconds != 120 {
		t.Errorf("TimeoutSeconds: got %d", cfg.TimeoutSeconds)
	}
}

func TestApplyEnv_MaxDiffLines(t *testing.T) {
	t.Setenv("NANO_GUARD_MAX_DIFF_LINES", "500")
	cfg := defaults()
	applyEnv(&cfg)
	if cfg.MaxDiffLines != 500 {
		t.Errorf("MaxDiffLines: got %d", cfg.MaxDiffLines)
	}
}

func TestApplyEnv_FailOpen_False(t *testing.T) {
	t.Setenv("NANO_GUARD_FAIL_OPEN", "false")
	cfg := defaults()
	applyEnv(&cfg)
	if cfg.FailOpen {
		t.Error("FailOpen: want false")
	}
}

func TestApplyEnv_InvalidInteger(t *testing.T) {
	t.Setenv("NANO_GUARD_TIMEOUT", "notanumber")
	cfg := defaults()
	applyEnv(&cfg)
	// Should keep default, not crash
	if cfg.TimeoutSeconds != 30 {
		t.Errorf("TimeoutSeconds: want default 30, got %d", cfg.TimeoutSeconds)
	}
}

func TestApplyEnv_InvalidBool(t *testing.T) {
	t.Setenv("NANO_GUARD_FAIL_OPEN", "yep")
	cfg := defaults()
	applyEnv(&cfg)
	// Should keep default, not crash
	if !cfg.FailOpen {
		t.Error("FailOpen: want default true after invalid env value")
	}
}

// -----------------------------------------------------------------------
// LoadFrom() — merge priority tests
// -----------------------------------------------------------------------

func TestLoadFrom_OnlyDefaults(t *testing.T) {
	cfg, err := LoadFrom("", "")
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	want := defaults()
	if cfg.Model != want.Model {
		t.Errorf("Model: got %q, want %q", cfg.Model, want.Model)
	}
}

func TestLoadFrom_GlobalOnly(t *testing.T) {
	global := tmpJSON(t, map[string]any{"model": "global-model"})
	cfg, err := LoadFrom(global, "")
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if cfg.Model != "global-model" {
		t.Errorf("Model: got %q", cfg.Model)
	}
}

func TestLoadFrom_ProjectOverridesGlobal(t *testing.T) {
	global := tmpJSON(t, map[string]any{"model": "global-model", "timeout_seconds": 60})
	project := tmpJSON(t, map[string]any{"model": "project-model"})
	cfg, err := LoadFrom(global, project)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if cfg.Model != "project-model" {
		t.Errorf("Model: want project-model, got %q", cfg.Model)
	}
	// Timeout came from global and was not overridden by project
	if cfg.TimeoutSeconds != 60 {
		t.Errorf("TimeoutSeconds: want 60 (from global), got %d", cfg.TimeoutSeconds)
	}
}

func TestLoadFrom_EnvOverridesProject(t *testing.T) {
	t.Setenv("NANO_GUARD_MODEL", "env-model")
	project := tmpJSON(t, map[string]any{"model": "project-model"})
	cfg, err := LoadFrom("", project)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if cfg.Model != "env-model" {
		t.Errorf("Model: want env-model (env wins), got %q", cfg.Model)
	}
}

func TestLoadFrom_MalformedProject(t *testing.T) {
	f, _ := os.CreateTemp(t.TempDir(), "bad-*.json")
	f.WriteString("!!!")
	f.Close()
	_, err := LoadFrom("", f.Name())
	if err == nil {
		t.Fatal("expected error for malformed project config")
	}
}

func TestLoadFrom_FullSpecExample(t *testing.T) {
	// Matches the exact example from spec 03-config-schema.md
	project := tmpJSON(t, map[string]any{
		"model":           "qwen2.5-coder:7b",
		"ollama_host":     "http://localhost:11434",
		"timeout_seconds": 30,
		"max_diff_lines":  200,
		"fail_open":       true,
		"rules": map[string]any{
			"unhandled_errors":  true,
			"debug_logs":        true,
			"type_safety":       true,
			"placeholder_stubs": true,
		},
		"ignore_paths": []string{
			"**/*.test.ts",
			"**/*.spec.go",
			"**/vendor/**",
			"**/node_modules/**",
		},
	})
	cfg, err := LoadFrom("", project)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if len(cfg.IgnorePaths) != 4 {
		t.Errorf("IgnorePaths: want 4, got %d", len(cfg.IgnorePaths))
	}
}

// -----------------------------------------------------------------------
// ShouldIgnore()
// -----------------------------------------------------------------------

func TestShouldIgnore_TestFile(t *testing.T) {
	cfg := Config{
		IgnorePaths: []string{"**/*.test.ts"},
	}
	if !cfg.ShouldIgnore("src/foo.test.ts") {
		t.Error("expected src/foo.test.ts to be ignored")
	}
	if cfg.ShouldIgnore("src/foo.ts") {
		t.Error("expected src/foo.ts NOT to be ignored")
	}
}

func TestShouldIgnore_VendorDir(t *testing.T) {
	cfg := Config{
		IgnorePaths: []string{"**/vendor/**"},
	}
	if !cfg.ShouldIgnore("vendor/somelib/file.go") {
		t.Error("expected vendor path to be ignored")
	}
	if cfg.ShouldIgnore("internal/vendor.go") {
		t.Error("expected internal/vendor.go NOT to be ignored")
	}
}

func TestShouldIgnore_MultiplePatterns(t *testing.T) {
	cfg := Config{
		IgnorePaths: []string{"**/*.test.ts", "**/node_modules/**"},
	}
	if !cfg.ShouldIgnore("node_modules/react/index.js") {
		t.Error("node_modules: expected ignored")
	}
	if !cfg.ShouldIgnore("src/app.test.ts") {
		t.Error("test file: expected ignored")
	}
	if cfg.ShouldIgnore("src/app.ts") {
		t.Error("src/app.ts: expected NOT ignored")
	}
}

func TestShouldIgnore_NoPatterns(t *testing.T) {
	cfg := Config{}
	if cfg.ShouldIgnore("anything/at/all.go") {
		t.Error("expected no ignores when IgnorePaths is empty")
	}
}

func TestShouldIgnore_InvalidPattern(t *testing.T) {
	// Invalid glob patterns are skipped silently (fail-open)
	cfg := Config{
		IgnorePaths: []string{"[invalid"},
	}
	// Must not panic and must return false
	if cfg.ShouldIgnore("some/file.go") {
		t.Error("expected false for invalid-pattern config")
	}
}

func TestShouldIgnore_GlobsCompiledOnce(t *testing.T) {
	cfg := Config{
		IgnorePaths: []string{"**/*.go"},
	}
	// Call twice to exercise the "already compiled" branch
	cfg.ShouldIgnore("foo.go")
	cfg.ShouldIgnore("bar.go")
	if len(cfg.compiledPatterns) != 1 {
		t.Errorf("compiledPatterns should be length 1, got %d", len(cfg.compiledPatterns))
	}
}

// -----------------------------------------------------------------------
// globalConfigPath()
// -----------------------------------------------------------------------

func TestGlobalConfigPath_XDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/xdg")
	path := globalConfigPath()
	want := "/custom/xdg/nano-guard/config.json"
	if path != want {
		t.Errorf("globalConfigPath: got %q, want %q", path, want)
	}
}

func TestGlobalConfigPath_Default(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	path := globalConfigPath()
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".config", "nano-guard", "config.json")
	if path != want {
		t.Errorf("globalConfigPath: got %q, want %q", path, want)
	}
}

// -----------------------------------------------------------------------
// Load() — smoke test (no config files on disk, no env overrides)
// -----------------------------------------------------------------------

func TestLoad_SmokeTest(t *testing.T) {
	// Ensure no env overrides are set
	for _, k := range []string{
		"NANO_GUARD_MODEL", "OLLAMA_HOST",
		"NANO_GUARD_TIMEOUT", "NANO_GUARD_MAX_DIFF_LINES", "NANO_GUARD_FAIL_OPEN",
	} {
		t.Setenv(k, "")
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	want := defaults()
	if cfg.Model != want.Model {
		t.Errorf("Model: got %q, want %q", cfg.Model, want.Model)
	}
}

// -----------------------------------------------------------------------
// JSON round-trip
// -----------------------------------------------------------------------

func TestConfig_JSONRoundTrip(t *testing.T) {
	cfg := defaults()
	cfg.Model = "codellama:7b"
	cfg.IgnorePaths = []string{"**/vendor/**"}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var out Config
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.Model != cfg.Model {
		t.Errorf("Model: got %q, want %q", out.Model, cfg.Model)
	}
	if len(out.IgnorePaths) != 1 {
		t.Errorf("IgnorePaths: got %v", out.IgnorePaths)
	}
}
