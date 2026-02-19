package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultValues(t *testing.T) {
	cfg := Default()

	// Watch defaults
	if got := cfg.Watch.PollInterval.Duration; got != 1*time.Second {
		t.Errorf("PollInterval = %v, want %v", got, 1*time.Second)
	}
	if cfg.Watch.PollOnly {
		t.Error("PollOnly should default to false")
	}
	if got := cfg.Watch.Debounce.Duration; got != 100*time.Millisecond {
		t.Errorf("Debounce = %v, want %v", got, 100*time.Millisecond)
	}

	// Display defaults
	if got := cfg.Display.DiffStyle; got != "unified" {
		t.Errorf("DiffStyle = %q, want %q", got, "unified")
	}
	if got := cfg.Display.Theme; got != "dracula" {
		t.Errorf("Theme = %q, want %q", got, "dracula")
	}
	if cfg.Display.WordDiff {
		t.Error("WordDiff should default to false")
	}
	if got := cfg.Display.ContextLines; got != 3 {
		t.Errorf("ContextLines = %d, want %d", got, 3)
	}
	if !cfg.Display.ShowLineNumbers {
		t.Error("ShowLineNumbers should default to true")
	}

	// Timeline defaults
	if got := cfg.Timeline.MaxSnapshots; got != 1000 {
		t.Errorf("MaxSnapshots = %d, want %d", got, 1000)
	}

	// Filter defaults
	if cfg.Filter.ExtraIgnore != nil {
		t.Errorf("ExtraIgnore should be nil, got %v", cfg.Filter.ExtraIgnore)
	}

	// Editor defaults
	if got := cfg.Editor.Command; got != "" {
		t.Errorf("Editor.Command = %q, want empty string", got)
	}
}

func TestLoadFromMissingFile(t *testing.T) {
	cfg, err := LoadFrom("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("LoadFrom missing file should not error, got %v", err)
	}

	// Should return defaults
	if got := cfg.Display.Theme; got != "dracula" {
		t.Errorf("Theme = %q, want %q (default)", got, "dracula")
	}
	if got := cfg.Timeline.MaxSnapshots; got != 1000 {
		t.Errorf("MaxSnapshots = %d, want %d (default)", got, 1000)
	}
}

func TestLoadFromTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[watch]
poll_interval = "2s"
poll_only = true
debounce = "200ms"

[display]
diff_style = "side-by-side"
theme = "monokai"
word_diff = true
context_lines = 5
show_line_numbers = false

[timeline]
max_snapshots = 500

[filter]
extra_ignore = ["*.log", ".env*"]

[editor]
command = "code --goto"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom returned error: %v", err)
	}

	// Watch
	if got := cfg.Watch.PollInterval.Duration; got != 2*time.Second {
		t.Errorf("PollInterval = %v, want %v", got, 2*time.Second)
	}
	if !cfg.Watch.PollOnly {
		t.Error("PollOnly should be true")
	}
	if got := cfg.Watch.Debounce.Duration; got != 200*time.Millisecond {
		t.Errorf("Debounce = %v, want %v", got, 200*time.Millisecond)
	}

	// Display
	if got := cfg.Display.DiffStyle; got != "side-by-side" {
		t.Errorf("DiffStyle = %q, want %q", got, "side-by-side")
	}
	if got := cfg.Display.Theme; got != "monokai" {
		t.Errorf("Theme = %q, want %q", got, "monokai")
	}
	if !cfg.Display.WordDiff {
		t.Error("WordDiff should be true")
	}
	if got := cfg.Display.ContextLines; got != 5 {
		t.Errorf("ContextLines = %d, want %d", got, 5)
	}
	if cfg.Display.ShowLineNumbers {
		t.Error("ShowLineNumbers should be false")
	}

	// Timeline
	if got := cfg.Timeline.MaxSnapshots; got != 500 {
		t.Errorf("MaxSnapshots = %d, want %d", got, 500)
	}

	// Filter
	if got := len(cfg.Filter.ExtraIgnore); got != 2 {
		t.Fatalf("ExtraIgnore len = %d, want 2", got)
	}
	if cfg.Filter.ExtraIgnore[0] != "*.log" {
		t.Errorf("ExtraIgnore[0] = %q, want %q", cfg.Filter.ExtraIgnore[0], "*.log")
	}
	if cfg.Filter.ExtraIgnore[1] != ".env*" {
		t.Errorf("ExtraIgnore[1] = %q, want %q", cfg.Filter.ExtraIgnore[1], ".env*")
	}

	// Editor
	if got := cfg.Editor.Command; got != "code --goto" {
		t.Errorf("Editor.Command = %q, want %q", got, "code --goto")
	}
}

func TestLoadPartialTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	// Only override a few fields — rest should keep defaults
	content := `
[display]
theme = "nord"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom returned error: %v", err)
	}

	// Overridden value
	if got := cfg.Display.Theme; got != "nord" {
		t.Errorf("Theme = %q, want %q", got, "nord")
	}

	// Defaults preserved
	if got := cfg.Watch.PollInterval.Duration; got != 1*time.Second {
		t.Errorf("PollInterval = %v, want %v (default)", got, 1*time.Second)
	}
	if got := cfg.Display.DiffStyle; got != "unified" {
		t.Errorf("DiffStyle = %q, want %q (default)", got, "unified")
	}
	if got := cfg.Display.ContextLines; got != 3 {
		t.Errorf("ContextLines = %d, want %d (default)", got, 3)
	}
	if !cfg.Display.ShowLineNumbers {
		t.Error("ShowLineNumbers should default to true")
	}
	if got := cfg.Timeline.MaxSnapshots; got != 1000 {
		t.Errorf("MaxSnapshots = %d, want %d (default)", got, 1000)
	}
}

func TestLoadInvalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	if err := os.WriteFile(path, []byte("not valid [[[ toml"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("expected error for invalid TOML, got nil")
	}
}

func TestMergeCLIFlags(t *testing.T) {
	cfg := Default()

	pollInterval := 5 * time.Second
	pollOnly := true
	theme := "solarized-dark"
	diffStyle := "full-file"
	contextLines := 10
	maxSnapshots := 200

	flags := CLIFlags{
		PollInterval: &pollInterval,
		PollOnly:     &pollOnly,
		Theme:        &theme,
		DiffStyle:    &diffStyle,
		ContextLines: &contextLines,
		MaxSnapshots: &maxSnapshots,
		Ignore:       []string{"*.tmp", "build/**"},
	}

	merged := cfg.Merge(flags)

	if got := merged.Watch.PollInterval.Duration; got != 5*time.Second {
		t.Errorf("PollInterval = %v, want %v", got, 5*time.Second)
	}
	if !merged.Watch.PollOnly {
		t.Error("PollOnly should be true")
	}
	if got := merged.Display.Theme; got != "solarized-dark" {
		t.Errorf("Theme = %q, want %q", got, "solarized-dark")
	}
	if got := merged.Display.DiffStyle; got != "full-file" {
		t.Errorf("DiffStyle = %q, want %q", got, "full-file")
	}
	if got := merged.Display.ContextLines; got != 10 {
		t.Errorf("ContextLines = %d, want %d", got, 10)
	}
	if got := merged.Timeline.MaxSnapshots; got != 200 {
		t.Errorf("MaxSnapshots = %d, want %d", got, 200)
	}
	if got := len(merged.Filter.ExtraIgnore); got != 2 {
		t.Fatalf("ExtraIgnore len = %d, want 2", got)
	}

	// Values not overridden by flags should remain at defaults
	if got := merged.Watch.Debounce.Duration; got != 100*time.Millisecond {
		t.Errorf("Debounce = %v, want %v (default)", got, 100*time.Millisecond)
	}
	if merged.Display.WordDiff {
		t.Error("WordDiff should remain false (default)")
	}
	if !merged.Display.ShowLineNumbers {
		t.Error("ShowLineNumbers should remain true (default)")
	}
}

func TestMergeNilFlags(t *testing.T) {
	cfg := Default()
	flags := CLIFlags{} // all nil

	merged := cfg.Merge(flags)

	// Everything should remain at defaults
	if got := merged.Watch.PollInterval.Duration; got != 1*time.Second {
		t.Errorf("PollInterval = %v, want %v", got, 1*time.Second)
	}
	if got := merged.Display.Theme; got != "dracula" {
		t.Errorf("Theme = %q, want %q", got, "dracula")
	}
	if got := merged.Timeline.MaxSnapshots; got != 1000 {
		t.Errorf("MaxSnapshots = %d, want %d", got, 1000)
	}
}

func TestMergeFlagsOverrideTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[display]
theme = "monokai"
context_lines = 5

[timeline]
max_snapshots = 500

[filter]
extra_ignore = ["*.log"]
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom returned error: %v", err)
	}

	// CLI flags override TOML values
	theme := "nord"
	maxSnapshots := 100
	flags := CLIFlags{
		Theme:        &theme,
		MaxSnapshots: &maxSnapshots,
		Ignore:       []string{"*.tmp"},
	}

	merged := cfg.Merge(flags)

	// Overridden by CLI
	if got := merged.Display.Theme; got != "nord" {
		t.Errorf("Theme = %q, want %q (CLI override)", got, "nord")
	}
	if got := merged.Timeline.MaxSnapshots; got != 100 {
		t.Errorf("MaxSnapshots = %d, want %d (CLI override)", got, 100)
	}

	// Kept from TOML (not overridden by CLI)
	if got := merged.Display.ContextLines; got != 5 {
		t.Errorf("ContextLines = %d, want %d (from TOML)", got, 5)
	}

	// CLI ignore patterns appended to TOML ignore patterns
	if got := len(merged.Filter.ExtraIgnore); got != 2 {
		t.Fatalf("ExtraIgnore len = %d, want 2", got)
	}
	if merged.Filter.ExtraIgnore[0] != "*.log" {
		t.Errorf("ExtraIgnore[0] = %q, want %q (from TOML)", merged.Filter.ExtraIgnore[0], "*.log")
	}
	if merged.Filter.ExtraIgnore[1] != "*.tmp" {
		t.Errorf("ExtraIgnore[1] = %q, want %q (from CLI)", merged.Filter.ExtraIgnore[1], "*.tmp")
	}

	// Default value preserved (not in TOML, not in CLI)
	if got := merged.Watch.PollInterval.Duration; got != 1*time.Second {
		t.Errorf("PollInterval = %v, want %v (default)", got, 1*time.Second)
	}
}

func TestMergeDoesNotMutateOriginal(t *testing.T) {
	cfg := Default()
	theme := "nord"
	flags := CLIFlags{Theme: &theme}

	_ = cfg.Merge(flags)

	// Original config should be unchanged
	if got := cfg.Display.Theme; got != "dracula" {
		t.Errorf("Original config mutated: Theme = %q, want %q", got, "dracula")
	}
}

func TestDurationUnmarshal(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"100ms", 100 * time.Millisecond},
		{"1s", 1 * time.Second},
		{"500ms", 500 * time.Millisecond},
		{"2m", 2 * time.Minute},
		{"1h30m", 90 * time.Minute},
	}

	for _, tt := range tests {
		var d Duration
		if err := d.UnmarshalText([]byte(tt.input)); err != nil {
			t.Errorf("UnmarshalText(%q) error: %v", tt.input, err)
			continue
		}
		if d.Duration != tt.want {
			t.Errorf("UnmarshalText(%q) = %v, want %v", tt.input, d.Duration, tt.want)
		}
	}
}

func TestDurationUnmarshalInvalid(t *testing.T) {
	var d Duration
	if err := d.UnmarshalText([]byte("not-a-duration")); err == nil {
		t.Error("expected error for invalid duration, got nil")
	}
}

func TestDurationMarshal(t *testing.T) {
	d := Duration{500 * time.Millisecond}
	text, err := d.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText error: %v", err)
	}
	if string(text) != "500ms" {
		t.Errorf("MarshalText() = %q, want %q", string(text), "500ms")
	}
}

func TestSaveCreatesFileAndDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "dir", "config.toml")

	cfg := Default()
	cfg.Display.Theme = "monokai"
	cfg.Display.DiffStyle = "side-by-side"

	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	// Read it back
	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom error: %v", err)
	}

	if loaded.Display.Theme != "monokai" {
		t.Errorf("Theme = %q, want %q", loaded.Display.Theme, "monokai")
	}
	if loaded.Display.DiffStyle != "side-by-side" {
		t.Errorf("DiffStyle = %q, want %q", loaded.Display.DiffStyle, "side-by-side")
	}
	// Non-display defaults should be preserved
	if loaded.Watch.PollInterval.Duration != 1*time.Second {
		t.Errorf("PollInterval = %v, want %v", loaded.Watch.PollInterval.Duration, 1*time.Second)
	}
	if loaded.Timeline.MaxSnapshots != 1000 {
		t.Errorf("MaxSnapshots = %d, want %d", loaded.Timeline.MaxSnapshots, 1000)
	}
}

func TestSavePreservesAllFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := Default()
	cfg.Watch.PollOnly = true
	cfg.Display.Theme = "nord"
	cfg.Display.ContextLines = 7
	cfg.Filter.ExtraIgnore = []string{"*.log", "dist/**"}
	cfg.Editor.Command = "code --goto"

	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom error: %v", err)
	}

	if !loaded.Watch.PollOnly {
		t.Error("PollOnly should be true")
	}
	if loaded.Display.Theme != "nord" {
		t.Errorf("Theme = %q, want %q", loaded.Display.Theme, "nord")
	}
	if loaded.Display.ContextLines != 7 {
		t.Errorf("ContextLines = %d, want %d", loaded.Display.ContextLines, 7)
	}
	if len(loaded.Filter.ExtraIgnore) != 2 {
		t.Fatalf("ExtraIgnore len = %d, want 2", len(loaded.Filter.ExtraIgnore))
	}
	if loaded.Editor.Command != "code --goto" {
		t.Errorf("Editor.Command = %q, want %q", loaded.Editor.Command, "code --goto")
	}
}

func TestSaveOverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	// Write initial config
	cfg := Default()
	cfg.Display.Theme = "dracula"
	if err := cfg.Save(path); err != nil {
		t.Fatalf("first Save error: %v", err)
	}

	// Overwrite with new theme
	cfg.Display.Theme = "monokai"
	if err := cfg.Save(path); err != nil {
		t.Fatalf("second Save error: %v", err)
	}

	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom error: %v", err)
	}
	if loaded.Display.Theme != "monokai" {
		t.Errorf("Theme = %q, want %q", loaded.Display.Theme, "monokai")
	}
}

func TestMergeZeroValueFlags(t *testing.T) {
	cfg := Default()

	// Setting context_lines to 0 via CLI should work (zero is a valid value)
	contextLines := 0
	maxSnapshots := 0
	flags := CLIFlags{
		ContextLines: &contextLines,
		MaxSnapshots: &maxSnapshots,
	}

	merged := cfg.Merge(flags)

	if got := merged.Display.ContextLines; got != 0 {
		t.Errorf("ContextLines = %d, want 0 (explicit zero)", got)
	}
	if got := merged.Timeline.MaxSnapshots; got != 0 {
		t.Errorf("MaxSnapshots = %d, want 0 (explicit zero)", got)
	}
}
