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

// Options are the parser-relevant subset of the configuration.
type Options struct {
	Underscore  string // "italic" or "underline"
	LineEscape  string
	WordEscape  string
	SceneMarker string
	SceneGlyph  string
	EndText     string
}

// Parse converts src into paragraphs. Conversion begins after the story begin
// marker and stops at the end marker, or at end of file if there isn't one.
func Parse(src []byte, opts Options) ([]doc.Paragraph, error) {
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
		return nil, ErrNoBeginMarker
	}
	if endIdx >= 0 && endIdx < beginIdx {
		return nil, ErrEndBeforeBegin
	}

	var paras []doc.Paragraph
	for _, line := range lines[beginIdx+1:] {
		trimmed := trimLeadingSpace(line)

		switch {
		// The end marker is checked first so an indented one still terminates
		// the story instead of being swallowed by the comment rule below.
		case hasMarker(trimmed, endMarkers):
			return append(paras, endParagraph(opts)...), nil

		// Scene changes are column-0 strict, unlike comments: an indented "**"
		// is plausibly bold text opening a tab-indented paragraph.
		case strings.HasPrefix(line, opts.SceneMarker):
			paras = append(paras, literalParagraph(opts.SceneGlyph))

		case strings.HasPrefix(trimmed, "#"):
			// Comment. Leading whitespace is tolerated so comments can be
			// indented to match the prose around them.

		default:
			paras = append(paras, doc.Paragraph{Runs: parseLine(line, opts)})
		}
	}
	return append(paras, endParagraph(opts)...), nil
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
// and anything still open simply closes at end of line.
func parseLine(line string, opts Options) []doc.Run {
	var (
		runs []doc.Run
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
	return runs
}

func isSpaceOrTab(b byte) bool {
	return b == ' ' || b == '\t'
}
