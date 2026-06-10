package nav

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModelUpdateView(t *testing.T) {
	m := model{items: []Item{
		{ID: "a", Title: "A", Description: "first", Detail: "detail a"},
		{ID: "b", Title: "B", Description: "second", Detail: "detail b"},
	}, title: "Title"}
	if m.Init() == nil {
		t.Fatal("expected tick command")
	}
	if view := m.View(); !strings.Contains(view, "Title") ||
		!strings.Contains(view, "detail a") ||
		!strings.Contains(view, "goroutines") {
		t.Fatalf("bad view: %s", view)
	}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(model)
	if m.width != 100 || m.height != 30 {
		t.Fatalf("size = %dx%d", m.width, m.height)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(model)
	if m.cursor != 1 {
		t.Fatalf("cursor = %d", m.cursor)
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if m.selected != 1 || cmd == nil {
		t.Fatalf("selected=%d cmd=%v", m.selected, cmd)
	}
}
