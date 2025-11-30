package main

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/bluesky-social/indigo/atproto/atdata"
	"github.com/bluesky-social/indigo/atproto/lexicon"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/lex/lexlint"

	"github.com/urfave/cli/v3"
)

var cmdLexBreaking = &cli.Command{
	Name:      "breaking",
	Usage:     "check for changes that break lexicon evolution rules",
	ArgsUsage: `<file-or-dir>*`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "lexicons-dir",
			Value:   "lexicons/",
			Usage:   "base directory for project Lexicon files",
			Sources: cli.EnvVars("LEXICONS_DIR"),
		},
		&cli.BoolFlag{
			Name:  "json",
			Usage: "output structured JSON",
		},
	},
	Action: runLexBreaking,
}

func runLexBreaking(ctx context.Context, cmd *cli.Command) error {
	return runComparisons(ctx, cmd, compareBreaking)
}

func compareBreaking(ctx context.Context, cmd *cli.Command, nsid syntax.NSID, localJSON, remoteJSON json.RawMessage) error {

	// skip schemas which aren't in both locations
	if localJSON == nil || remoteJSON == nil {
		return nil
	}

	localData, err := atdata.UnmarshalJSON(localJSON)
	if err != nil {
		return err
	}
	remoteData, err := atdata.UnmarshalJSON(remoteJSON)
	if err != nil {
		return err
	}
	delete(localData, "$type")
	delete(remoteData, "$type")

	// skip if rqual
	if reflect.DeepEqual(localData, remoteData) {
		return nil
	}

	// parse as schema files
	var local lexicon.SchemaFile
	err = json.Unmarshal(localJSON, &local)
	if err == nil {
		err = local.FinishParse()
	}
	if err == nil {
		err = local.CheckSchema()
	}
	if err != nil {
		return err
	}

	var remote lexicon.SchemaFile
	err = json.Unmarshal(remoteJSON, &remote)
	if err == nil {
		err = remote.FinishParse()
	}
	if err == nil {
		err = local.CheckSchema()
	}
	if err != nil {
		return err
	}

	issues := lexlint.BreakingChanges(&remote, &local)

	if cmd.Bool("json") {
		for _, iss := range issues {
			b, err := json.Marshal(iss)
			if err != nil {
				return nil
			}
			fmt.Println(string(b))
		}
	} else {
		if len(issues) == 0 {
			fmt.Printf(" 🟢 %s\n", nsid)
		} else {
			fmt.Printf(" 🟡 %s\n", nsid)
			for _, iss := range issues {
				fmt.Printf("    [%s]: %s\n", iss.LintName, iss.Message)
			}
		}
	}
	if len(issues) > 0 {
		return ErrLintFailures
	}
	return nil
}
