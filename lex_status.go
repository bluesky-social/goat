package main

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/bluesky-social/indigo/atproto/atdata"
	"github.com/bluesky-social/indigo/atproto/syntax"

	"github.com/urfave/cli/v3"
)

var cmdLexStatus = &cli.Command{
	Name:        "status",
	Usage:       "check if local lexicons are in-sync with live network",
	Description: "Enumerates all local lexicons (JSON files), and checks for changes against the live network\nWill detect new published lexicons under a known lexicon group, but will not discover new groups under the same domain prefix.\nOperates on entire ./lexicons/ directory unless specific files or directories are provided.",
	ArgsUsage:   `<file-or-dir>*`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "lexicons-dir",
			Value:   "lexicons/",
			Usage:   "base directory for project Lexicon files",
			Sources: cli.EnvVars("LEXICONS_DIR"),
		},
	},
	Action: runLexStatus,
}

func runLexStatus(ctx context.Context, cmd *cli.Command) error {
	return runComparisons(ctx, cmd, compareStatus)
}

func compareStatus(ctx context.Context, cmd *cli.Command, nsid syntax.NSID, localJSON, remoteJSON json.RawMessage) error {

	// new remote schema (missing local)
	if localJSON == nil {
		fmt.Printf(" ⭕ %s\n", nsid)
		return nil
	}

	// new unpublished local schema
	if remoteJSON == nil {
		fmt.Printf(" 🟠 %s\n", nsid)
		return nil
	}

	local, err := atdata.UnmarshalJSON(localJSON)
	if err != nil {
		return err
	}
	remote, err := atdata.UnmarshalJSON(remoteJSON)
	if err != nil {
		return err
	}
	delete(local, "$type")
	delete(remote, "$type")
	if reflect.DeepEqual(local, remote) {
		fmt.Printf(" 🟢 %s\n", nsid)
	} else {
		fmt.Printf(" 🟣 %s\n", nsid)
	}
	return nil
}
