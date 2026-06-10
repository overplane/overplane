package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var out, errb bytes.Buffer
	if code := run([]string{"--version"}, &out, &errb); code != 0 || !strings.Contains(out.String(), "overplane v") {
		t.Fatalf("version code=%d out=%s err=%s", code, out.String(), errb.String())
	}
	out.Reset()
	errb.Reset()
	code := run([]string{"--log-format=json", "version"}, &out, &errb)
	if code != 0 || !strings.Contains(out.String(), "overplane v") {
		t.Fatalf("run version code=%d out=%s err=%s", code, out.String(), errb.String())
	}
	out.Reset()
	errb.Reset()
	if code := run([]string{"unknown"}, &out, &errb); code != 2 {
		t.Fatalf("unknown code=%d", code)
	}
}
