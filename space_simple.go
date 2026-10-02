package main

import (
	"context"
	"fmt"

	// TODO: will end up in indigo
	spaceapi "tangled.org/bnewbold.net/cobalt/atspace/comatproto"

	"github.com/urfave/cli/v3"
)

var cmdSpaceSimple = &cli.Command{
	Name:  "simple",
	Usage: "commands for managing simplespace spaces",
	Flags: []cli.Flag{},
	Commands: []*cli.Command{
		&cli.Command{
			Name:    "list",
			Aliases: []string{"ls"},
			Usage:   "list all space repos for current account (not just simplespace)",
			Flags:   []cli.Flag{},
			Action:  runSpaceSimpleList,
		},
	},
}

func runSpaceSimpleList(ctx context.Context, cmd *cli.Command) error {

	client, err := loadAuthClient(ctx, cmd)
	if err == ErrNoAuthSession {
		return fmt.Errorf("PDS auth required, but not logged in")
	} else if err != nil {
		return err
	}

	cursor := ""
	for {
		// ctx, c, cursor string, did string, limit *int64, spaceType string
		resp, err := spaceapi.SpaceListSpaces(ctx, client, cursor, "", new(int64(100)), "")
		if err != nil {
			return err
		}
		for _, sp := range resp.Spaces {
			fmt.Println(sp.Uri)
		}
		if resp.Cursor != nil && *resp.Cursor != "" {
			cursor = *resp.Cursor
		} else {
			break
		}
	}

	return nil
}
