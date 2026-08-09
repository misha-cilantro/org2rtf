// Package doc defines the intermediate representation shared by the parser and
// the writer. Keeping it separate means the RTF writer never has to import the
// org parser, and either side can be tested without the other.
package doc

// Run is a stretch of text sharing one set of character styles. Text may
// contain tab characters; escaping them is the writer's job.
type Run struct {
	Text      string
	Bold      bool
	Italic    bool
	Underline bool
}

// Paragraph is one output paragraph. A paragraph with no runs is an empty
// paragraph, which is meaningful: blank source lines produce them.
type Paragraph struct {
	Runs     []Run
	Centered bool
}
