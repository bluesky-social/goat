package main

import (
	"context"
	"fmt"
	"os"

	_ "github.com/joho/godotenv/autoload"

	"github.com/earthboundkid/versioninfo/v2"
	"github.com/urfave/cli/v3"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(-1)
	}
}

func run(args []string) error {

	app := cli.Command{
		Name:    "goat",
		Usage:   "Go AT protocol CLI tool",
		Version: versioninfo.Short(),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "log-level",
				Usage:   "log verbosity level (eg: warn, info, debug)",
				Sources: cli.EnvVars("GOAT_LOG_LEVEL", "GO_LOG_LEVEL", "LOG_LEVEL"),
			},
			&cli.StringFlag{
				Name:    "plc-url",
				Usage:   "URL of PLC directory to use",
				Value:   "https://plc.directory",
				Sources: cli.EnvVars("ATP_PLC_URL"),
			},
			&cli.StringFlag{
				Name:    "pds-url",
				Usage:   "URL of PDS to use (overrides DID doc)",
				Sources: cli.EnvVars("ATP_PDS_URL"),
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
		cmdPds,
		cmdRelay,
	}
	return app.Run(context.Background(), args)
}
