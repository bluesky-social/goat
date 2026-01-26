package main

import (
	"context"
	"fmt"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	appbsky "github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/atproto/syntax"
	lexutil "github.com/bluesky-social/indigo/lex/util"

	"github.com/urfave/cli/v3"
)

var cmdBsky = &cli.Command{
	Name:  "bsky",
	Usage: "commands for bluesky social app (eg, posting)",
	Flags: []cli.Flag{},
	Commands: []*cli.Command{
		&cli.Command{
			Name:      "post",
			Usage:     "create a bluesky post",
			ArgsUsage: `<text>`,
			Action:    runBskyPost,
		},
		cmdBskyPrefs,
	},
}

func runBskyPost(ctx context.Context, cmd *cli.Command) error {
	text := cmd.Args().First()
	if text == "" {
		return fmt.Errorf("need to provide post text as argument")
	}

	client, err := loadAuthClient(ctx, cmd)
	if err == ErrNoAuthSession {
		return fmt.Errorf("auth required, but not logged in")
	} else if err != nil {
		return err
	}

	post := appbsky.FeedPost{
		Text:      text,
		CreatedAt: syntax.DatetimeNow().String(),
	}
	resp, err := comatproto.RepoCreateRecord(ctx, client, &comatproto.RepoCreateRecord_Input{
		Collection: "app.bsky.feed.post",
		Repo:       client.AccountDID.String(),
		Record:     &lexutil.LexiconTypeDecoder{Val: &post},
	})
	if err != nil {
		return err
	}

	fmt.Printf("%s\t%s\n", resp.Uri, resp.Cid)
	aturi, err := syntax.ParseATURI(resp.Uri)
	if err != nil {
		return err
	}
	fmt.Printf("view post at: https://bsky.app/profile/%s/post/%s\n", aturi.Authority(), aturi.RecordKey())
	return nil
}
