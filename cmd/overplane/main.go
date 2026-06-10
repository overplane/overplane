package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/overplane/overplane/internal/cli"
	"github.com/overplane/overplane/internal/platform/color"
	openv "github.com/overplane/overplane/internal/platform/env"
	oplog "github.com/overplane/overplane/internal/platform/log"
	"github.com/overplane/overplane/internal/platform/telemetry"
	"github.com/overplane/overplane/internal/platform/version"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	ctx := context.Background()
	_ = openv.Load(ctx, "")
	openv.Normalize()

	fs := flag.NewFlagSet(cli.Binary, flag.ContinueOnError)
	fs.SetOutput(stderr)
	logFormat := fs.String("log-format", openv.String("LOG_FORMAT", oplog.FormatPretty), "log format")
	logLevel := fs.String("log-level", openv.String("LOG_LEVEL", "info"), "log level")
	logFile := fs.String("log-file", openv.String("LOG_FILE", ""), "log file")
	verbose := fs.Bool("verbose", false, "verbose logs")
	fs.BoolVar(verbose, "v", false, "verbose logs")
	showVersion := fs.Bool("version", false, "show version")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if *showVersion {
		fmt.Fprintln(stdout, version.String(cli.Binary))
		return 0
	}

	logW := stderr
	var logCloser io.Closer
	if *logFile != "" {
		f, err := os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 4
		}
		logW = f
		logCloser = f
		defer logCloser.Close()
	}
	logger, err := oplog.Configure(*logFormat, *logLevel, logW, *verbose)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if _, err := color.ApplyResolvedTheme(""); err != nil {
		logger.Warn("theme resolution failed", "err", err, "hint", "set OVERPLANE_THEME to a valid theme file")
	}
	ctx = oplog.WithContext(ctx, logger)
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	tel, err := telemetry.Init(ctx)
	if err != nil {
		logger.Error("telemetry init failed", "err", err)
		return 1
	}
	defer tel.Shutdown(context.Background())

	r := &cli.Runner{In: os.Stdin, Out: stdout, Err: stderr, Tel: tel}
	if err := cli.Dispatch(ctx, r, fs.Args()); err != nil {
		code := cli.ExitCode(err)
		if code != 2 || len(fs.Args()) > 0 {
			fmt.Fprintln(stderr, err)
		}
		return code
	}
	return 0
}
