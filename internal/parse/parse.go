// Package parse turns .org source text into the intermediate document
// representation. It never adds, removes, or normalises anything the spec does
// not call for; in particular tabs pass through untouched.
package parse

import (
	"errors"
	"strings"

	"org2rtf/internal/doc"
)

// Errors returned for malformed story boundaries.
var (
	ErrNoBeginMarker  = errors.New("no #+begin_story or #+story_start line found")
	ErrEndBeforeBegin = errors.New("story end marker appears before the begin marker")
)

var (
	beginMarkers = []string{"#+begin_story", "#+story_start"}
	endMarkers   = []string{"#+end_story", "#+story_end"}
)

// Org keywords read from the preamble to build the title block.
const (
	titleKeyword  = "#+title:"
	authorKeyword = "#+author:"

	// bylinePrefix turns the author name into a byline.
	bylinePrefix = "by "
)

// Warning reports a line that ended with formatting still open, which the
// parser closed for it. That is legal but usually means a literal asterisk or
// underscore went unescaped, so it is worth surfacing.
type Warning struct {
	Line    int      // 1-based line number in the source file
	Text    string   // the offending source line, verbatim
	Markers []string // the markers left open, in the order "*", "_"
}

// Options are the parser-relevant subset of the configuration.
type Options struct {
	Underscore  string // "italic" or "underline"
	LineEscape  string
	WordEscape  string
	SceneMarker string
	SceneGlyph  string
	EndText     string

	// TitleBlankLines is how many empty paragraphs precede the title block,
	// pushing it down the first page.
	TitleBlankLines int
}

// Parse converts src into paragraphs. Conversion begins after the story begin
// marker and stops at the end marker, or at end of file if there isn't one.
// Any lines that ended with formatting still open are reported as warnings; the
// formatting is closed regardless, so warnings never block conversion.
func Parse(src []byte, opts Options) ([]doc.Paragraph, []Warning, error) {
	lines := splitLines(string(src))

	beginIdx, endIdx := -1, -1
	for i, line := range lines {
		trimmed := trimLeadingSpace(line)
		if beginIdx < 0 && hasMarker(trimmed, beginMarkers) {
			beginIdx = i
		}
		if endIdx < 0 && hasMarker(trimmed, endMarkers) {
			endIdx = i
		}
	}
	if beginIdx < 0 {
		return nil, nil, ErrNoBeginMarker
	}
	if endIdx >= 0 && endIdx < beginIdx {
		return nil, nil, ErrEndBeforeBegin
	}

	// The title block is built from keywords in the preamble, the part of the
	// file before the begin marker. A #+title: after the marker is just a
	// comment, like any other line starting with "#".
	paras, warnings := titleBlock(lines[:beginIdx], opts)

	for offset, line := range lines[beginIdx+1:] {
		trimmed := trimLeadingSpace(line)

		// Structure is driven entirely by org section headings: a run of
		// asterisks at column 0 followed by whitespace or nothing. A heading
		// whose depth matches the scene marker is a scene change, and any
		// other heading is ignored. The rule is column-0 strict, unlike
		// comments: an indented "**" is plausibly bold text opening a
		// tab-indented paragraph.
		asterisks := leadingAsterisks(line)
		heading := asterisks > 0 && isHeading(line, asterisks)

		switch {
		// The end marker is checked first so an indented one still terminates
		// the story instead of being swallowed by the comment rule below.
		case hasMarker(trimmed, endMarkers):
			return append(paras, endParagraph(opts)...), warnings, nil

		// Matched on the depth of the heading rather than as a prefix, so with
		// a "**" marker a "***" heading is not mistaken for a scene change.
		case heading && asterisks == len(opts.SceneMarker):
			paras = append(paras, literalParagraph(opts.SceneGlyph))

		// A section heading at any other depth. Ignored so sections can be
		// used freely to structure the source.
		case heading:

		case strings.HasPrefix(trimmed, "#"):
			// Comment. Leading whitespace is tolerated so comments can be
			// indented to match the prose around them.

		default:
			runs, openBold, openEmph := parseLine(line, opts)
			paras = append(paras, doc.Paragraph{Runs: runs})

			if markers := openMarkers(openBold, openEmph); markers != nil {
				warnings = append(warnings, Warning{
					// +2 because beginIdx is 0-based and the body starts on
					// the line after it.
					Line:    beginIdx + 2 + offset,
					Text:    line,
					Markers: markers,
				})
			}
		}
	}
	return append(paras, endParagraph(opts)...), warnings, nil
}

