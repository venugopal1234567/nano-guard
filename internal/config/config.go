// Package config implements loading and merging of Nano-Guard configuration.
//
// Load priority (highest → lowest):
//  1. Environment variables (NANO_GUARD_MODEL, OLLAMA_HOST, …)
//  2. Project-level  ./nano-guard.config.json
//  3. Global         ~/.config/nano-guard/config.json
//  4. Built-in defaults
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/bmatcuk/doublestar/v4"
)

// -----------------------------------------------------------------------
// Schema types
// -----------------------------------------------------------------------

// Rules holds the toggles for each code-quality check.
type Rules struct {
	UnhandledErrors  bool `json:"unhandled_errors"`
	DebugLogs        bool `json:"debug_logs"`
	TypeSafety       bool `json:"type_safety"`
	PlaceholderStubs bool `json:"placeholder_stubs"`
}

// Config is the fully-resolved configuration for a single invocation.
// All fields carry their built-in defaults; consumers should obtain a Config
// via Load() rather than constructing one manually.
type Config struct {
	Model          string   `json:"model"`
	OllamaHost     string   `json:"ollama_host"`
	TimeoutSeconds int      `json:"timeout_seconds"`
	MaxDiffLines   int      `json:"max_diff_lines"`
	FailOpen       bool     `json:"fail_open"`
	Rules          Rules    `json:"rules"`
	IgnorePaths    []string `json:"ignore_paths"`

	// compiledPatterns is populated lazily by ShouldIgnore; never serialised.
	// We store the validated pattern strings (invalid globs are dropped).
	compiledPatterns []string
}

// -----------------------------------------------------------------------
// Defaults
// -----------------------------------------------------------------------

// defaults returns a Config pre-filled with the spec-defined defaults.
func defaults() Config {
	return Config{
		Model:          "qwen2.5-coder:7b",
		OllamaHost:     "http://localhost:11434",
		TimeoutSeconds: 30,
		MaxDiffLines:   200,
		FailOpen:       true,
		Rules: Rules{
			UnhandledErrors:  true,
			DebugLogs:        true,
			TypeSafety:       true,
			PlaceholderStubs: true,
		},
		IgnorePaths: []string{},
	}
}

// -----------------------------------------------------------------------
// Partial JSON overlay
// -----------------------------------------------------------------------

// fileConfig mirrors Config but uses pointers so we can distinguish
// "not set" from "explicitly set to zero/false/empty".
type fileConfig struct {
	Model          *string    `json:"model"`
	OllamaHost     *string    `json:"ollama_host"`
	TimeoutSeconds *int       `json:"timeout_seconds"`
	MaxDiffLines   *int       `json:"max_diff_lines"`
	FailOpen       *bool      `json:"fail_open"`
	Rules          *fileRules `json:"rules"`
	IgnorePaths    []string   `json:"ignore_paths"`
}

type fileRules struct {
	UnhandledErrors  *bool `json:"unhandled_errors"`
	DebugLogs        *bool `json:"debug_logs"`
	TypeSafety       *bool `json:"type_safety"`
	PlaceholderStubs *bool `json:"placeholder_stubs"`
}

// applyFile overlays non-nil fields from fc onto dst.
func applyFile(dst *Config, fc fileConfig) {
	if fc.Model != nil {
		dst.Model = *fc.Model
	}
	if fc.OllamaHost != nil {
		dst.OllamaHost = *fc.OllamaHost
	}
	if fc.TimeoutSeconds != nil {
		dst.TimeoutSeconds = *fc.TimeoutSeconds
	}
	if fc.MaxDiffLines != nil {
		dst.MaxDiffLines = *fc.MaxDiffLines
	}
	if fc.FailOpen != nil {
		dst.FailOpen = *fc.FailOpen
	}
	if fc.Rules != nil {
		if fc.Rules.UnhandledErrors != nil {
			dst.Rules.UnhandledErrors = *fc.Rules.UnhandledErrors
		}
		if fc.Rules.DebugLogs != nil {
			dst.Rules.DebugLogs = *fc.Rules.DebugLogs
		}
		if fc.Rules.TypeSafety != nil {
			dst.Rules.TypeSafety = *fc.Rules.TypeSafety
		}
		if fc.Rules.PlaceholderStubs != nil {
			dst.Rules.PlaceholderStubs = *fc.Rules.PlaceholderStubs
		}
	}
	if fc.IgnorePaths != nil {
		dst.IgnorePaths = fc.IgnorePaths
	}
}

// -----------------------------------------------------------------------
// File loading
// -----------------------------------------------------------------------

