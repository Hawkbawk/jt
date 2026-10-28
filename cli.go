package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"runtime/debug"
)

// version is set at build time with -ldflags "-X main.version=...".
var version string

const usage = `jt is a terminal kanban board for Jira.

Usage:
  jt [flags]

Flags:
  -h, --help       Show this help
  -v, --version    Show the version

Environment:
  JIRA_CONFIG_FILE    Path to the jira-cli config (default $XDG_CONFIG_HOME/.jira/.config.yml)
  JT_CONFIG_FILE      Path to the jt settings file

jt reuses jira-cli's config and credentials, so run 'jira init' first.
`

func buildVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

// parseArgs handles the flags jt supports. It reports whether jt should exit
// instead of starting the UI.
func parseArgs(args []string, stdout io.Writer) (done bool, err error) {
	fs := flag.NewFlagSet("jt", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var help, showVersion bool
	fs.BoolVar(&help, "help", false, "")
	fs.BoolVar(&help, "h", false, "")
	fs.BoolVar(&showVersion, "version", false, "")
	fs.BoolVar(&showVersion, "v", false, "")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			help = true
		} else {
			return true, fmt.Errorf("%w (see 'jt --help')", err)
		}
	}
	switch {
	case help:
		_, err = fmt.Fprint(stdout, usage)
		return true, err
	case showVersion:
		_, err = fmt.Fprintln(stdout, "jt", buildVersion())
		return true, err
	case fs.NArg() > 0:
		return true, fmt.Errorf("unexpected argument %q (see 'jt --help')", fs.Arg(0))
	}
	return false, nil
}
