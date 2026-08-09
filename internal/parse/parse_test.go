package parse

import (
	"errors"
	"reflect"
	"testing"

	"org2rtf/internal/doc"
)

// testOpts are the defaults with the trailing END paragraph disabled, so tests
// assert on the body alone.
func testOpts() Options {
	return Options{
		Underscore:  "italic",
		LineEscape:  "```",
		WordEscape:  "`",
		SceneMarker: "**",
		SceneGlyph:  "*",
		EndText:     "",
	}
}

// body wraps lines in story markers and returns the resulting paragraphs.
func body(t *testing.T, opts Options, lines string) []doc.Paragraph {
	t.Helper()
	src := "#+begin_story\n" + lines + "\n#+end_story\n"
	paras, err := Parse([]byte(src), opts)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return paras
}

// runsOf returns the runs of the only paragraph produced by a single line.
func runsOf(t *testing.T, opts Options, line string) []doc.Run {
	t.Helper()
	paras := body(t, opts, line)
	if len(paras) != 1 {
		t.Fatalf("got %d paragraphs, want 1", len(paras))
	}
	return paras[0].Runs
}

func TestInlineFormatting(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []doc.Run
	}{
		{
			name: "plain text is one run",
			in:   "Nothing to see here.",
			want: []doc.Run{{Text: "Nothing to see here."}},
		},
		{
			name: "asterisks bold",
			in:   "She said *hello* to him.",
			want: []doc.Run{
				{Text: "She said "},
				{Text: "hello", Bold: true},
				{Text: " to him."},
			},
		},
		{
			name: "underscores italicise",
			in:   "She _left_ quietly.",
			want: []doc.Run{
				{Text: "She "},
				{Text: "left", Italic: true},
				{Text: " quietly."},
			},
		},
		{
			name: "markers fire word-internally",
			in:   "The file_name_here is fine.",
			want: []doc.Run{
				{Text: "The file"},
				{Text: "name", Italic: true},
				{Text: "here is fine."},
			},
		},
		{
			name: "toggles overlap independently",
			in:   "*bold _both* italic_",
			want: []doc.Run{
				{Text: "bold ", Bold: true},
				{Text: "both", Bold: true, Italic: true},
				{Text: " italic", Italic: true},
			},
		},
		{
			name: "unclosed marker closes at end of line",
			in:   "He paid 5 * 3 dollars.",
			want: []doc.Run{
				{Text: "He paid 5 "},
				{Text: " 3 dollars.", Bold: true},
			},
		},
		{
			name: "word escape makes a word literal",
			in:   "`this_word_example",
			want: []doc.Run{{Text: "this_word_example"}},
		},
		{
			name: "doubled word escape is a literal backtick",
			in:   "Press `` to continue",
			want: []doc.Run{{Text: "Press ` to continue"}},
		},
		{
			// The whole remainder is literal, including the space that happens
			// to follow the marker. Nothing is trimmed.
			name: "line escape makes the rest literal",
			in:   "Notes: ``` see *this* and _that_",
			want: []doc.Run{{Text: "Notes:  see *this* and _that_"}},
		},
		{
			name: "four backticks are line escape then literal backtick",
			in:   "a ````b*c* d",
			want: []doc.Run{{Text: "a `b*c* d"}},
		},
		{
			name: "lone backtick before whitespace is literal",
			in:   "foo ` bar",
			want: []doc.Run{{Text: "foo ` bar"}},
		},
		{
			name: "trailing lone backtick is literal",
			in:   "foo `",
			want: []doc.Run{{Text: "foo `"}},
		},
		{
			name: "word escape suppresses markers but not open formatting",
			in:   "*Hello `this_word there*",
			want: []doc.Run{{Text: "Hello this_word there", Bold: true}},
		},
		{
			name: "word escape ends at a tab",
			in:   "`a_b\tc_d",
			want: []doc.Run{
				{Text: "a_b\tc"},
				{Text: "d", Italic: true},
			},
		},
		{
			name: "slash and plus are literal",
			in:   "and/or on 12/25/2024 in C++",
			want: []doc.Run{{Text: "and/or on 12/25/2024 in C++"}},
		},
		{
			name: "tabs are preserved exactly",
			in:   "\t\t\tThree tabs\t\tthen two.",
			want: []doc.Run{{Text: "\t\t\tThree tabs\t\tthen two."}},
		},
		{
			name: "tab adjacent to a marker",
			in:   "\t*bold*\t\t_it_\t",
			want: []doc.Run{
				{Text: "\t"},
				{Text: "bold", Bold: true},
				{Text: "\t\t"},
				{Text: "it", Italic: true},
				{Text: "\t"},
			},
		},
		{
			// Not at column 0, so this is prose rather than a scene change.
			name: "adjacent markers produce no empty run",
			in:   "a **text** b",
			want: []doc.Run{
				{Text: "a "},
				{Text: "text"},
				{Text: " b"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runsOf(t, testOpts(), tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got  %#v\nwant %#v", got, tt.want)
			}
		})
	}
}

