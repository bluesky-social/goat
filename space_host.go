package main

import (
	"context"
	"fmt"

	// TODO: will end up in indigo
	"tangled.org/bnewbold.net/cobalt/atspace"
	spaceapi "tangled.org/bnewbold.net/cobalt/atspace/comatproto"
	"tangled.org/bnewbold.net/cobalt/atspace/xsyntax"

	"github.com/urfave/cli/v3"
)

var cmdSpaceHost = &cli.Command{
	Name:  "host",
	Usage: "commands for space hosts",
	Flags: []cli.Flag{},
	Commands: []*cli.Command{
		&cli.Command{
			Name:      "list-repos",
			Usage:     "list all space repos for given space",
			ArgsUsage: `<space>`,
			Flags:     []cli.Flag{},
			Action:    runSpaceHostListRepos,
		},
	},
}

func runSpaceHostListRepos(ctx context.Context, cmd *cli.Command) error {
	dir := configDirectory(cmd.String("plc-host"))

	space, err := xsyntax.ParseSpaceRef(cmd.Args().First())
	if err != nil {
		return err
	}

	client, err := loadAuthClient(ctx, cmd)
	if err == ErrNoAuthSession {
		return fmt.Errorf("PDS auth required, but not logged in")
	} else if err != nil {
		return err
	}
	sc, err := atspace.NewSpaceClient(ctx, dir, client, space)
	if err != nil {
		return fmt.Errorf("establishing space session: %w", err)
	}

	cursor := ""
	for {
		// ctx, c, cursor string, limit *int64, space string
		resp, err := spaceapi.SpaceListRepos(ctx, sc.SpaceHostClient, cursor, new(int64(100)), space.String())
		if err != nil {
			return err
		}
		for _, repo := range resp.Repos {
			fmt.Printf("%s\t%s\t%x\n", repo.Did, repo.Rev, repo.Hash)
		}
		if resp.Cursor != nil && *resp.Cursor != "" {
			cursor = *resp.Cursor
		} else {
			break
		}
	}

	return nil
}
