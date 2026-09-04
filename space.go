package main

import (
	"github.com/urfave/cli/v3"
)

var cmdSpace = &cli.Command{
	Name:  "space",
	Usage: "commands for permissioned data spaces",
	Commands: []*cli.Command{
		cmdSpaceRecord,
	},
}
