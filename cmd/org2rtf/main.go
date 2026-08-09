// Command org2rtf converts .org manuscript files to .rtf.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"org2rtf/internal/config"
	"org2rtf/internal/parse"
	"org2rtf/internal/rtf"
)

const version = "0.1.0"

const usage = `org2rtf converts .org manuscript files to .rtf.

Usage:
  org2rtf [flags] <input.org>

Flags:
  -o, --output PATH      output file (default: input path with a .rtf extension)
  -f, --force            overwrite the output file if it already exists
      --config PATH      use this config file instead of searching for one

      --underscore MODE  what _ means: "italic" or "underline" (default: italic)
      --line-escape STR  rest-of-line literal marker (default: ` + "```" + `)
      --word-escape STR  single-word literal marker (default: ` + "`" + `)
      --scene-marker STR scene-change line prefix, asterisks only (default: **)
      --scene-glyph STR  text of the emitted scene-change paragraph (default: *)
      --end-text STR     final centered paragraph; empty disables it (default: END)

      --font NAME        (default: Times New Roman)
      --font-size PT     (default: 12)
      --line-spacing M   "single", "1.5" or "double" (default: single)
      --margin IN        all four margins, in inches (default: 1)
      --tab-width IN     distance between tab stops, in inches (default: 0.5)

  -h, --help
      --version

Options are resolved as: flag, then config file, then built-in default.
The config file is TOML, searched for as ./` + config.FileName +
	` then ~/` + config.FileName + `.
`

// errReported marks a failure the flag package has already printed.
var errReported = errors.New("reported")

func main() {
	err := run(os.Args[1:])
	if err == nil {
		return
	}
	if errors.Is(err, errReported) {
		os.Exit(2)
	}
	fmt.Fprintf(os.Stderr, "org2rtf: %v\n", err)
	os.Exit(1)
}

func run(args []string) error {
	d := config.Default()

	fs := flag.NewFlagSet("org2rtf", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }

	var (
		outShort   = fs.String("o", "", "")
		outLong    = fs.String("output", "", "")
		forceShort = fs.Bool("f", false, "")
		forceLong  = fs.Bool("force", false, "")
		configPath = fs.String("config", "", "")
		showVer    = fs.Bool("version", false, "")

		underscore  = fs.String("underscore", d.Underscore, "")
		lineEscape  = fs.String("line-escape", d.LineEscape, "")
		wordEscape  = fs.String("word-escape", d.WordEscape, "")
		sceneMarker = fs.String("scene-marker", d.SceneMarker, "")
		sceneGlyph  = fs.String("scene-glyph", d.SceneGlyph, "")
		endText     = fs.String("end-text", d.EndText, "")
		font        = fs.String("font", d.Font, "")
		fontSize    = fs.Float64("font-size", d.FontSize, "")
		lineSpacing = fs.String("line-spacing", d.LineSpacing, "")
		margin      = fs.Float64("margin", d.Margin, "")
		tabWidth    = fs.Float64("tab-width", d.TabWidth, "")
	)

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return errReported
	}

	if *showVer {
		fmt.Println("org2rtf " + version)
		return nil
	}

	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })

	// Precedence: built-in defaults, then the config file, then flags.
	cfg := config.Default()
	path := *configPath
	if path == "" {
		path = config.Find()
	}
	if path != "" {
		if err := config.LoadInto(&cfg, path); err != nil {
			return err
		}
	}
	if set["underscore"] {
		cfg.Underscore = *underscore
	}
	if set["line-escape"] {
		cfg.LineEscape = *lineEscape
	}
	if set["word-escape"] {
		cfg.WordEscape = *wordEscape
	}
	if set["scene-marker"] {
		cfg.SceneMarker = *sceneMarker
	}
	if set["scene-glyph"] {
		cfg.SceneGlyph = *sceneGlyph
	}
	if set["end-text"] {
		cfg.EndText = *endText
	}
	if set["font"] {
		cfg.Font = *font
	}
	if set["font-size"] {
		cfg.FontSize = *fontSize
	}
	if set["line-spacing"] {
		cfg.LineSpacing = *lineSpacing
	}
	if set["margin"] {
		cfg.Margin = *margin
	}
	if set["tab-width"] {
		cfg.TabWidth = *tabWidth
	}

	if err := cfg.Validate(); err != nil {
		return err
	}

	rest := fs.Args()
	switch {
	case len(rest) == 0:
		return errors.New("no input file given (try --help)")
	case len(rest) > 1:
		return fmt.Errorf("expected exactly one input file, got %d", len(rest))
	}
	input := rest[0]

	output := *outLong
	if set["o"] {
		output = *outShort
	}
	if output == "" {
		output = defaultOutput(input)
	}
	force := *forceShort || *forceLong

	if fi, err := os.Stat(input); err != nil {
		return err
	} else if fi.IsDir() {
		return fmt.Errorf("%s is a directory", input)
	}

	same, err := samePath(input, output)
	if err != nil {
		return err
	}
	if same {
		return fmt.Errorf("output %s is the same file as the input", output)
	}

	if _, err := os.Stat(output); err == nil && !force {
		return fmt.Errorf("%s already exists (use --force to overwrite)", output)
	}

	src, err := os.ReadFile(input)
	if err != nil {
		return err
	}

	paras, err := parse.Parse(src, parseOptions(cfg))
	if err != nil {
		return fmt.Errorf("%s: %w", input, err)
	}

	// Rendered in full before anything is written, so a failure can never
	// leave a truncated file behind.
	if err := os.WriteFile(output, rtf.Render(paras, rtf.OptionsFrom(cfg)), 0o644); err != nil {
		return err
	}

	fmt.Printf("wrote %s (%d paragraphs)\n", output, len(paras))
	return nil
}

// parseOptions extracts the parser-relevant options from a resolved config.
func parseOptions(cfg config.Config) parse.Options {
	return parse.Options{
		Underscore:  cfg.Underscore,
		LineEscape:  cfg.LineEscape,
		WordEscape:  cfg.WordEscape,
		SceneMarker: cfg.SceneMarker,
		SceneGlyph:  cfg.SceneGlyph,
		EndText:     cfg.EndText,
	}
}

// defaultOutput replaces the input's extension with .rtf.
func defaultOutput(input string) string {
	return strings.TrimSuffix(input, filepath.Ext(input)) + ".rtf"
}

// samePath reports whether two paths resolve to the same location, guarding
// against an output path that would overwrite the source manuscript.
func samePath(a, b string) (bool, error) {
	absA, err := filepath.Abs(a)
	if err != nil {
		return false, err
	}
	absB, err := filepath.Abs(b)
	if err != nil {
		return false, err
	}
	return filepath.Clean(absA) == filepath.Clean(absB), nil
}
