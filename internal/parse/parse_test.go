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
	paras, _, err := Parse([]byte(src), opts)
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
			// Attached to the text, so it is a real marker, and unmatched.
			name: "unclosed marker closes at end of line",
			in:   "He paid 5 *3 dollars.",
			want: []doc.Run{
				{Text: "He paid 5 "},
				{Text: "3 dollars.", Bold: true},
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
			name: "bare asterisk between spaces is literal",
			in:   "bongocat * 30m ago",
			want: []doc.Run{{Text: "bongocat * 30m ago"}},
		},
		{
			name: "two bare asterisks on one line",
			in:   "a * b * c",
			want: []doc.Run{{Text: "a * b * c"}},
		},
		{
			name: "bare underscore between spaces is literal",
			in:   "a _ b",
			want: []doc.Run{{Text: "a _ b"}},
		},
		{
			name: "bare run of markers is literal",
			in:   "a ** b *** c",
			want: []doc.Run{{Text: "a ** b *** c"}},
		},
		{
			name: "bare marker separated by tabs is literal",
			in:   "a\t*\tb",
			want: []doc.Run{{Text: "a\t*\tb"}},
		},
		{
			name: "trailing bare marker is literal",
			in:   "5 x 3 *",
			want: []doc.Run{{Text: "5 x 3 *"}},
		},
		{
			name: "bare marker does not disturb real emphasis on the line",
			in:   "a * b *bold* c",
			want: []doc.Run{
				{Text: "a * b "},
				{Text: "bold", Bold: true},
				{Text: " c"},
			},
		},
		{
			name: "attached markers still emphasise",
			in:   "5 *3* dollars",
			want: []doc.Run{
				{Text: "5 "},
				{Text: "3", Bold: true},
				{Text: " dollars"},
			},
		},
		{
			name: "indented lone marker is literal",
			in:   "\t*",
			want: []doc.Run{{Text: "\t*"}},
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

	t.Run("org headings that are not the scene marker are ignored", func(t *testing.T) {
		ignored := []string{
			"* Section One",
			"*** Sub-section",
			"***** Deeply nested",
			"*",
			"****",
			"*\tTab after the asterisk",
		}
		for _, line := range ignored {
			if paras := body(t, opts, line); len(paras) != 0 {
				t.Errorf("%q produced %#v, want it ignored", line, paras)
			}
		}
	})

	t.Run("prose opening with a bold word is kept", func(t *testing.T) {
		kept := []struct {
			line string
			want []doc.Run
		}{
			{"*Bang!* she said", []doc.Run{
				{Text: "Bang!", Bold: true},
				{Text: " she said"},
			}},
			// Three toggles leave bold on when the text begins, and three
			// more turn it back off, so nothing is left open.
			{"***emphatic***", []doc.Run{{Text: "emphatic", Bold: true}}},
		}
		for _, tt := range kept {
			paras := body(t, opts, tt.line)
			if len(paras) != 1 {
				t.Fatalf("%q produced %d paragraphs, want 1", tt.line, len(paras))
			}
			if !reflect.DeepEqual(paras[0].Runs, tt.want) {
				t.Errorf("%q gave %#v\nwant %#v", tt.line, paras[0].Runs, tt.want)
			}
		}
	})

	t.Run("scene changes must be valid org sections", func(t *testing.T) {
		// Trailing text on a scene line is still discarded.
		for _, line := range []string{"**", "** Chapter Two", "**\tTabbed"} {
			paras := body(t, opts, line)
			want := []doc.Paragraph{{Runs: []doc.Run{{Text: "*"}}, Centered: true}}
			if !reflect.DeepEqual(paras, want) {
				t.Errorf("%q gave %#v, want a scene change", line, paras)
			}
		}

		// The right number of asterisks is not enough on its own: without the
		// whitespace it is not a section, so it stays prose.
		for _, line := range []string{"**Bang* she said", "**Bang!** he replied"} {
			paras := body(t, opts, line)
			if len(paras) != 1 || paras[0].Centered {
				t.Errorf("%q gave %#v, want one left-aligned paragraph", line, paras)
			}
		}
	})

	t.Run("the rule follows a custom scene marker", func(t *testing.T) {
		custom := testOpts()
		custom.SceneMarker = "***"

		if paras := body(t, custom, "***"); len(paras) != 1 || !paras[0].Centered {
			t.Errorf("*** should be the scene change: %#v", paras)
		}
		if paras := body(t, custom, "** Now just a heading"); len(paras) != 0 {
			t.Errorf("** should now be an ignored heading: %#v", paras)
		}
	})

	t.Run("indented asterisks stay prose", func(t *testing.T) {
		for _, line := range []string{"\t* Not a heading", "\t*** Nor this"} {
			if paras := body(t, opts, line); len(paras) != 1 || paras[0].Centered {
				t.Errorf("%q gave %#v, want one left-aligned paragraph", line, paras)
			}
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
		src := "#+OPTIONS: toc:nil\nIgnored prose.\n#+begin_story\nKept.\n#+end_story\nAlso ignored.\n"
		paras, _, err := Parse([]byte(src), opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(paras) != 1 || paras[0].Runs[0].Text != "Kept." {
			t.Errorf("got %#v", paras)
		}
	})

	t.Run("markers are case insensitive", func(t *testing.T) {
		src := "#+BEGIN_STORY\nKept.\n#+END_STORY\n"
		paras, _, err := Parse([]byte(src), opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(paras) != 1 {
			t.Errorf("got %#v", paras)
		}
	})

	t.Run("alternate marker spellings", func(t *testing.T) {
		src := "#+story_start\nKept.\n#+story_end\n"
		paras, _, err := Parse([]byte(src), opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(paras) != 1 {
			t.Errorf("got %#v", paras)
		}
	})

	t.Run("indented end marker still ends the story", func(t *testing.T) {
		src := "#+begin_story\nKept.\n\t#+end_story\nIgnored.\n"
		paras, _, err := Parse([]byte(src), opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(paras) != 1 {
			t.Errorf("got %#v, want the end marker to stop conversion", paras)
		}
	})

	t.Run("missing end marker runs to EOF", func(t *testing.T) {
		src := "#+begin_story\nOne.\nTwo.\n"
		paras, _, err := Parse([]byte(src), opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(paras) != 2 {
			t.Errorf("got %d paragraphs, want 2", len(paras))
		}
	})

	t.Run("missing begin marker is an error", func(t *testing.T) {
		_, _, err := Parse([]byte("Just prose.\n"), opts)
		if !errors.Is(err, ErrNoBeginMarker) {
			t.Errorf("got %v, want ErrNoBeginMarker", err)
		}
	})

	t.Run("end before begin is an error", func(t *testing.T) {
		src := "#+end_story\n#+begin_story\nx\n"
		_, _, err := Parse([]byte(src), opts)
		if !errors.Is(err, ErrEndBeforeBegin) {
			t.Errorf("got %v, want ErrEndBeforeBegin", err)
		}
	})

	t.Run("second begin marker is treated as a comment", func(t *testing.T) {
		src := "#+begin_story\nOne.\n#+begin_story\nTwo.\n#+end_story\n"
		paras, _, err := Parse([]byte(src), opts)
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

	paras, _, err := Parse([]byte("#+begin_story\nOne.\n#+end_story\n"), opts)
	if err != nil {
		t.Fatal(err)
	}
	last := paras[len(paras)-1]
	if !last.Centered || last.Runs[0].Text != "END" {
		t.Errorf("got %#v, want a centered END paragraph", last)
	}

	t.Run("appended when the end marker is missing", func(t *testing.T) {
		paras, _, err := Parse([]byte("#+begin_story\nOne.\n"), opts)
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
		paras, _, err := Parse([]byte("#+begin_story\nOne.\n#+end_story\n"), opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(paras) != 1 {
			t.Errorf("got %d paragraphs, want 1", len(paras))
		}
	})
}

func TestUnclosedMarkerWarnings(t *testing.T) {
	opts := testOpts()

	t.Run("reports the line, its number and the marker", func(t *testing.T) {
		src := "#+TITLE: x\npreamble\n#+begin_story\n\tFine *line* here.\n\tHe paid 5 *3 dollars.\n#+end_story\n"

		_, warnings, err := Parse([]byte(src), opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(warnings) != 1 {
			t.Fatalf("got %d warnings, want 1: %#v", len(warnings), warnings)
		}

		got := warnings[0]
		want := Warning{Line: 5, Text: "\tHe paid 5 *3 dollars.", Markers: []string{"*"}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got  %#v\nwant %#v", got, want)
		}
	})

	t.Run("both markers left open", func(t *testing.T) {
		_, warnings, err := Parse([]byte("#+begin_story\nA *b _c\n#+end_story\n"), opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(warnings) != 1 {
			t.Fatalf("got %d warnings, want 1", len(warnings))
		}
		if want := []string{"*", "_"}; !reflect.DeepEqual(warnings[0].Markers, want) {
			t.Errorf("got %v, want %v", warnings[0].Markers, want)
		}
	})

	t.Run("line escape still leaves earlier formatting open", func(t *testing.T) {
		_, warnings, err := Parse([]byte("#+begin_story\n*bold ``` rest\n#+end_story\n"), opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(warnings) != 1 {
			t.Fatalf("got %d warnings, want 1: %#v", len(warnings), warnings)
		}
	})

	t.Run("a bare marker no longer warns", func(t *testing.T) {
		src := "#+begin_story\n\tbongocat * 30m ago\n#+end_story\n"
		_, warnings, err := Parse([]byte(src), opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(warnings) != 0 {
			t.Errorf("got %#v, want no warnings for a bare asterisk", warnings)
		}
	})

	t.Run("an attached marker still warns", func(t *testing.T) {
		// The bare-run rule must not silence a genuinely unclosed marker.
		src := "#+begin_story\n\tHe paid 5 *3 dollars.\n#+end_story\n"
		_, warnings, err := Parse([]byte(src), opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(warnings) != 1 {
			t.Fatalf("got %d warnings, want 1: %#v", len(warnings), warnings)
		}
	})

	quiet := []struct{ name, line string }{
		{"balanced markers", "\tShe said *hello* and _left_."},
		{"bare asterisk", "\tbongocat * 30m ago"},
		{"bare marker run", "\ta ** b"},
		{"escaped literal asterisk", "\tHe paid 5 `* 3 dollars."},
		{"whole line escaped", "\tHe paid ``` 5 * 3 dollars."},
		{"no markers at all", "\tNothing to see here."},
		{"comment line", "# a *stray marker in a comment"},
		{"scene change line", "** a *stray marker on a scene line"},
		{"adjacent toggles balance out", "a **text** b"},
	}
	for _, tt := range quiet {
		t.Run("no warning: "+tt.name, func(t *testing.T) {
			src := "#+begin_story\n" + tt.line + "\n#+end_story\n"
			_, warnings, err := Parse([]byte(src), opts)
			if err != nil {
				t.Fatal(err)
			}
			if len(warnings) != 0 {
				t.Errorf("got %#v, want no warnings", warnings)
			}
		})
	}
}

func TestTitleBlock(t *testing.T) {
	opts := testOpts()
	opts.TitleBlankLines = 2

	t.Run("blanks, title, byline, then a blank", func(t *testing.T) {
		src := "#+title: The Long Goodbye\n#+author: Jane Doe\n#+begin_story\n\tThe door opened.\n#+end_story\n"

		paras, warnings, err := Parse([]byte(src), opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(warnings) != 0 {
			t.Errorf("unexpected warnings: %#v", warnings)
		}

		want := []doc.Paragraph{
			{},
			{},
			{Runs: []doc.Run{{Text: "The Long Goodbye"}}, Centered: true},
			{Runs: []doc.Run{{Text: "by Jane Doe"}}, Centered: true},
			{},
			{Runs: []doc.Run{{Text: "\tThe door opened."}}},
		}
		if !reflect.DeepEqual(paras, want) {
			t.Errorf("got  %#v\nwant %#v", paras, want)
		}
	})

	t.Run("title only", func(t *testing.T) {
		src := "#+title: Solo\n#+begin_story\nx\n#+end_story\n"
		paras, _, err := Parse([]byte(src), opts)
		if err != nil {
			t.Fatal(err)
		}
		want := []doc.Paragraph{
			{},
			{},
			{Runs: []doc.Run{{Text: "Solo"}}, Centered: true},
			{},
			{Runs: []doc.Run{{Text: "x"}}},
		}
		if !reflect.DeepEqual(paras, want) {
			t.Errorf("got  %#v\nwant %#v", paras, want)
		}
	})

	t.Run("author only still gets the leading blanks", func(t *testing.T) {
		src := "#+author: Jane Doe\n#+begin_story\nx\n#+end_story\n"
		paras, _, err := Parse([]byte(src), opts)
		if err != nil {
			t.Fatal(err)
		}
		want := []doc.Paragraph{
			{},
			{},
			{Runs: []doc.Run{{Text: "by Jane Doe"}}, Centered: true},
			{},
			{Runs: []doc.Run{{Text: "x"}}},
		}
		if !reflect.DeepEqual(paras, want) {
			t.Errorf("got  %#v\nwant %#v", paras, want)
		}
	})

	t.Run("neither keyword emits nothing", func(t *testing.T) {
		src := "#+begin_story\nx\n#+end_story\n"
		paras, _, err := Parse([]byte(src), opts)
		if err != nil {
			t.Fatal(err)
		}
		want := []doc.Paragraph{{Runs: []doc.Run{{Text: "x"}}}}
		if !reflect.DeepEqual(paras, want) {
			t.Errorf("got %#v, want no title block", paras)
		}
	})

	t.Run("keywords are case insensitive and the value is trimmed", func(t *testing.T) {
		src := "  #+TITLE:   Spaced Out   \n#+begin_story\nx\n#+end_story\n"
		paras, _, err := Parse([]byte(src), opts)
		if err != nil {
			t.Fatal(err)
		}
		if got := paras[2].Runs[0].Text; got != "Spaced Out" {
			t.Errorf("got %q, want %q", got, "Spaced Out")
		}
	})

	t.Run("an empty value is ignored", func(t *testing.T) {
		src := "#+title:\n#+author:   \n#+begin_story\nx\n#+end_story\n"
		paras, _, err := Parse([]byte(src), opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(paras) != 1 {
			t.Errorf("got %#v, want no title block", paras)
		}
	})

	t.Run("no keywords means no leading blanks either", func(t *testing.T) {
		src := "#+begin_story\nx\n#+end_story\n"
		paras, _, err := Parse([]byte(src), opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(paras) != 1 {
			t.Errorf("got %#v, want the story to start at the top", paras)
		}
	})

	t.Run("keywords after the begin marker are comments", func(t *testing.T) {
		src := "#+begin_story\n#+title: Too Late\nx\n#+end_story\n"
		paras, _, err := Parse([]byte(src), opts)
		if err != nil {
			t.Fatal(err)
		}
		want := []doc.Paragraph{{Runs: []doc.Run{{Text: "x"}}}}
		if !reflect.DeepEqual(paras, want) {
			t.Errorf("got %#v, want the keyword treated as a comment", paras)
		}
	})

	t.Run("only the first occurrence is used", func(t *testing.T) {
		src := "#+title: First\n#+title: Second\n#+begin_story\nx\n#+end_story\n"
		paras, _, err := Parse([]byte(src), opts)
		if err != nil {
			t.Fatal(err)
		}
		if got := paras[2].Runs[0].Text; got != "First" {
			t.Errorf("got %q, want %q", got, "First")
		}
	})

	t.Run("the value is literal, markers and all", func(t *testing.T) {
		src := "#+title: The *Big* Sleep\n#+author: A_B *C\n#+begin_story\nx\n#+end_story\n"

		paras, warnings, err := Parse([]byte(src), opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(warnings) != 0 {
			t.Errorf("literal text should never warn: %#v", warnings)
		}

		want := []doc.Paragraph{
			{},
			{},
			{Runs: []doc.Run{{Text: "The *Big* Sleep"}}, Centered: true},
			{Runs: []doc.Run{{Text: "by A_B *C"}}, Centered: true},
			{},
			{Runs: []doc.Run{{Text: "x"}}},
		}
		if !reflect.DeepEqual(paras, want) {
			t.Errorf("got  %#v\nwant %#v", paras, want)
		}
	})

	t.Run("zero blank lines puts the title at the top", func(t *testing.T) {
		zero := testOpts()
		zero.TitleBlankLines = 0

		paras, _, err := Parse([]byte("#+title: Top\n#+begin_story\nx\n#+end_story\n"), zero)
		if err != nil {
			t.Fatal(err)
		}
		want := []doc.Paragraph{
			{Runs: []doc.Run{{Text: "Top"}}, Centered: true},
			{},
			{Runs: []doc.Run{{Text: "x"}}},
		}
		if !reflect.DeepEqual(paras, want) {
			t.Errorf("got  %#v\nwant %#v", paras, want)
		}
	})
}

func TestCRLFMatchesLF(t *testing.T) {
	lf := "#+begin_story\n\tOne.\n\n\tTwo.\n#+end_story\n"
	crlf := "#+begin_story\r\n\tOne.\r\n\r\n\tTwo.\r\n#+end_story\r\n"

	a, _, err := Parse([]byte(lf), testOpts())
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := Parse([]byte(crlf), testOpts())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Errorf("CRLF differs from LF:\n%#v\n%#v", a, b)
	}
}

func TestTrailingNewlineDoesNotAddParagraph(t *testing.T) {
	withNL, _, err := Parse([]byte("#+begin_story\nOne.\n"), testOpts())
	if err != nil {
		t.Fatal(err)
	}
	withoutNL, _, err := Parse([]byte("#+begin_story\nOne."), testOpts())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(withNL, withoutNL) {
		t.Errorf("trailing newline changed the output: %#v vs %#v", withNL, withoutNL)
	}
}

func TestLeadingBOMIsDropped(t *testing.T) {
	paras, _, err := Parse([]byte("\uFEFF#+begin_story\nOne.\n#+end_story\n"), testOpts())
	if err != nil {
		t.Fatal(err)
	}
	if len(paras) != 1 {
		t.Errorf("got %#v, want the begin marker to be recognised past the BOM", paras)
	}
}
