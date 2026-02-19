// Package config handles loading and merging configuration from TOML files and CLI flags.
package config

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

// Config holds all configuration for hark.
type Config struct {
	Watch    WatchConfig    `toml:"watch"`
	Display  DisplayConfig  `toml:"display"`
	Timeline TimelineConfig `toml:"timeline"`
	Filter   FilterConfig   `toml:"filter"`
	Editor   EditorConfig   `toml:"editor"`
}

// WatchConfig controls change detection behaviour.
type WatchConfig struct {
	PollInterval Duration `toml:"poll_interval"`
	PollOnly     bool     `toml:"poll_only"`
	Debounce     Duration `toml:"debounce"`
}

// DisplayConfig controls diff rendering.
type DisplayConfig struct {
	DiffStyle       string `toml:"diff_style"`
	Theme           string `toml:"theme"`
	WordDiff        bool   `toml:"word_diff"`
	ContextLines    int    `toml:"context_lines"`
	ShowLineNumbers bool   `toml:"show_line_numbers"`
}

// TimelineConfig controls snapshot retention.
type TimelineConfig struct {
	MaxSnapshots int `toml:"max_snapshots"`
}

// FilterConfig controls file filtering beyond .gitignore.
type FilterConfig struct {
	ExtraIgnore []string `toml:"extra_ignore"`
}

// EditorConfig controls the editor integration.
type EditorConfig struct {
	Command string `toml:"command"`
}

// Duration wraps time.Duration to support TOML string parsing (e.g. "100ms", "1s").
type Duration struct {
	time.Duration
}

// UnmarshalText implements encoding.TextUnmarshaler for TOML duration strings.
func (d *Duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

// MarshalText implements encoding.TextMarshaler.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.Duration.String()), nil
}

// Default returns a Config with all default values applied.
func Default() Config {
	return Config{
		Watch: WatchConfig{
			PollInterval: Duration{1 * time.Second},
			PollOnly:     false,
			Debounce:     Duration{100 * time.Millisecond},
		},
		Display: DisplayConfig{
			DiffStyle:       "unified",
			Theme:           "dracula",
			WordDiff:        false,
			ContextLines:    3,
			ShowLineNumbers: true,
		},
		Timeline: TimelineConfig{
			MaxSnapshots: 1000,
		},
		Filter: FilterConfig{
			ExtraIgnore: nil,
		},
		Editor: EditorConfig{
			Command: "",
		},
	}
}

// DefaultConfigPath returns the default config file path (~/.config/hark/config.toml).
func DefaultConfigPath() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "hark", "config.toml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "hark", "config.toml")
}

// Load reads configuration from the default config file path, falling back to
// defaults if the file does not exist.
func Load() (Config, error) {
	return LoadFrom(DefaultConfigPath())
}

// LoadFrom reads configuration from the given TOML file path. If the file does
// not exist, defaults are returned with no error.
func LoadFrom(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// Save writes the full config to the given path, creating parent directories
// if needed. It marshals the entire config to preserve all values.
func (c Config) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	if err := enc.Encode(c); err != nil {
		return err
	}

	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// SaveDisplaySettingsAsync persists the config to the default path in a
// goroutine. Errors are logged but do not propagate — UI responsiveness
// takes priority over config persistence.
func (c Config) SaveDisplaySettingsAsync() {
	go func() {
		if err := c.Save(DefaultConfigPath()); err != nil {
			log.Printf("config: save error: %v", err)
		}
	}()
}

// CLIFlags represents the subset of configuration that can be overridden via
// CLI flags. Pointer fields allow distinguishing "not set" (nil) from "set to
// zero value".
type CLIFlags struct {
	PollInterval *time.Duration
	PollOnly     *bool
	Theme        *string
	DiffStyle    *string
	ContextLines *int
	Ignore       []string
	MaxSnapshots *int
}

// Merge applies CLI flag overrides to the config. Flag values that are non-nil
// take precedence over the corresponding config file values.
func (c Config) Merge(flags CLIFlags) Config {
	if flags.PollInterval != nil {
		c.Watch.PollInterval = Duration{*flags.PollInterval}
	}
	if flags.PollOnly != nil {
		c.Watch.PollOnly = *flags.PollOnly
	}
	if flags.Theme != nil {
		c.Display.Theme = *flags.Theme
	}
	if flags.DiffStyle != nil {
		c.Display.DiffStyle = *flags.DiffStyle
	}
	if flags.ContextLines != nil {
		c.Display.ContextLines = *flags.ContextLines
	}
	if flags.MaxSnapshots != nil {
		c.Timeline.MaxSnapshots = *flags.MaxSnapshots
	}
	if len(flags.Ignore) > 0 {
		c.Filter.ExtraIgnore = append(c.Filter.ExtraIgnore, flags.Ignore...)
	}
	return c
}
