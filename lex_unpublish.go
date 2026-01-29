package main

import (
	"context"
	"fmt"
	"sort"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/syntax"

	"github.com/urfave/cli/v3"
)

var cmdLexUnpublish = &cli.Command{
	Name:        "unpublish",
	Usage:       "delete lexicon schema records from current account",
	Description: "Deletes published schema records from current AT account repository.\nDoes not delete local schema JSON files.",
	ArgsUsage:   `<nsid>+`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "username",
			Aliases: []string{"u"},
			Usage:   "account identifier (handle or DID) for login",
			Sources: cli.EnvVars("GOAT_USERNAME", "ATP_USERNAME", "ATP_AUTH_USERNAME"),
		},
		&cli.StringFlag{
			Name:    "password",
			Aliases: []string{"p", "app-password"},
			Usage:   "account password (app password recommended) for login",
			Sources: cli.EnvVars("GOAT_PASSWORD", "ATP_PASSWORD", "ATP_AUTH_PASSWORD"),
		},
	},
	Action: runLexUnpublish,
}

func runLexUnpublish(ctx context.Context, cmd *cli.Command) error {

	if cmd.Args().Len() == 0 {
		cli.ShowSubcommandHelpAndExit(cmd, 1)
	}

	c, err := loginOrLoadAuthClient(ctx, cmd)
	if err != nil {
		return nil
	}

	if c.AccountDID == nil {
		return fmt.Errorf("require API client to have DID configured")
	}

	nsids := []string{}
	for _, arg := range cmd.Args().Slice() {
		n, err := syntax.ParseNSID(arg)
		if err != nil {
			return err
		}
		nsids = append(nsids, n.String())
	}
	sort.Strings(nsids)

	for _, nsid := range nsids {
		if err := unpublishSchema(ctx, c, syntax.NSID(nsid)); err != nil {
			fmt.Printf(" 🟠 %s\n", nsid)
			fmt.Printf("    record deletion failed: %s\n", err.Error())
			continue
		}
		fmt.Printf(" 🟢 %s\n", nsid)
	}

	return nil
}

func unpublishSchema(ctx context.Context, c *atclient.APIClient, nsid syntax.NSID) error {

	resp, err := comatproto.RepoDeleteRecord(ctx, c, &comatproto.RepoDeleteRecord_Input{
		Collection: schemaNSID.String(),
		Repo:       c.AccountDID.String(),
		Rkey:       nsid.String(),
	})
	if err != nil {
		return err
	}

	if resp.Commit == nil {
		return fmt.Errorf("schema record did not exist")
	}

	return nil
}
