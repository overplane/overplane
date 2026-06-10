package nav

import (
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
)

type Item struct{ ID, Title, Description, Detail string }

func Run(items []Item, title string) (*Item, error) {
	m := model{items: items, title: title, selected: -1, width: 80, height: 24}
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return nil, err
	}
	fm := final.(model)
	if fm.selected < 0 || fm.selected >= len(items) {
		return nil, nil
	}
	return &items[fm.selected], nil
}

type tickMsg time.Time

type model struct {
	items    []Item
	title    string
	cursor   int
	selected int
	quitting bool
	width    int
	height   int
}

func (m model) Init() tea.Cmd { return tick() }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		size := msg
		m.width = size.Width
		m.height = size.Height
		return m, nil
	case tickMsg:
		return m, tick()
	case tea.KeyMsg:
		return m.updateKey(msg)
	default:
		return m, nil
	}
}

func (m model) updateKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "ctrl+c", "q", "esc":
		m.selected = -1
		m.quitting = true
		return m, tea.Quit
	case "up", "k":
		m = m.moveCursor(-1)
	case "down", "j":
		m = m.moveCursor(1)
	case "home":
		m.cursor = 0
	case "end":
		if len(m.items) > 0 {
			m.cursor = len(m.items) - 1
		}
	case "enter":
		m.selected = m.cursor
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

func (m model) moveCursor(delta int) model {
	if len(m.items) == 0 {
		return m
	}
	next := m.cursor + delta
	if next < 0 {
		next = 0
	}
	if next >= len(m.items) {
		next = len(m.items) - 1
	}
	m.cursor = next
	return m
}

func (m model) View() string {
	if m.quitting {
		return ""
	}
	width := maxInt(m.width, 60)
	height := maxInt(m.height, 18)
	return renderScreen(m.viewLines(width, height), width, height)
}

func (m model) viewLines(width, height int) []string {
	lines := make([]string, 0, height)
	lines = append(lines,
		center(" "+m.title+" ", width, styleTitle),
		boxTop(width),
		tableHeader(width),
		boxSep(width),
	)
	lines = append(lines, m.tableLines(width, height)...)
	lines = append(lines, m.detailLines(width)...)
	return lines
}

func (m model) tableLines(width, height int) []string {
	lines := []string{}
	tableRows := maxInt(3, height-12)
	for i := 0; i < tableRows; i++ {
		if i < len(m.items) {
			lines = append(lines, tableRow(m.items[i], i, m.cursor, width))
			continue
		}
		lines = append(lines, boxLine("", width))
	}
	return append(lines, boxBottom(width))
}

func (m model) detailLines(width int) []string {
	if len(m.items) == 0 || m.cursor >= len(m.items) {
		return nil
	}
	lines := []string{boxTop(width)}
	for _, line := range wrapLines(m.items[m.cursor].Detail, width-4) {
		lines = append(lines, boxLine(styleDim+line+styleEnd, width))
	}
	return append(lines, boxBottom(width))
}

func renderScreen(lines []string, width, height int) string {
	var b strings.Builder
	for len(lines) < height-1 {
		lines = append(lines, "")
	}
	lines = append(lines, footerLine(width))
	for i := 0; i < height; i++ {
		if i >= len(lines) {
			b.WriteString(renderLine("", width))
		} else {
			b.WriteString(renderLine(lines[i], width))
		}
		if i < height-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func visibleLen(s string) int {
	return utf8.RuneCountInString(ansiPattern.ReplaceAllString(s, ""))
}

func pad(s string, width int) string {
	if n := visibleLen(s); n < width {
		return s + strings.Repeat(" ", width-n)
	}
	return s
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if visibleLen(s) <= width {
		return s
	}
	plain := []rune(ansiPattern.ReplaceAllString(s, ""))
	if width <= 1 {
		return "."
	}
	if width <= 3 {
		return string(plain[:width])
	}
	return string(plain[:width-3]) + "..."
}

func wrapLines(s string, width int) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return nil
	}
	var out []string
	line := ""
	for _, word := range words {
		if visibleLen(line)+1+len(word) > width {
			out = append(out, line)
			line = word
			continue
		}
		if line != "" {
			line += " "
		}
		line += word
	}
	if line != "" {
		out = append(out, line)
	}
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

const (
	base        = "\x1b[48;5;17m\x1b[38;5;153m"
	reset       = "\x1b[0m"
	styleEnd    = reset + base
	styleHi     = "\x1b[38;5;226m"
	styleOk     = "\x1b[38;5;46m"
	styleDim    = "\x1b[38;5;244m"
	styleTitle  = "\x1b[38;5;15;1m"
	styleSelect = "\x1b[48;5;19m\x1b[38;5;226;1m"
	styleBorder = "\x1b[38;5;39m"
)

func renderLine(s string, width int) string {
	return base + fit(s, width) + reset
}

func fit(s string, width int) string {
	s = truncate(s, width)
	if n := visibleLen(s); n < width {
		s += strings.Repeat(" ", width-n)
	}
	return s
}

func center(s string, width int, style string) string {
	n := visibleLen(s)
	if n >= width {
		return style + truncate(s, width) + styleEnd
	}
	left := (width - n) / 2
	return strings.Repeat(" ", left) + style + s + styleEnd
}

func boxTop(width int) string {
	return styleBorder + "┌" + strings.Repeat("─", width-2) + "┐" + styleEnd
}

func boxBottom(width int) string {
	return styleBorder + "└" + strings.Repeat("─", width-2) + "┘" + styleEnd
}

func boxSep(width int) string {
	return styleBorder + "├" + strings.Repeat("─", width-2) + "┤" + styleEnd
}

func boxLine(content string, width int) string {
	return styleBorder + "│" + styleEnd + " " + fit(content, width-4) +
		" " + styleBorder + "│" + styleEnd
}

func tableHeader(width int) string {
	content := styleHi + pad("Subsystem", 18) + " " +
		pad("Status", 12) + " " + pad("Details", width-40) + styleEnd
	return boxLine(content, width)
}

func tableRow(item Item, idx, cursor, width int) string {
	status := "ready"
	if idx == cursor {
		status = "selected"
	}
	content := pad(item.Title, 18) + " " + pad(status, 12) + " " + truncate(item.Description, width-40)
	if idx == cursor {
		content = styleSelect + content + styleEnd
	} else {
		content = styleOk + pad(item.Title, 18) + styleEnd + " " +
			styleDim + pad(status, 12) + styleEnd + " " + item.Description
	}
	return boxLine(content, width)
}

func helpLegend() string {
	return styleHi + " ↑/↓ navigate " + styleEnd +
		styleDim + " enter select  home/end jump  q quit " + styleEnd
}

func compactHelpLegend() string {
	return styleHi + " ↑/↓ nav " + styleEnd + styleDim + " q quit " + styleEnd
}

func footerLine(width int) string {
	status := rightStatus()
	help := helpLegend()
	if visibleLen(help)+visibleLen(status)+1 > width {
		help = compactHelpLegend()
	}
	space := width - visibleLen(help) - visibleLen(status)
	if space < 1 {
		help = truncate(help, maxInt(0, width-visibleLen(status)-1))
		space = 1
	}
	return help + strings.Repeat(" ", space) + status
}

func rightStatus() string {
	text := fmt.Sprintf(
		" load %s  goroutines %d  %s ",
		loadAvg(),
		runtime.NumGoroutine(),
		time.Now().Format("15:04:05"),
	)
	return styleHi + text + styleEnd
}

func loadAvg() string {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return "n/a"
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return "n/a"
	}
	return fields[0]
}
