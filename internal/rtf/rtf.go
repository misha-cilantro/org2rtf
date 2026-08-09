// Package rtf serialises a document to RTF 1.x.
package rtf

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf16"

	"org2rtf/internal/config"
	"org2rtf/internal/doc"
)

// twipsPerInch is the RTF measurement unit: 1440 twips to the inch.
const twipsPerInch = 1440

// Options are the writer-relevant subset of the configuration.
type Options struct {
	Font        string
	FontSize    float64 // points
	LineSpacing string
	Margin      float64 // inches
	TabWidth    float64 // inches
}

// OptionsFrom extracts the writer options from a resolved config.
func OptionsFrom(c config.Config) Options {
	return Options{
		Font:        c.Font,
		FontSize:    c.FontSize,
		LineSpacing: c.LineSpacing,
		Margin:      c.Margin,
		TabWidth:    c.TabWidth,
	}
}

// Render returns the complete RTF document. It builds the whole thing in memory
// so a failure part-way through can never leave a truncated file on disk.
func Render(paras []doc.Paragraph, opts Options) []byte {
	var b strings.Builder

	margin := inchesToTwips(opts.Margin)
	fontSize := halfPoints(opts.FontSize)

	// \uc1 declares that each \uNNNN escape is followed by exactly one
	// replacement character for readers that cannot handle Unicode.
	b.WriteString(`{\rtf1\ansi\ansicpg1252\uc1\deff0`)
	b.WriteString(`{\fonttbl{\f0\fnil\fcharset0 `)
	writeEscaped(&b, opts.Font)
	b.WriteString(";}}")
	fmt.Fprintf(&b, `\deftab%d`, inchesToTwips(opts.TabWidth))
	fmt.Fprintf(&b, `\margl%d\margr%d\margt%d\margb%d`, margin, margin, margin, margin)
	b.WriteString("\n")

	for _, p := range paras {
		writeParagraph(&b, p, opts, fontSize)
	}

	b.WriteString("}\n")
	return []byte(b.String())
}

func writeParagraph(b *strings.Builder, p doc.Paragraph, opts Options, fontSize int) {
	b.WriteString(`\pard\plain`)
	if p.Centered {
		b.WriteString(`\qc`)
	}
	if sl := spacingControl(opts.LineSpacing); sl != "" {
		b.WriteString(sl)
	}
	fmt.Fprintf(b, `\f0\fs%d`, fontSize)

	// The space terminates the preceding control word and is consumed by the
	// reader, so it does not add a space to the text.
	b.WriteString(" ")

	for _, run := range p.Runs {
		style := runStyle(run)
		if style == "" {
			writeEscaped(b, run.Text)
			continue
		}
		// A group scopes the styling, so there is nothing to close explicitly
		// and a run left open at end of line cannot leak into the next one.
		b.WriteString("{" + style + " ")
		writeEscaped(b, run.Text)
		b.WriteString("}")
	}

	b.WriteString("\\par\n")
}

func runStyle(r doc.Run) string {
	var s string
	if r.Bold {
		s += `\b`
	}
	if r.Italic {
		s += `\i`
	}
	if r.Underline {
		s += `\ul`
	}
	return s
}

// spacingControl returns the \sl control words for a spacing mode. \slmult1
// makes \sl a multiple of single line height, where 240 is single spaced.
func spacingControl(mode string) string {
	switch mode {
	case config.OneAndHalf:
		return `\sl360\slmult1`
	case config.Double:
		return `\sl480\slmult1`
	default:
		return ""
	}
}

// writeEscaped emits text with RTF's reserved characters escaped. Tabs become
// \tab so runs of them survive exactly, and non-ASCII becomes \uNNNN? so smart
// quotes, dashes and accents round-trip.
func writeEscaped(b *strings.Builder, text string) {
	for _, r := range text {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == '{':
			b.WriteString(`\{`)
		case r == '}':
			b.WriteString(`\}`)
		case r == '\t':
			// The trailing space delimits the control word and is consumed.
			b.WriteString(`\tab `)
		case r < 0x20:
			fmt.Fprintf(b, `\'%02x`, r)
		case r < 0x80:
			b.WriteRune(r)
		case r <= 0xFFFF:
			writeUnicode(b, r)
		default:
			// Outside the BMP: emit the UTF-16 surrogate pair.
			hi, lo := utf16.EncodeRune(r)
			writeUnicode(b, hi)
			writeUnicode(b, lo)
		}
	}
}

// writeUnicode emits one code unit as \uN followed by the "?" substitute that
// \uc1 tells the reader to skip.
//
// The space before the "?" delimits the control word explicitly. Both forms are
// spec-legal, but without it some readers (pandoc's RTF reader among them)
// treat "?" as the delimiter and then swallow the character after it, so
// "caf\u233?, x" loses the comma.
func writeUnicode(b *strings.Builder, r rune) {
	fmt.Fprintf(b, `\u%d ?`, signed16(r))
}

// signed16 converts a code unit to the signed 16-bit value \u expects.
func signed16(r rune) int {
	v := int(r)
	if v > 32767 {
		v -= 65536
	}
	return v
}

func inchesToTwips(in float64) int {
	return int(math.Round(in * twipsPerInch))
}

func halfPoints(pt float64) int {
	return int(math.Round(pt * 2))
}
