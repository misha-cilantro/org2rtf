package rtf

import (
	"strings"
	"testing"

	"org2rtf/internal/config"
	"org2rtf/internal/doc"
)

func testOptions() Options {
	return OptionsFrom(config.Default())
}

func escapeOf(text string) string {
	var b strings.Builder
	writeEscaped(&b, text)
	return b.String()
}

func TestEscaping(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain ascii", "Hello, world.", "Hello, world."},
		{"backslash", `a\b`, `a\\b`},
		{"braces", "a{b}c", `a\{b\}c`},
		{"single tab", "a\tb", `a\tab b`},
		{"consecutive tabs", "\t\t\t", `\tab \tab \tab `},
		{"em dash", "a\u2014b", `a\u8212 ?b`},
		{"curly quotes", "\u201chi\u201d", `\u8220 ?hi\u8221 ?`},
		{"accented letter", "caf\u00e9", `caf\u233 ?`},
		{"astral plane emoji", "\U0001F600", `\u-10179 ?\u-8704 ?`},
		{
			// Regression: the character after an escape must survive.
			name: "text immediately after an escape",
			in:   "caf\u00e9, x",
			want: `caf\u233 ?, x`,
		},
		{"control character", "a\x0cb", `a\'0cb`},
		{"backtick is not special to rtf", "a`b", "a`b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := escapeOf(tt.in); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTabCountIsExact(t *testing.T) {
	// The core promise of the tool: N tabs in, N tabs out.
	for n := 1; n <= 5; n++ {
		in := strings.Repeat("\t", n)
		got := strings.Count(escapeOf(in), `\tab `)
		if got != n {
			t.Errorf("%d tabs produced %d \\tab control words", n, got)
		}
	}
}

func TestHeader(t *testing.T) {
	out := string(Render(nil, testOptions()))

	for _, want := range []string{
		`{\rtf1\ansi`,
		`\uc1`,
		`Times New Roman;`,
		`\deftab720`, // 0.5in
		`\margl1440`, // 1in
	} {
		if !strings.Contains(out, want) {
			t.Errorf("header missing %q:\n%s", want, out)
		}
	}
	if !strings.HasSuffix(out, "}\n") {
		t.Errorf("document not closed:\n%s", out)
	}
}

func TestHeaderRespectsOptions(t *testing.T) {
	opts := testOptions()
	opts.Font = "Courier New"
	opts.FontSize = 11.5
	opts.Margin = 1.25
	opts.TabWidth = 0.25
	opts.LineSpacing = config.Double

	out := string(Render([]doc.Paragraph{{Runs: []doc.Run{{Text: "x"}}}}, opts))

	for _, want := range []string{
		`Courier New;`,
		`\fs23`, // 11.5pt in half-points
		`\margl1800`,
		`\deftab360`,
		`\sl480\slmult1`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestParagraphs(t *testing.T) {
	paras := []doc.Paragraph{
		{Runs: []doc.Run{
			{Text: "plain "},
			{Text: "bold", Bold: true},
			{Text: "italic", Italic: true},
			{Text: "under", Underline: true},
			{Text: "both", Bold: true, Italic: true},
		}},
		{},
		{Runs: []doc.Run{{Text: "*"}}, Centered: true},
	}

	out := string(Render(paras, testOptions()))

	for _, want := range []string{
		`{\b bold}`,
		`{\i italic}`,
		`{\ul under}`,
		`{\b\i both}`,
		`\pard\plain\qc`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}

	if got := strings.Count(out, `\par`+"\n"); got != len(paras) {
		t.Errorf("got %d paragraph terminators, want %d:\n%s", got, len(paras), out)
	}
}

func TestFontNameIsEscaped(t *testing.T) {
	opts := testOptions()
	opts.Font = `Odd{Font}`

	if out := string(Render(nil, opts)); !strings.Contains(out, `Odd\{Font\}`) {
		t.Errorf("font name not escaped:\n%s", out)
	}
}
