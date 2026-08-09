package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultIsValid(t *testing.T) {
	if err := Default().Validate(); err != nil {
		t.Fatalf("built-in defaults are invalid: %v", err)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{"bad underscore", func(c *Config) { c.Underscore = "bold" }, "underscore"},
		{"bad line spacing", func(c *Config) { c.LineSpacing = "triple" }, "line-spacing"},
		{"empty line escape", func(c *Config) { c.LineEscape = "" }, "line-escape"},
		{"empty word escape", func(c *Config) { c.WordEscape = "" }, "word-escape"},
		{
			name:    "line escape ambiguous with doubled word escape",
			mutate:  func(c *Config) { c.LineEscape = "``" },
			wantErr: "ambiguous",
		},
		{"empty scene marker", func(c *Config) { c.SceneMarker = "" }, "scene-marker"},
		{"non-asterisk scene marker", func(c *Config) { c.SceneMarker = "##" }, "asterisk"},
		{"empty font", func(c *Config) { c.Font = "" }, "font"},
		{"zero font size", func(c *Config) { c.FontSize = 0 }, "font-size"},
		{"negative margin", func(c *Config) { c.Margin = -1 }, "margin"},
		{"zero tab width", func(c *Config) { c.TabWidth = 0 }, "tab-width"},
		{"negative title blank lines", func(c *Config) { c.TitleBlankLines = -1 }, "title-blank-lines"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			tt.mutate(&cfg)

			err := cfg.Validate()
			if err == nil {
				t.Fatalf("got no error, want one mentioning %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("got %q, want it to mention %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAcceptsEmptyEndTextAndGlyph(t *testing.T) {
	cfg := Default()
	cfg.EndText = ""
	cfg.SceneGlyph = ""

	if err := cfg.Validate(); err != nil {
		t.Errorf("empty end text and glyph should be allowed: %v", err)
	}
}

func TestValidateAcceptsZeroTitleBlankLines(t *testing.T) {
	cfg := Default()
	cfg.TitleBlankLines = 0

	if err := cfg.Validate(); err != nil {
		t.Errorf("zero title blank lines should be allowed: %v", err)
	}
}

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadIntoOverlaysOnlyGivenKeys(t *testing.T) {
	path := writeConfig(t, "underscore = \"underline\"\nmargin = 1.5\n")

	cfg := Default()
	if err := LoadInto(&cfg, path); err != nil {
		t.Fatal(err)
	}

	if cfg.Underscore != Underline {
		t.Errorf("underscore = %q, want %q", cfg.Underscore, Underline)
	}
	if cfg.Margin != 1.5 {
		t.Errorf("margin = %v, want 1.5", cfg.Margin)
	}
	if cfg.Font != Default().Font {
		t.Errorf("font = %q, want the default to survive", cfg.Font)
	}
}

func TestLoadIntoRejectsUnknownKeys(t *testing.T) {
	path := writeConfig(t, "underscore = \"italic\"\nunderscroe = \"oops\"\n")

	cfg := Default()
	err := LoadInto(&cfg, path)
	if err == nil {
		t.Fatal("got no error, want one for the misspelled key")
	}
	if !strings.Contains(err.Error(), "underscroe") {
		t.Errorf("got %q, want it to name the unknown key", err)
	}
}

func TestLoadIntoCanClearEndText(t *testing.T) {
	path := writeConfig(t, "end_text = \"\"\n")

	cfg := Default()
	if err := LoadInto(&cfg, path); err != nil {
		t.Fatal(err)
	}
	if cfg.EndText != "" {
		t.Errorf("end_text = %q, want it cleared", cfg.EndText)
	}
}

func TestLoadIntoReportsMissingFile(t *testing.T) {
	cfg := Default()
	if err := LoadInto(&cfg, filepath.Join(t.TempDir(), "absent.toml")); err == nil {
		t.Error("got no error for a missing config file")
	}
}

func TestFindPrefersWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if got := Find(); got != "" {
		t.Fatalf("got %q, want no config file in an empty directory", got)
	}

	if err := os.WriteFile(FileName, []byte("margin = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Find(); got != FileName {
		t.Errorf("got %q, want %q", got, FileName)
	}
}
