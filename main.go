package main

import (
	"context"
	"fmt"
	"os"

	_ "github.com/joho/godotenv/autoload"

	"github.com/earthboundkid/versioninfo/v2"
	"github.com/urfave/cli/v3"
)

// this can be set at build time with: -ldflags="-X 'main.Version=X.Y.Z'"
var Version string

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(-1)
	}
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
		},
	}
	app.Commands = []*cli.Command{
		cmdRecordGet,
		cmdRecordList,
		cmdFirehose,
		cmdResolve,
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
