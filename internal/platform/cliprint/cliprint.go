// Package cliprint renders consistent, colorized CLI progress output shared by
// sitebuild, infrabuild, and other tooling wrappers.
package cliprint

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/overplane/overplane/internal/platform/color"
)

// StepBarWidth is the fixed terminal width for step section markers.
const StepBarWidth = 80

const (
	stepLead = "› "
	stepSep  = " "
	stepFill = "─"

	stepMarkerSlot = 0 // titles
	stepLabelSlot  = 1 // steps
	stepRuleSlot   = 7 // dim rule tail
)

// Printer writes pipeline-style CLI output to stdout/stderr.
type Printer struct {
	Out    io.Writer
	ErrOut io.Writer
}

// Default prints to the process stdout/stderr.
var Default = New(os.Stdout, os.Stderr)

// New returns a Printer writing to out and errW.
func New(out, errW io.Writer) Printer {
	if out == nil {
		out = os.Stdout
	}
	if errW == nil {
		errW = os.Stderr
	}
	return Printer{Out: out, ErrOut: errW}
}

// Step prints a fixed-width step marker for a pipeline stage.
func (p Printer) Step(format string, a ...any) {
	fmt.Fprint(p.Out, FormatStepBar(fmt.Sprintf(format, a...)))
}

// FormatStepBar renders a bold marker, step-colored label, and dim rule tail.
func FormatStepBar(label string) string {
	leadW := labelWidth(stepLead)
	sepW := labelWidth(stepSep)
	label = fitLabel(label, StepBarWidth-leadW-sepW)
	fill := strings.Repeat(stepFill, StepBarWidth-leadW-sepW-labelWidth(label))
	if !color.Enabled() {
		return stepLead + label + stepSep + fill + "\n"
	}
	return color.Sprint(stepMarkerSlot, stepLead) +
		color.Sprint(stepLabelSlot, label) +
		color.Sprint(stepRuleSlot, stepSep+fill) +
		"\n"
}

// Ok prints a success line.
func (p Printer) Ok(format string, a ...any) {
	fmt.Fprint(p.Out, color.Sprint(3, "✓ "))
	fmt.Fprintf(p.Out, format+"\n", a...)
}

// Warn prints a warning line.
func (p Printer) Warn(format string, a ...any) {
	fmt.Fprint(p.Out, color.Sprint(2, "! "))
	fmt.Fprintf(p.Out, format+"\n", a...)
}

// Err prints an error line to stderr.
func (p Printer) Err(format string, a ...any) {
	fmt.Fprint(p.ErrOut, color.Sprint(0, "✗ "))
	fmt.Fprintf(p.ErrOut, format+"\n", a...)
}

// Info prints a dim, indented detail line.
func (p Printer) Info(format string, a ...any) {
	fmt.Fprint(p.Out, color.Sprint(7, "  "+fmt.Sprintf(format, a...)+"\n"))
}

func fitLabel(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	if limit == 1 {
		return string(runes[:1])
	}
	return string(runes[:limit-1]) + "…"
}

func labelWidth(s string) int {
	return len([]rune(s))
}
