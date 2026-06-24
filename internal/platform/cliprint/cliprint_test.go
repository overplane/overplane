package cliprint

import (
	"os"
	"strings"
	"testing"

	"github.com/overplane/overplane/internal/platform/color"
)

func TestFormatStepBarPlain(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("CLICOLOR_FORCE", "")
	color.ResetForTest()
	got := strings.TrimSuffix(FormatStepBar("validating configuration"), "\n")
	if len([]rune(got)) != StepBarWidth {
		t.Fatalf("bar width = %d, want %d: %q", len([]rune(got)), StepBarWidth, got)
	}
	if !strings.HasPrefix(got, "› validating configuration ") {
		t.Fatalf("missing leader: %q", got)
	}
	if !strings.Contains(got, "─") {
		t.Fatalf("missing rule fill: %q", got)
	}
}

func TestFormatStepBarColored(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	t.Setenv("CLICOLOR_FORCE", "1")
	color.ResetForTest()
	if !color.Enabled() {
		t.Fatal("expected color enabled")
	}
	got := FormatStepBar("ensuring fonts")
	if strings.Contains(got, "\x1b[48;5;") {
		t.Fatalf("step bar should not use background color: %q", got)
	}
	for _, want := range []string{"\x1b[38;5;214", "\x1b[38;5;130", "\x1b[38;5;244"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected palette tone %q in: %q", want, got)
		}
	}
}

func TestFormatStepBarTruncatesLongLabels(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("CLICOLOR_FORCE", "")
	color.ResetForTest()
	long := strings.Repeat("x", StepBarWidth+20)
	got := strings.TrimSuffix(FormatStepBar(long), "\n")
	if len([]rune(got)) != StepBarWidth {
		t.Fatalf("bar width = %d, want %d", len([]rune(got)), StepBarWidth)
	}
	if !strings.Contains(got, "…") {
		t.Fatalf("expected ellipsis in truncated bar: %q", got)
	}
}