func TestUnderlineMode(t *testing.T) {
	opts := testOpts()
	opts.Underscore = "underline"

	got := runsOf(t, opts, "She _left_ quietly.")
	want := []doc.Run{
		{Text: "She "},
		{Text: "left", Underline: true},
		{Text: " quietly."},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %#v\nwant %#v", got, want)
	}
}

func TestBlankLinesBecomeEmptyParagraphs(t *testing.T) {
	paras := body(t, testOpts(), "One.\n\n\nTwo.")

	if len(paras) != 4 {
		t.Fatalf("got %d paragraphs, want 4", len(paras))
	}
	if len(paras[1].Runs) != 0 || len(paras[2].Runs) != 0 {
		t.Errorf("consecutive blank lines were collapsed: %#v", paras)
	}
}

func TestLineClassification(t *testing.T) {
	opts := testOpts()

	t.Run("comment at column zero is skipped", func(t *testing.T) {
		if paras := body(t, opts, "# nope"); len(paras) != 0 {
			t.Errorf("got %#v, want no paragraphs", paras)
		}
	})

	t.Run("indented comment is skipped", func(t *testing.T) {
		if paras := body(t, opts, "\t\t# nope"); len(paras) != 0 {
			t.Errorf("got %#v, want no paragraphs", paras)
		}
	})

	t.Run("scene marker emits a centered glyph", func(t *testing.T) {
		paras := body(t, opts, "**")
		want := []doc.Paragraph{{Runs: []doc.Run{{Text: "*"}}, Centered: true}}
		if !reflect.DeepEqual(paras, want) {
			t.Errorf("got %#v\nwant %#v", paras, want)
		}
	})

	t.Run("scene marker discards trailing text", func(t *testing.T) {
		paras := body(t, opts, "** Chapter Two")
		want := []doc.Paragraph{{Runs: []doc.Run{{Text: "*"}}, Centered: true}}
		if !reflect.DeepEqual(paras, want) {
			t.Errorf("got %#v\nwant %#v", paras, want)
		}
	})

	t.Run("indented scene marker stays prose", func(t *testing.T) {
		paras := body(t, opts, "\t**Bang* she said")
		if len(paras) != 1 || paras[0].Centered {
			t.Fatalf("got %#v, want one left-aligned paragraph", paras)
		}
	})

	t.Run("scene glyph is not scanned for markers", func(t *testing.T) {
		// The default glyph is "*", which would toggle bold and vanish if the
		// inline scanner were applied to it.
		paras := body(t, opts, "**")
		if len(paras[0].Runs) != 1 || paras[0].Runs[0].Text != "*" {
			t.Errorf("got %#v, want a literal asterisk", paras[0].Runs)
		}
	})
}

