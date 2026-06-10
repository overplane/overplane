package log

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/overplane/overplane/internal/platform/color"
)

const (
	FormatPretty = "pretty"
	FormatJSON   = "json"
)

type contextKey struct{}

var defaultLogger *slog.Logger

func Configure(format, level string, w io.Writer, verbose bool) (*slog.Logger, error) {
	l, err := New(format, level, w, verbose)
	if err != nil {
		return nil, err
	}
	slog.SetDefault(l)
	defaultLogger = l
	return l, nil
}

func New(format, level string, w io.Writer, verbose bool) (*slog.Logger, error) {
	if w == nil {
		w = io.Discard
	}
	lv, err := parseLevel(level)
	if err != nil {
		return nil, err
	}
	switch format {
	case "", FormatPretty:
		return slog.New(&prettyHandler{handlerState{w: w, level: lv, verbose: verbose, mu: &sync.Mutex{}}}), nil
	case FormatJSON:
		return slog.New(&jsonHandler{handlerState{w: w, level: lv, verbose: verbose, mu: &sync.Mutex{}}}), nil
	default:
		return nil, fmt.Errorf("invalid log format %q", format)
	}
}

func Default() *slog.Logger {
	if defaultLogger == nil {
		l, _ := New(FormatPretty, "info", io.Discard, false)
		defaultLogger = l
	}
	return defaultLogger
}

func WithContext(ctx context.Context, l *slog.Logger) context.Context {
	if l == nil {
		l = Default()
	}
	return context.WithValue(ctx, contextKey{}, l)
}

func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(contextKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return Default()
}

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func StripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid log level %q", s)
	}
}

type attr struct {
	key string
	val any
}

type handlerState struct {
	w       io.Writer
	level   slog.Level
	verbose bool
	attrs   []slog.Attr
	mu      *sync.Mutex
}

type prettyHandler struct{ handlerState }
type jsonHandler struct{ handlerState }

func (h *prettyHandler) Enabled(_ context.Context, level slog.Level) bool { return level >= h.level }
func (h *jsonHandler) Enabled(_ context.Context, level slog.Level) bool   { return level >= h.level }

func (h *prettyHandler) Handle(_ context.Context, r slog.Record) error {
	attrs := h.normalized(r)
	h.mu.Lock()
	defer h.mu.Unlock()
	isDebug := r.Level <= slog.LevelDebug
	ts := color.Sprint(7, r.Time.UTC().Format(time.RFC3339))
	slot := levelSlot(r.Level)
	level := color.Sprint(slot, fmt.Sprintf("%-5s", strings.ToUpper(r.Level.String())))
	msg := r.Message
	if r.Level != slog.LevelDebug {
		msg = fmt.Sprintf("%-45s", msg)
	}
	if r.Level >= slog.LevelWarn {
		msg = color.Sprint(slot, msg)
	} else if r.Level <= slog.LevelDebug {
		msg = color.Sprint(7, msg)
	}
	var inline []string
	var hint string
	for _, a := range attrs {
		if a.key == "hint" {
			hint = fmt.Sprint(a.val)
			continue
		}
		if isDebug {
			inline = append(inline, color.Sprint(7, a.key+"="+fmt.Sprint(a.val)))
			continue
		}
		inline = append(inline, coloredKey(a.key)+"="+fmt.Sprint(a.val))
	}
	line := fmt.Sprintf("%s %s |%s| %s\n", ts, level, msg, strings.Join(inline, " "))
	if _, err := io.WriteString(h.w, line); err != nil {
		return err
	}
	if hint != "" {
		_, err := io.WriteString(h.w, strings.Repeat(" ", 68)+color.Sprint(7, "↳ "+hint)+"\n")
		return err
	}
	return nil
}

