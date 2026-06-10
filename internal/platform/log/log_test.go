package log_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/overplane/overplane/internal/platform/color"
	oplog "github.com/overplane/overplane/internal/platform/log"
)

func TestPrettyAndJSON(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var pretty bytes.Buffer
	l, err := oplog.Configure(oplog.FormatPretty, "debug", &pretty, true)
	if err != nil {
		t.Fatal(err)
	}
	l.Info("hello", "step", "unit", "hint", "fix it")
	if got := oplog.StripANSI(pretty.String()); !strings.Contains(got, "INFO") || !strings.Contains(got, "fix it") {
		t.Fatalf("bad pretty: %s", got)
	}
	pretty.Reset()
	l.Warn("warn line")
	l.Info("info line")
	got := oplog.StripANSI(pretty.String())
	if !strings.Contains(got, "WARN  |warn line") || !strings.Contains(got, "INFO  |info line") {
		t.Fatalf("levels should be padded to width 5: %q", got)
	}
	ctx := oplog.WithContext(context.Background(), l)
	if oplog.FromContext(ctx) != l || oplog.Default() == nil {
		t.Fatal("context logger not returned")
	}
	var js bytes.Buffer
	jl, err := oplog.New(oplog.FormatJSON, "info", &js, false)
	if err != nil {
		t.Fatal(err)
	}
	jl.LogAttrs(context.Background(), slog.LevelInfo, "json", slog.String("path", "x"))
	var obj map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(js.Bytes()), &obj); err != nil {
		t.Fatalf("invalid json %s: %v", js.String(), err)
	}
	if _, err := oplog.New("bad", "info", &js, false); err == nil {
		t.Fatal("expected bad format error")
	}
	if _, err := oplog.New(oplog.FormatJSON, "bad", &js, false); err == nil {
		t.Fatal("expected bad level error")
	}
}

func TestPrettySpecialKeyColors(t *testing.T) {
	oldNoColor, hadNoColor := os.LookupEnv("NO_COLOR")
	if err := os.Unsetenv("NO_COLOR"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if hadNoColor {
			_ = os.Setenv("NO_COLOR", oldNoColor)
			return
		}
		_ = os.Unsetenv("NO_COLOR")
	})
	t.Setenv("CLICOLOR_FORCE", "1")
	color.ResetForTest()
	var out bytes.Buffer
	l, err := oplog.New(oplog.FormatPretty, "info", &out, false)
	if err != nil {
		t.Fatal(err)
	}
	l.Info("colored keys", "action", "preview", "err", "boom")
	got := out.String()
	if !strings.Contains(got, "\x1b[38;5;226maction\x1b[0m=preview") {
		t.Fatalf("action key not fixed yellow: %q", got)
	}
	if !strings.Contains(got, "\x1b[38;5;196merr\x1b[0m=boom") {
		t.Fatalf("err key not fixed red: %q", got)
	}
}

func TestPrettyLevelColors(t *testing.T) {
	oldNoColor, hadNoColor := os.LookupEnv("NO_COLOR")
	if err := os.Unsetenv("NO_COLOR"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if hadNoColor {
			_ = os.Setenv("NO_COLOR", oldNoColor)
			return
		}
		_ = os.Unsetenv("NO_COLOR")
	})
	t.Setenv("CLICOLOR_FORCE", "1")
	color.ResetForTest()
	var out bytes.Buffer
	l, err := oplog.New(oplog.FormatPretty, "debug", &out, false)
	if err != nil {
		t.Fatal(err)
	}
	l.Error("error line")
	l.Warn("warn line")
	l.Debug("debug line", "action", "preview")
	got := out.String()
	if !strings.Contains(got, "\x1b[38;5;166mERROR\x1b[0m") || !strings.Contains(got, "|\x1b[38;5;166merror line") {
		t.Fatalf("error label/message should use error slot: %q", got)
	}
	if !strings.Contains(got, "\x1b[38;5;214mWARN \x1b[0m") || !strings.Contains(got, "|\x1b[38;5;214mwarn line") {
		t.Fatalf("warn label/message should use warn slot: %q", got)
	}
	if !strings.Contains(got, "\x1b[38;5;244mDEBUG\x1b[0m") ||
		!strings.Contains(got, "\x1b[38;5;244maction=preview\x1b[0m") {
		t.Fatalf("debug row should be dim: %q", got)
	}
}
