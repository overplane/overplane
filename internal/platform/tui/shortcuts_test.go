package tui_test

import (
	"context"
	"errors"
	"testing"

	"github.com/overplane/overplane/internal/platform/tui"
)

func TestNonInteractiveShortcuts(t *testing.T) {
	if err := tui.EnsureInteractive("hint"); !errors.Is(err, tui.ErrNotInteractive) {
		t.Fatalf("ensure = %v", err)
	}
	if _, err := tui.ConfirmYesNo(context.Background(), "t", "d", true); !errors.Is(err, tui.ErrNotInteractive) {
		t.Fatalf("confirm = %v", err)
	}
	if _, err := tui.PromptString(context.Background(), "t", "d", "p", nil); !errors.Is(err, tui.ErrNotInteractive) {
		t.Fatalf("prompt = %v", err)
	}
	oneOption := []tui.Option{{Value: "a", Label: "A"}}
	if _, err := tui.Select(context.Background(), "t", oneOption); !errors.Is(err, tui.ErrNotInteractive) {
		t.Fatalf("select = %v", err)
	}
	if _, err := tui.MultiSelect(context.Background(), "t", oneOption, nil); !errors.Is(err, tui.ErrNotInteractive) {
		t.Fatalf("multi = %v", err)
	}
	if _, err := tui.ChoosePath(context.Background(), "t", ".", true); !errors.Is(err, tui.ErrNotInteractive) {
		t.Fatalf("path = %v", err)
	}
	_ = tui.Theme(false)
	if err := tui.OpenEditor(context.Background(), "x"); !errors.Is(err, tui.ErrNotInteractive) {
		t.Fatalf("editor = %v", err)
	}
}
