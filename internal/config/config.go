// Package config holds the resolved option set and the rules for building it
// from defaults, a TOML file, and command-line flags.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Underscore modes.
const (
	Italic    = "italic"
	Underline = "underline"
)

// Line spacing modes.
const (
	Single     = "single"
	OneAndHalf = "1.5"
	Double     = "double"
)

// FileName is the config file searched for in the working directory and then
// the user's home directory.
const FileName = ".org2rtf.toml"

// Config is the fully resolved option set. Every field is settable by flag and
// by config file; the toml tags are the config-file key names.
type Config struct {
	Underscore  string `toml:"underscore"`
	LineEscape  string `toml:"line_escape"`
	WordEscape  string `toml:"word_escape"`
	SceneMarker string `toml:"scene_marker"`
	SceneGlyph  string `toml:"scene_glyph"`
	EndText     string `toml:"end_text"`

	Font        string  `toml:"font"`
	FontSize    float64 `toml:"font_size"`
	LineSpacing string  `toml:"line_spacing"`
	Margin      float64 `toml:"margin"`
	TabWidth    float64 `toml:"tab_width"`

	// TitleBlankLines is a count of empty paragraphs, not a measurement: the
	// distance it produces depends on the font size and line spacing.
	TitleBlankLines int `toml:"title_blank_lines"`
}

// Default returns the built-in defaults, the lowest layer of precedence.
func Default() Config {
	return Config{
		Underscore:  Italic,
		LineEscape:  "```",
		WordEscape:  "`",
		SceneMarker: "**",
		SceneGlyph:  "*",
		EndText:     "END",

		Font:        "Times New Roman",
		FontSize:    12,
		LineSpacing: Single,
		Margin:      1.0,
		TabWidth:    0.5,

		// About a third of the way down page one at the default 12pt single
		// spacing, which is roughly standard manuscript format.
		TitleBlankLines: 12,
	}
}

// Find returns the path of the config file to use, or "" if there isn't one.
// The working directory wins over the home directory.
func Find() string {
	if fileExists(FileName) {
		return FileName
	}
	if home, err := os.UserHomeDir(); err == nil {
		if p := filepath.Join(home, FileName); fileExists(p) {
			return p
		}
	}
	return ""
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

// LoadInto overlays the TOML file at path onto cfg, leaving fields the file
// doesn't mention untouched. An unrecognised key is an error rather than a
// warning: it is nearly always a typo, and ignoring it silently produces output
// that is wrong in a way the user cannot see.
func LoadInto(cfg *Config, path string) error {
	md, err := toml.DecodeFile(path, cfg)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, len(undecoded))
		for i, k := range undecoded {
			keys[i] = k.String()
		}
		return fmt.Errorf("%s: unknown config key(s): %s", path, strings.Join(keys, ", "))
	}
	return nil
}

// Validate reports the first problem with the resolved configuration. It runs
// before any output file is touched.
func (c Config) Validate() error {
	switch c.Underscore {
	case Italic, Underline:
	default:
		return fmt.Errorf("underscore: must be %q or %q, got %q", Italic, Underline, c.Underscore)
	}

	switch c.LineSpacing {
	case Single, OneAndHalf, Double:
	default:
		return fmt.Errorf("line-spacing: must be %q, %q or %q, got %q",
			Single, OneAndHalf, Double, c.LineSpacing)
	}

	if c.LineEscape == "" {
		return fmt.Errorf("line-escape: must not be empty")
	}
	if c.WordEscape == "" {
		return fmt.Errorf("word-escape: must not be empty")
	}
	// The scanner tests for the line escape before the doubled word escape, so
	// if they are the same string one of the two forms becomes unreachable.
	if c.LineEscape == c.WordEscape+c.WordEscape {
		return fmt.Errorf("line-escape %q is ambiguous with word-escape %q doubled",
			c.LineEscape, c.WordEscape)
	}

	if c.SceneMarker == "" {
		return fmt.Errorf("scene-marker: must not be empty")
	}
	if strings.Trim(c.SceneMarker, "*") != "" {
		return fmt.Errorf("scene-marker: only asterisks are allowed, got %q", c.SceneMarker)
	}

	if c.Font == "" {
		return fmt.Errorf("font: must not be empty")
	}
	if c.FontSize <= 0 {
		return fmt.Errorf("font-size: must be positive, got %v", c.FontSize)
	}
	if c.Margin <= 0 {
		return fmt.Errorf("margin: must be positive, got %v", c.Margin)
	}
	if c.TabWidth <= 0 {
		return fmt.Errorf("tab-width: must be positive, got %v", c.TabWidth)
	}
	// Zero is meaningful here: it puts the title block at the top of the page.
	if c.TitleBlankLines < 0 {
		return fmt.Errorf("title-blank-lines: must not be negative, got %v", c.TitleBlankLines)
	}
	return nil
}