// loadFile reads and parses a single JSON config file, returning a
// fileConfig on success.  If the file does not exist the returned bool is
// false and err is nil.  Any other read/parse error is returned as err.
func loadFile(path string) (fileConfig, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return fileConfig{}, false, nil
	}
	if err != nil {
		return fileConfig{}, false, fmt.Errorf("config: read %s: %w", path, err)
	}

	var fc fileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return fileConfig{}, false, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return fc, true, nil
}

// -----------------------------------------------------------------------
// Environment variable overrides
// -----------------------------------------------------------------------

// applyEnv reads environment variables and overlays them onto dst.
// Supported variables (all optional):
//
//	NANO_GUARD_MODEL          – overrides model
//	OLLAMA_HOST               – overrides ollama_host
//	NANO_GUARD_TIMEOUT        – overrides timeout_seconds (integer)
//	NANO_GUARD_MAX_DIFF_LINES – overrides max_diff_lines  (integer)
//	NANO_GUARD_FAIL_OPEN      – overrides fail_open       ("true"/"false")
func applyEnv(dst *Config) {
	if v := os.Getenv("NANO_GUARD_MODEL"); v != "" {
		dst.Model = v
	}
	if v := os.Getenv("OLLAMA_HOST"); v != "" {
		dst.OllamaHost = v
	}
	if v := os.Getenv("NANO_GUARD_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			dst.TimeoutSeconds = n
		}
	}
	if v := os.Getenv("NANO_GUARD_MAX_DIFF_LINES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			dst.MaxDiffLines = n
		}
	}
	if v := os.Getenv("NANO_GUARD_FAIL_OPEN"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			dst.FailOpen = b
		}
	}
}

// -----------------------------------------------------------------------
// Public API
// -----------------------------------------------------------------------

// globalConfigPath returns the path of the global config file.
// It respects XDG_CONFIG_HOME if set.
func globalConfigPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "nano-guard", "config.json")
}

// Load resolves the final Config by merging sources in priority order:
//  1. Built-in defaults
//  2. Global config  (~/.config/nano-guard/config.json)
//  3. Project config (./nano-guard.config.json in cwd)
//  4. Environment variables
//
// Returns an error only if a config file exists but cannot be parsed.
func Load() (Config, error) {
	cfg := defaults()

	// --- 3rd priority: global config --------------------------------
	globalPath := globalConfigPath()
	if fc, ok, err := loadFile(globalPath); err != nil {
		return cfg, err
	} else if ok {
		applyFile(&cfg, fc)
	}

	// --- 2nd priority: project config --------------------------------
	projectPath := "nano-guard.config.json"
	if fc, ok, err := loadFile(projectPath); err != nil {
		return cfg, err
	} else if ok {
		applyFile(&cfg, fc)
	}

	// --- 1st priority: environment variables -------------------------
	applyEnv(&cfg)

	return cfg, nil
}

// LoadFrom is like Load but accepts explicit paths for the global and
// project config files.  Pass an empty string to skip either file.
// This is used by tests and the `nano-guard init` verifier.
func LoadFrom(globalPath, projectPath string) (Config, error) {
	cfg := defaults()

	if globalPath != "" {
		if fc, ok, err := loadFile(globalPath); err != nil {
			return cfg, err
		} else if ok {
			applyFile(&cfg, fc)
		}
	}

	if projectPath != "" {
		if fc, ok, err := loadFile(projectPath); err != nil {
			return cfg, err
		} else if ok {
			applyFile(&cfg, fc)
		}
	}

	applyEnv(&cfg)
	return cfg, nil
}

// -----------------------------------------------------------------------
// Glob helper
// -----------------------------------------------------------------------

// compilePatterns validates and caches the ignore glob patterns.
// Invalid patterns are silently skipped (fail-open philosophy).
func (c *Config) compilePatterns() {
	if c.compiledPatterns != nil {
		return
	}
	c.compiledPatterns = make([]string, 0, len(c.IgnorePaths))
	for _, pattern := range c.IgnorePaths {
		if doublestar.ValidatePattern(pattern) {
			c.compiledPatterns = append(c.compiledPatterns, pattern)
		}
		// Invalid patterns are silently skipped (fail-open philosophy).
	}
}

// ShouldIgnore returns true if the given file path matches any of the
// configured ignore_paths glob patterns.
// Patterns follow gitignore-style doublestar syntax (e.g. **/vendor/**).
func (c *Config) ShouldIgnore(path string) bool {
	c.compilePatterns()
	for _, pattern := range c.compiledPatterns {
		if ok, _ := doublestar.Match(pattern, path); ok {
			return true
		}
	}
	return false
}
