package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"org2rtf/internal/parse"
)

const sampleOrg = "#+begin_story\n\tOne *bold* line.\n#+end_story\n"

// newStory writes a sample .org file into an isolated working directory and
// returns its path. Chdir keeps a stray .org2rtf.toml elsewhere on the machine
// from leaking into the test.
func newStory(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(sampleOrg), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPrintWarnings(t *testing.T) {
	t.Run("one marker", func(t *testing.T) {
		var buf bytes.Buffer
		printWarnings(&buf, []parse.Warning{
			{Line: 12, Text: "\tHe paid 5 *3 dollars.", Markers: []string{"*"}},
		}, "`")

		out := buf.String()
		for _, want := range []string{
			"line 12",
			"unclosed * marker",
			"He paid 5 *3 dollars.",
			"write `* to keep it literal",
			"add the matching * to close it",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("two markers read as plural", func(t *testing.T) {
		var buf bytes.Buffer
		printWarnings(&buf, []parse.Warning{
			{Line: 3, Text: "A *b _c", Markers: []string{"*", "_"}},
		}, "`")

		out := buf.String()
		for _, want := range []string{
			"unclosed * and _ markers",
			"write `* or `_ to keep them literal",
			"add the matching * and _ to close them",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("uses the configured word escape", func(t *testing.T) {
		var buf bytes.Buffer
		printWarnings(&buf, []parse.Warning{
			{Line: 1, Text: "x *y", Markers: []string{"*"}},
		}, "~")

		if out := buf.String(); !strings.Contains(out, "write ~* to keep it literal") {
			t.Errorf("hint did not use the configured escape:\n%s", out)
		}
	})

	t.Run("nothing printed when there are no warnings", func(t *testing.T) {
		var buf bytes.Buffer
		printWarnings(&buf, nil, "`")
		if buf.Len() != 0 {
			t.Errorf("got %q, want no output", buf.String())
		}
	})
}

func TestDefaultOutputPath(t *testing.T) {
	tests := []struct{ in, want string }{
		{"story.org", "story.rtf"},
		{"story", "story.rtf"},
		{"story.tar.org", "story.tar.rtf"},
		{filepath.Join("sub", "story.org"), filepath.Join("sub", "story.rtf")},
	}

	for _, tt := range tests {
		if got := defaultOutput(tt.in); got != tt.want {
			t.Errorf("defaultOutput(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestRunWritesOutput(t *testing.T) {
	in := newStory(t, "story.org")

	if err := run([]string{in}); err != nil {
		t.Fatalf("run: %v", err)
	}

	out := defaultOutput(in)
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("expected %s to exist: %v", out, err)
	}
	if !strings.HasPrefix(string(data), `{\rtf1`) {
		t.Errorf("output is not RTF:\n%s", data)
	}
	if !strings.Contains(string(data), `{\b bold}`) {
		t.Errorf("output missing bold run:\n%s", data)
	}
}

func TestRunRefusesToOverwrite(t *testing.T) {
	in := newStory(t, "story.org")
	out := defaultOutput(in)

	if err := os.WriteFile(out, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := run([]string{in})
	if err == nil {
		t.Fatal("got no error, want a refusal to overwrite")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("got %q, want it to mention --force", err)
	}

	data, _ := os.ReadFile(out)
	if string(data) != "existing" {
		t.Error("the existing file was modified despite the refusal")
	}

	if err := run([]string{"--force", in}); err != nil {
		t.Fatalf("run with --force: %v", err)
	}
	if data, _ := os.ReadFile(out); string(data) == "existing" {
		t.Error("--force did not overwrite the file")
	}
}

func TestRunRejectsOutputEqualToInput(t *testing.T) {
	in := newStory(t, "story.org")

	err := run([]string{"-o", in, in})
	if err == nil {
		t.Fatal("got no error, want a refusal to overwrite the source")
	}

	data, _ := os.ReadFile(in)
	if string(data) != sampleOrg {
		t.Error("the source file was modified")
	}
}

func TestRunErrors(t *testing.T) {
	t.Run("no input file", func(t *testing.T) {
		if err := run(nil); err == nil {
			t.Error("got no error for a missing input argument")
		}
	})

	t.Run("too many input files", func(t *testing.T) {
		if err := run([]string{"a.org", "b.org"}); err == nil {
			t.Error("got no error for two input arguments")
		}
	})

	t.Run("input does not exist", func(t *testing.T) {
		t.Chdir(t.TempDir())
		if err := run([]string{"absent.org"}); err == nil {
			t.Error("got no error for a missing input file")
		}
	})

	t.Run("invalid option value", func(t *testing.T) {
		in := newStory(t, "story.org")
		err := run([]string{"--underscore", "bold", in})
		if err == nil {
			t.Fatal("got no error for an invalid --underscore value")
		}
		if _, statErr := os.Stat(defaultOutput(in)); statErr == nil {
			t.Error("an output file was written despite the invalid option")
		}
	})

	t.Run("missing begin marker", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		in := filepath.Join(dir, "story.org")
		if err := os.WriteFile(in, []byte("Just prose.\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		if err := run([]string{in}); err == nil {
			t.Fatal("got no error for a file with no begin marker")
		}
		if _, err := os.Stat(defaultOutput(in)); err == nil {
			t.Error("an output file was written despite the parse error")
		}
	})
}

func TestPrecedence(t *testing.T) {
	in := newStory(t, "story.org")

	// A config file in the working directory is picked up automatically.
	if err := os.WriteFile(".org2rtf.toml", []byte("underscore = \"underline\"\nfont = \"Courier New\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := run([]string{in}); err != nil {
		t.Fatalf("run: %v", err)
	}
	data, err := os.ReadFile(defaultOutput(in))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Courier New;") {
		t.Errorf("config file font was not applied:\n%s", data)
	}

	// A flag beats the config file.
	if err := run([]string{"--font", "Georgia", "--force", in}); err != nil {
		t.Fatalf("run: %v", err)
	}
	data, err = os.ReadFile(defaultOutput(in))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Georgia;") {
		t.Errorf("flag did not override the config file:\n%s", data)
	}
}

func TestUnknownConfigKeyIsReported(t *testing.T) {
	in := newStory(t, "story.org")

	if err := os.WriteFile(".org2rtf.toml", []byte("fnot = \"Courier New\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := run([]string{in})
	if err == nil {
		t.Fatal("got no error for an unknown config key")
	}
	if !strings.Contains(err.Error(), "fnot") {
		t.Errorf("got %q, want it to name the unknown key", err)
	}
}