func (h *jsonHandler) Handle(_ context.Context, r slog.Record) error {
	attrs := h.normalized(r)
	obj := make(orderedObject, 0, 3+len(attrs))
	obj = append(obj,
		orderedPair{"time", r.Time.UTC().Format(time.RFC3339)},
		orderedPair{"level", strings.ToUpper(r.Level.String())},
		orderedPair{"message", r.Message},
	)
	for _, a := range attrs {
		obj = append(obj, orderedPair{a.key, a.val})
	}
	var b bytes.Buffer
	if err := obj.write(&b); err != nil {
		return err
	}
	b.WriteByte('\n')
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write(b.Bytes())
	return err
}

func (h *prettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	cp := *h
	cp.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &cp
}

func (h *jsonHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	cp := *h
	cp.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &cp
}

func (h *prettyHandler) WithGroup(string) slog.Handler { return h }
func (h *jsonHandler) WithGroup(string) slog.Handler   { return h }

func (h *handlerState) normalized(r slog.Record) []attr {
	var raw []slog.Attr
	raw = append(raw, h.attrs...)
	r.Attrs(func(a slog.Attr) bool {
		raw = append(raw, a)
		return true
	})
	if h.verbose {
		if frame, ok := callerFrame(r.PC); ok {
			raw = append([]slog.Attr{
				slog.String("file", fmt.Sprintf("%s:%d", filepath.Base(frame.File), frame.Line)),
				slog.String("symbol", shortSymbol(frame.Function)),
			}, raw...)
		}
	}
	out := make([]attr, 0, len(raw))
	for _, a := range raw {
		a.Value = a.Value.Resolve()
		if !h.verbose && (a.Key == "file" || a.Key == "symbol" || a.Key == "source" || a.Key == "pkg") {
			continue
		}
		out = append(out, attr{key: a.Key, val: value(a.Value)})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return order(out[i].key) < order(out[j].key)
	})
	return out
}

func value(v slog.Value) any {
	if err, ok := v.Any().(error); ok {
		return err.Error()
	}
	return v.Any()
}

func callerFrame(pc uintptr) (runtime.Frame, bool) {
	if pc == 0 {
		return runtime.Frame{}, false
	}
	frames := runtime.CallersFrames([]uintptr{pc})
	f, more := frames.Next()
	return f, more || f.Function != ""
}

func shortSymbol(s string) string {
	if i := strings.LastIndexByte(s, '/'); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.LastIndexByte(s, '.'); i >= 0 {
		return s[i+1:]
	}
	return s
}

func order(k string) int {
	switch k {
	case "file":
		return 0
	case "symbol":
		return 1
	case "step":
		return 2
	default:
		return 10
	}
}

func levelSlot(l slog.Level) int {
	switch {
	case l >= slog.LevelError:
		return 2
	case l >= slog.LevelWarn:
		return 0
	case l <= slog.LevelDebug:
		return 7
	default:
		return 4
	}
}

func keySlot(k string) int {
	switch k {
	case "file", "symbol":
		return 0
	case "step":
		return 1
	case "err", "error":
		return 0
	case "result", "action":
		return 2
	}
	var h uint32 = 2166136261
	for _, c := range []byte(k) {
		h ^= uint32(c)
		h *= 16777619
	}
	return int(h & 0x0f)
}

func coloredKey(k string) string {
	switch k {
	case "err", "error":
		return fixedXterm(196, k)
	case "result", "action":
		return fixedXterm(226, k)
	default:
		return color.Sprint(keySlot(k), k)
	}
}

func fixedXterm(index uint8, s string) string {
	if !color.Enabled() {
		return s
	}
	return "\x1b[38;5;" + fmt.Sprint(index) + "m" + s + "\x1b[0m"
}

type orderedPair struct {
	k string
	v any
}

type orderedObject []orderedPair

func (o orderedObject) write(b *bytes.Buffer) error {
	b.WriteByte('{')
	for i, p := range o {
		if i > 0 {
			b.WriteByte(',')
		}
		k, _ := json.Marshal(p.k)
		b.Write(k)
		b.WriteByte(':')
		v, err := json.Marshal(p.v)
		if err != nil {
			return err
		}
		b.Write(v)
	}
	b.WriteByte('}')
	return nil
}
