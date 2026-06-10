package color

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/jedib0t/go-pretty/v6/table"
	"golang.org/x/term"
)

type Palette [16]uint8

var defaultPalette = Palette{214, 130, 166, 64, 67, 136, 95, 244, 180, 94, 101, 172, 173, 58, 230, 238}

var (
	mu          sync.RWMutex
	active      = defaultPalette
	enabledOnce sync.Once
	enabled     bool
)

type HelpFlag struct{ Name, Placeholder, Description string }

type HelpSpec struct {
	Command, Usage, Description string
	Flags                       []HelpFlag
	Examples                    []string
}

func Get() Palette {
	mu.RLock()
	defer mu.RUnlock()
	return active
}

func Set(p Palette) {
	mu.Lock()
	defer mu.Unlock()
	active = p
}

func Enabled() bool {
	enabledOnce.Do(func() {
		if _, ok := os.LookupEnv("NO_COLOR"); ok {
			enabled = false
			return
		}
		if v := os.Getenv("CLICOLOR_FORCE"); v != "" && v != "0" {
			enabled = true
			return
		}
		enabled = term.IsTerminal(int(os.Stdout.Fd())) || term.IsTerminal(int(os.Stderr.Fd()))
	})
	return enabled
}

func FG(slot int) string {
	if !Enabled() {
		return ""
	}
	return "\x1b[38;5;" + strconv.Itoa(int(Get()[slot&0x0f])) + "m"
}

func BG(slot int) string {
	if !Enabled() {
		return ""
	}
	return "\x1b[48;5;" + strconv.Itoa(int(Get()[slot&0x0f])) + "m"
}

func Sprint(slot int, s string) string {
	if !Enabled() {
		return s
	}
	return FG(slot) + s + "\x1b[0m"
}

func Fprint(w io.Writer, slot int, s string) (int, error) {
	return io.WriteString(w, Sprint(slot, s))
}

func Table(out io.Writer) table.Writer {
	t := table.NewWriter()
	t.SetOutputMirror(out)
	style := table.StyleRounded
	style.Name = "overplane"
	style.Options.DrawBorder = true
	style.Options.SeparateHeader = true
	style.Options.SeparateRows = false
	t.SetStyle(style)
	return t
}

func RenderHelp(spec HelpSpec) string {
	var b strings.Builder
	if spec.Command != "" {
		b.WriteString(Sprint(0, spec.Command))
		if spec.Description != "" {
			b.WriteString(" - ")
			b.WriteString(spec.Description)
		}
		b.WriteByte('\n')
	}
	if spec.Usage != "" {
		b.WriteString(Sprint(7, "Usage: "))
		b.WriteString(spec.Usage)
		b.WriteString("\n")
	}
	if len(spec.Flags) > 0 {
		b.WriteString("\n")
		b.WriteString(Sprint(0, "Flags"))
		b.WriteByte('\n')
		for _, f := range spec.Flags {
			name := Sprint(4, f.Name)
			if f.Placeholder != "" {
				name += " " + Sprint(8, f.Placeholder)
			}
			fmt.Fprintf(&b, "  %-24s %s\n", name, f.Description)
		}
	}
	if len(spec.Examples) > 0 {
		b.WriteString("\n")
		b.WriteString(Sprint(2, "Examples"))
		b.WriteByte('\n')
		for _, ex := range spec.Examples {
			b.WriteString("  ")
			b.WriteString(Sprint(7, ex))
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func ResetForTest() {
	mu.Lock()
	active = defaultPalette
	mu.Unlock()
	enabledOnce = sync.Once{}
	enabled = false
}