// titleBlock builds the centered title and byline from the preamble's #+title:
// and #+author: keywords. The block is preceded by title_blank_lines empty
// paragraphs, which push it down the first page, and followed by one more. If
// neither keyword is present nothing is emitted and the story starts at the top.
func titleBlock(preamble []string, opts Options) ([]doc.Paragraph, []Warning) {
	var (
		block    []doc.Paragraph
		warnings []Warning
	)

	add := func(text string, keywordLine int, sourceLine string) {
		runs, openBold, openEmph := parseLine(text, opts)
		block = append(block, doc.Paragraph{Runs: runs, Centered: true})

		if markers := openMarkers(openBold, openEmph); markers != nil {
			warnings = append(warnings, Warning{
				Line:    keywordLine + 1,
				Text:    sourceLine,
				Markers: markers,
			})
		}
	}

	if title, at := keywordValue(preamble, titleKeyword); title != "" {
		add(title, at, preamble[at])
	}
	if author, at := keywordValue(preamble, authorKeyword); author != "" {
		add(bylinePrefix+author, at, preamble[at])
	}

	if len(block) == 0 {
		return nil, nil
	}

	paras := make([]doc.Paragraph, opts.TitleBlankLines, opts.TitleBlankLines+len(block)+1)
	paras = append(paras, block...)
	return append(paras, doc.Paragraph{}), warnings
}

// keywordValue returns the value of the first occurrence of an org keyword and
// the index of the line it was found on, or ("", -1) if it is absent. Leading
// whitespace before the keyword is tolerated, and the value is trimmed: the
// space after the colon is keyword syntax, not part of the title.
func keywordValue(lines []string, keyword string) (string, int) {
	for i, line := range lines {
		trimmed := trimLeadingSpace(line)
		if len(trimmed) >= len(keyword) && strings.EqualFold(trimmed[:len(keyword)], keyword) {
			return strings.TrimSpace(trimmed[len(keyword):]), i
		}
	}
	return "", -1
}

// openMarkers names the toggles left open at end of line, or nil if none were.
func openMarkers(bold, emph bool) []string {
	var markers []string
	if bold {
		markers = append(markers, "*")
	}
	if emph {
		markers = append(markers, "_")
	}
	return markers
}

func endParagraph(opts Options) []doc.Paragraph {
	if opts.EndText == "" {
		return nil
	}
	return []doc.Paragraph{literalParagraph(opts.EndText)}
}

// literalParagraph builds a centered paragraph whose text is taken verbatim.
// The scene glyph and end text are not scanned for markers; the default glyph
// is "*", which would otherwise toggle bold and emit nothing.
func literalParagraph(text string) doc.Paragraph {
	p := doc.Paragraph{Centered: true}
	if text != "" {
		p.Runs = []doc.Run{{Text: text}}
	}
	return p
}

// splitLines splits on \n and strips one trailing \r from each line, so CRLF
// sources behave identically to LF ones. A trailing newline at end of file does
// not produce an extra empty line. A leading BOM is dropped: it is an encoding
// artifact rather than text, and would otherwise be emitted as a character.
func splitLines(s string) []string {
	s = strings.TrimPrefix(s, "\uFEFF")
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	if last := len(lines) - 1; lines[last] == "" {
		lines = lines[:last]
	}
	for i, line := range lines {
		lines[i] = strings.TrimSuffix(line, "\r")
	}
	return lines
}

