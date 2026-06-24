package version

import (
	"fmt"
	"runtime"
	"strings"
)

var (
	Version = "0.0.3"
	Commit  = "dev"
	Date    = "unknown"
)

func String(binary string) string {
	return fmt.Sprintf("%s v%s (%s, %s)", binary, Version, Commit, Date)
}

// BuildDate returns the calendar-date portion of the stamped RFC3339 build
// timestamp (e.g. "2026-06-10"); unstamped values pass through unchanged.
func BuildDate() string {
	if d, _, ok := strings.Cut(Date, "T"); ok {
		return d
	}
	return Date
}

func Runtime() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}
