package main

import (
	"context"
	"fmt"
	"os"

	_ "github.com/joho/godotenv/autoload"

	"github.com/earthboundkid/versioninfo/v2"
	"github.com/urfave/cli/v3"
	"golang.org/x/term"
)

// this can be set at build time with: -ldflags="-X 'main.Version=X.Y.Z'"
var Version string

func main() {
	if err := run(os.Args); err != nil {
		if term.IsTerminal(int(os.Stderr.Fd())) && os.Getenv("NO_COLOR") == "" {
			fmt.Fprintf(os.Stderr, "\033[1;31merror:\033[0m %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		os.Exit(-1)
	}
}

func stderrIsTerminal() bool {
	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func run(args []string) error {

	cmdVersion := versioninfo.Short()
	if Version != "" {
		cmdVersion = Version
	}

	app := cli.Command{
		Name:    "goat",
		Usage:   "Go AT protocol CLI tool",
		Version: cmdVersion,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "log-level",
				Usage:   "log verbosity level (eg: warn, info, debug)",
				Sources: cli.EnvVars("GOAT_LOG_LEVEL", "GO_LOG_LEVEL", "LOG_LEVEL"),
			},
			&cli.StringFlag{
				Name:    "plc-host",
				Usage:   "method, hostname, and port of PLC directory",
				Value:   "https://plc.directory",
				Sources: cli.EnvVars("ATP_PLC_HOST"),
			},
			&cli.BoolFlag{
				Name:  "no-color",
				Usage: "disable colored output",
			},
		},
	}
	app.Commands = []*cli.Command{
		cmdRecordGet,
		cmdRecordList,
		cmdFirehose,
		cmdResolve,
		cmdXrpc,
		cmdRepo,
		cmdBlob,
		cmdLex,
		cmdAccount,
		cmdPLC,
		cmdBsky,
		cmdRecord,
		cmdSyntax,
		cmdKey,
		cmdPDS,
		cmdRelay,
	}
	return app.Run(context.Background(), args)
}

func stderrIsTerminal() bool {
	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