func trimLeadingSpace(s string) string {
	return strings.TrimLeft(s, " \t")
}

// leadingAsterisks counts the asterisks starting at column 0.
func leadingAsterisks(line string) int {
	n := 0
	for n < len(line) && line[n] == '*' {
		n++
	}
	return n
}

// isHeading reports whether an asterisk run of length n has the shape of an org
// section heading: followed by whitespace, or making up the whole line. Prose
// that merely opens with markers, such as "*Bang!* she said" or "**Bang!** he
// replied", does not qualify and is kept as text.
func isHeading(line string, n int) bool {
	return n == len(line) || isSpaceOrTab(line[n])
}

// hasMarker reports whether line begins with one of the given org keywords,
// matched case-insensitively.
func hasMarker(line string, markers []string) bool {
	for _, m := range markers {
		if len(line) >= len(m) && strings.EqualFold(line[:len(m)], m) {
			return true
		}
	}
	return false
}

// parseLine scans one line into styled runs. Bold and emphasis are independent
// toggles rather than a nesting stack, so overlapping ranges are well defined
// and anything still open simply closes at end of line. The returned flags say
// which toggles were still open when the line ended.
func parseLine(line string, opts Options) (runs []doc.Run, openBold, openEmph bool) {
	var (
		buf  strings.Builder
		bold bool
		emph bool
	)

	flush := func() {
		if buf.Len() == 0 {
			return
		}
		run := doc.Run{Text: buf.String(), Bold: bold}
		if emph {
			if opts.Underscore == "underline" {
				run.Underline = true
			} else {
				run.Italic = true
			}
		}
		runs = append(runs, run)
		buf.Reset()
	}

	doubleWord := opts.WordEscape + opts.WordEscape

	for i := 0; i < len(line); {
		rest := line[i:]

		// Tested longest-first, so with the defaults four backticks parse as
		// ``` followed by a literal backtick opening the literal remainder.
		switch {
		case strings.HasPrefix(rest, opts.LineEscape):
			buf.WriteString(rest[len(opts.LineEscape):])
			i = len(line)
			continue

		case strings.HasPrefix(rest, doubleWord):
			buf.WriteString(opts.WordEscape)
			i += len(doubleWord)
			continue

		case strings.HasPrefix(rest, opts.WordEscape):
			after := i + len(opts.WordEscape)
			if after < len(line) && !isSpaceOrTab(line[after]) {
				end := after
				for end < len(line) && !isSpaceOrTab(line[end]) {
					end++
				}
				buf.WriteString(line[after:end])
				i = end
				continue
			}
			// Followed by whitespace or end of line, so there is nothing to
			// escape and the marker stands for itself.
			buf.WriteString(opts.WordEscape)
			i = after
			continue
		}

		// A run of markers with whitespace on both sides cannot be emphasis: an
		// opener is followed by the text it emphasises and a closer is preceded
		// by it. So "bongocat * 30m ago" is literal, and no correctly written
		// emphasis is affected.
		if c := line[i]; c == '*' || c == '_' {
			if end := runEnd(line, i, c); isBareRun(line, i, end) {
				buf.WriteString(line[i:end])
				i = end
				continue
			}
		}

		switch line[i] {
		case '*':
			flush()
			bold = !bold
		case '_':
			flush()
			emph = !emph
		default:
			buf.WriteByte(line[i])
		}
		i++
	}

	flush()
	return runs, bold, emph
}

func isSpaceOrTab(b byte) bool {
	return b == ' ' || b == '\t'
}

// runEnd returns the index just past the run of c starting at i.
func runEnd(line string, i int, c byte) int {
	end := i
	for end < len(line) && line[end] == c {
		end++
	}
	return end
}

// isBareRun reports whether line[start:end] has whitespace, or the edge of the
// line, on both sides.
func isBareRun(line string, start, end int) bool {
	leftClear := start == 0 || isSpaceOrTab(line[start-1])
	rightClear := end == len(line) || isSpaceOrTab(line[end])
	return leftClear && rightClear
}
