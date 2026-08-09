package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"org2rtf/internal/config"
	"org2rtf/internal/parse"
	"org2rtf/internal/rtf"
)

var update = flag.Bool("update", false, "rewrite the golden .rtf files from the .org sources")

var goldenCases = []struct {
	name   string
	adjust func(*config.Config)
}{
	{name: "manuscript"},
	{name: "noend"},
	{
		name:   "underline",
		adjust: func(c *config.Config) { c.Underscore = config.Underline },
	},
	{
		name: "options",
		adjust: func(c *config.Config) {
			c.Font = "Courier New"
			c.FontSize = 11
			c.LineSpacing = config.Double
			c.Margin = 1.25
			c.TabWidth = 0.25
			c.SceneGlyph = "# # #"
			c.EndText = ""
		},
	},
}

func TestGolden(t *testing.T) {
	for _, tc := range goldenCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.Default()
			if tc.adjust != nil {
				tc.adjust(&cfg)
			}
			if err := cfg.Validate(); err != nil {
				t.Fatalf("test config is invalid: %v", err)
			}

			src, err := os.ReadFile(filepath.Join("testdata", tc.name+".org"))
			if err != nil {
				t.Fatal(err)
			}

			paras, err := parse.Parse(src, parseOptions(cfg))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			got := rtf.Render(paras, rtf.OptionsFrom(cfg))

			goldenPath := filepath.Join("testdata", tc.name+".rtf")
			if *update {
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatal(err)
				}
				t.Logf("updated %s", goldenPath)
				return
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("%v (run: go test ./cmd/org2rtf -update)", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("output differs from %s\n--- got ---\n%s\n--- want ---\n%s",
					goldenPath, got, want)
			}
		})
	}
}
