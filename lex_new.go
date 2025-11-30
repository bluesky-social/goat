package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"

	"github.com/bluesky-social/indigo/atproto/atdata"
	"github.com/bluesky-social/indigo/atproto/syntax"

	"github.com/urfave/cli/v3"
)

//go:embed lexicon-templates/record.json
var tmplRecord string

//go:embed lexicon-templates/query.json
var tmplQuery string

//go:embed lexicon-templates/query-view.json
var tmplQueryView string

//go:embed lexicon-templates/query-list.json
var tmplQueryList string

//go:embed lexicon-templates/procedure.json
var tmplProcedure string

//go:embed lexicon-templates/permission-set.json
var tmplPermissionSet string

var cmdLexNew = &cli.Command{
	Name:        "new",
	Usage:       "create new lexicon schema from template",
	ArgsUsage:   "<schema-type> <nsid>",
	Description: "Instantiates new schemas (JSON files) from templates, with provided NSID substituted.",
	Arguments: []cli.Argument{
		&cli.StringArg{
			Name: "schema-type",
		},
		&cli.StringArg{
			Name: "nsid",
		},
	},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "lexicons-dir",
			Value:   "lexicons/",
			Usage:   "base directory for project Lexicon files",
			Sources: cli.EnvVars("LEXICONS_DIR"),
		},
		&cli.BoolFlag{
			Name:    "list-templates",
			Aliases: []string{"l"},
			Usage:   "list available templates (schema types)",
		},
	},
	Action: runLexNew,
}

func runLexNew(ctx context.Context, cmd *cli.Command) error {

	if cmd.Bool("list-templates") {
		fmt.Println("Available schema templates:")
		fmt.Println("")
		fmt.Println("  record")
		fmt.Println("  query")
		fmt.Println("  query-view")
		fmt.Println("  query-list")
		fmt.Println("  procedure")
		fmt.Println("  permission-set")
		fmt.Println("")
		return nil
	}

	if cmd.StringArg("nsid") == "" {
		cli.ShowSubcommandHelpAndExit(cmd, 1)
	}

	nsid, err := syntax.ParseNSID(cmd.StringArg("nsid"))
	if err != nil {
		return fmt.Errorf("invalid schema NSID syntax: %w", err)
	}

	schemaType := cmd.StringArg("schema-type")

	var orig json.RawMessage
	switch schemaType {
	case "record":
		orig = []byte(tmplRecord)
	case "query":
		orig = []byte(tmplQuery)
	case "query-view":
		orig = []byte(tmplQueryView)
	case "query-list":
		orig = []byte(tmplQueryList)
	case "procedure":
		orig = []byte(tmplProcedure)
	case "permission-set":
		orig = []byte(tmplPermissionSet)
	default:
		return fmt.Errorf("unknown schema template: %s", schemaType)
	}

	d, err := atdata.UnmarshalJSON(orig)
	if err != nil {
		return err
	}
	d["id"] = nsid.String()

	b, err := json.Marshal(d)
	if err != nil {
		return err
	}

	fpath := pathForNSID(cmd, nsid)
	_, err = os.Stat(fpath)
	if err == nil {
		return fmt.Errorf("output file already exists: %s", fpath)
	}

	return writeLexiconFile(ctx, cmd, nsid, fpath, json.RawMessage(b))
}