func TestStoryBoundaries(t *testing.T) {
	opts := testOpts()

	t.Run("text before the begin marker is ignored", func(t *testing.T) {
		src := "#+TITLE: x\nIgnored prose.\n#+begin_story\nKept.\n#+end_story\nAlso ignored.\n"
		paras, err := Parse([]byte(src), opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(paras) != 1 || paras[0].Runs[0].Text != "Kept." {
			t.Errorf("got %#v", paras)
		}
	})

	t.Run("markers are case insensitive", func(t *testing.T) {
		src := "#+BEGIN_STORY\nKept.\n#+END_STORY\n"
		paras, err := Parse([]byte(src), opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(paras) != 1 {
			t.Errorf("got %#v", paras)
		}
	})

	t.Run("alternate marker spellings", func(t *testing.T) {
		src := "#+story_start\nKept.\n#+story_end\n"
		paras, err := Parse([]byte(src), opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(paras) != 1 {
			t.Errorf("got %#v", paras)
		}
	})

	t.Run("indented end marker still ends the story", func(t *testing.T) {
		src := "#+begin_story\nKept.\n\t#+end_story\nIgnored.\n"
		paras, err := Parse([]byte(src), opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(paras) != 1 {
			t.Errorf("got %#v, want the end marker to stop conversion", paras)
		}
	})

	t.Run("missing end marker runs to EOF", func(t *testing.T) {
		src := "#+begin_story\nOne.\nTwo.\n"
		paras, err := Parse([]byte(src), opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(paras) != 2 {
			t.Errorf("got %d paragraphs, want 2", len(paras))
		}
	})

	t.Run("missing begin marker is an error", func(t *testing.T) {
		_, err := Parse([]byte("Just prose.\n"), opts)
		if !errors.Is(err, ErrNoBeginMarker) {
			t.Errorf("got %v, want ErrNoBeginMarker", err)
		}
	})

	t.Run("end before begin is an error", func(t *testing.T) {
		src := "#+end_story\n#+begin_story\nx\n"
		_, err := Parse([]byte(src), opts)
		if !errors.Is(err, ErrEndBeforeBegin) {
			t.Errorf("got %v, want ErrEndBeforeBegin", err)
		}
	})

	t.Run("second begin marker is treated as a comment", func(t *testing.T) {
		src := "#+begin_story\nOne.\n#+begin_story\nTwo.\n#+end_story\n"
		paras, err := Parse([]byte(src), opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(paras) != 2 {
			t.Errorf("got %d paragraphs, want 2 (the repeat marker is a comment)", len(paras))
		}
	})
}

func TestEndText(t *testing.T) {
	opts := testOpts()
	opts.EndText = "END"

	paras, err := Parse([]byte("#+begin_story\nOne.\n#+end_story\n"), opts)
	if err != nil {
		t.Fatal(err)
	}
	last := paras[len(paras)-1]
	if !last.Centered || last.Runs[0].Text != "END" {
		t.Errorf("got %#v, want a centered END paragraph", last)
	}

	t.Run("appended when the end marker is missing", func(t *testing.T) {
		paras, err := Parse([]byte("#+begin_story\nOne.\n"), opts)
		if err != nil {
			t.Fatal(err)
		}
		last := paras[len(paras)-1]
		if !last.Centered || last.Runs[0].Text != "END" {
			t.Errorf("got %#v, want a centered END paragraph", last)
		}
	})

	t.Run("empty end text emits nothing", func(t *testing.T) {
		opts.EndText = ""
		paras, err := Parse([]byte("#+begin_story\nOne.\n#+end_story\n"), opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(paras) != 1 {
			t.Errorf("got %d paragraphs, want 1", len(paras))
		}
	})
}

func TestCRLFMatchesLF(t *testing.T) {
	lf := "#+begin_story\n\tOne.\n\n\tTwo.\n#+end_story\n"
	crlf := "#+begin_story\r\n\tOne.\r\n\r\n\tTwo.\r\n#+end_story\r\n"

	a, err := Parse([]byte(lf), testOpts())
	if err != nil {
		t.Fatal(err)
	}
	b, err := Parse([]byte(crlf), testOpts())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Errorf("CRLF differs from LF:\n%#v\n%#v", a, b)
	}
}

func TestTrailingNewlineDoesNotAddParagraph(t *testing.T) {
	withNL, err := Parse([]byte("#+begin_story\nOne.\n"), testOpts())
	if err != nil {
		t.Fatal(err)
	}
	withoutNL, err := Parse([]byte("#+begin_story\nOne."), testOpts())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(withNL, withoutNL) {
		t.Errorf("trailing newline changed the output: %#v vs %#v", withNL, withoutNL)
	}
}

func TestLeadingBOMIsDropped(t *testing.T) {
	paras, err := Parse([]byte("\uFEFF#+begin_story\nOne.\n#+end_story\n"), testOpts())
	if err != nil {
		t.Fatal(err)
	}
	if len(paras) != 1 {
		t.Errorf("got %#v, want the begin marker to be recognised past the BOM", paras)
	}
}
