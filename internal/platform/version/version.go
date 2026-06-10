package version

import (
	"fmt"
	"runtime"
)

var (
	Version = "0.0.2"
	Commit  = "dev"
	Date    = "unknown"
)

func String(binary string) string {
	return fmt.Sprintf("%s v%s (%s, %s)", binary, Version, Commit, Date)
}

func Runtime() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}
