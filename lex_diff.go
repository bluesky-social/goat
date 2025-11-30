package main

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/bluesky-social/indigo/atproto/atdata"
	"github.com/bluesky-social/indigo/atproto/syntax"

	"github.com/urfave/cli/v3"
	"github.com/yudai/gojsondiff"
	"github.com/yudai/gojsondiff/formatter"
)

var cmdLexDiff = &cli.Command{
	Name:      "diff",
	Usage:     "print differences for any updated lexicon schemas",
	ArgsUsage: `<file-or-dir>*`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "lexicons-dir",
			Value:   "lexicons/",
			Usage:   "base directory for project Lexicon files",
			Sources: cli.EnvVars("LEXICONS_DIR"),
		},
	},
	Action: runLexDiff,
}

func runLexDiff(ctx context.Context, cmd *cli.Command) error {
	return runComparisons(ctx, cmd, compareDiff)
}

func compareDiff(ctx context.Context, cmd *cli.Command, nsid syntax.NSID, localJSON, remoteJSON json.RawMessage) error {

	// skip schemas which aren't in both locations
	if localJSON == nil || remoteJSON == nil {
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

	// skip if rqual
	if reflect.DeepEqual(local, remote) {
		return nil
	}

	// re-marshal with type removed
	localJSON, err = json.Marshal(local)
	if err != nil {
		return err
	}
	remoteJSON, err = json.Marshal(remote)
	if err != nil {
		return err
	}

	// compute and print diff
	var diffString string
	var outJSON map[string]interface{}
	differ := gojsondiff.New()
	d, err := differ.Compare(localJSON, remoteJSON)
	if err != nil {
		return nil
	}
	json.Unmarshal(localJSON, &outJSON)
	config := formatter.AsciiFormatterConfig{
		//ShowArrayIndex: true,
		Coloring: true,
	}
	formatter := formatter.NewAsciiFormatter(outJSON, config)
	diffString, err = formatter.Format(d)
	if err != nil {
		return err
	}

	fmt.Printf("diff %s\n", nsid)
	fmt.Println("--- local")
	fmt.Println("+++ remote")
	fmt.Print(diffString)
	fmt.Println()

	return nil
}
