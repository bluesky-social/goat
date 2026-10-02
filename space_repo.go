package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"tangled.org/bnewbold.net/cobalt/atspace"
	"tangled.org/bnewbold.net/cobalt/atspace/xsyntax"
	// TODO: will end up in indigo
	spaceapi "tangled.org/bnewbold.net/cobalt/atspace/comatproto"

	"github.com/urfave/cli/v3"
)

var cmdSpaceRepo = &cli.Command{
	Name:  "repo",
	Usage: "commands for space repos",
	Flags: []cli.Flag{},
	Commands: []*cli.Command{
		&cli.Command{
			Name:      "export",
			Usage:     "download space repo CAR file",
			ArgsUsage: `<space> <repo>`,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "output",
					Aliases: []string{"o"},
					Usage:   "file path for CAR download",
				},
			},
			Action: runSpaceRepoExport,
		},
		&cli.Command{
			Name:      "latest",
			Usage:     "print commit metadata for space repo",
			ArgsUsage: `<space> <repo>`,
			Flags:     []cli.Flag{},
			Action:    runSpaceRepoLatest,
		},
		&cli.Command{
			Name:      "oplog",
			Usage:     "print space repo operations (ops)",
			ArgsUsage: `<space> <repo>`,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "since",
					Usage: "revision (TID) to show ops since",
				},
			},
			Action: runSpaceRepoOplog,
		},
	},
}

func runSpaceRepoExport(ctx context.Context, cmd *cli.Command) error {
	dir := configDirectory(cmd.String("plc-host"))

	if cmd.Args().Len() != 2 {
		return fmt.Errorf("need to provide space-ref and repo-ident as arguments")
	}
	space, err := xsyntax.ParseSpaceRef(cmd.Args().First())
	if err != nil {
		return err
	}
	username := cmd.Args().Slice()[1]
	if username == "" {
		return fmt.Errorf("need to provide username as an argument")
	}
	ident, err := resolveIdent(ctx, cmd, username)
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

	carPath := cmd.String("output")
	if carPath == "" {
		// NOTE: having the rev in the the path might be nice
		now := time.Now().Format("20060102150405")
		// XXX: somehow encode the space-ref in the filename?
		carPath = fmt.Sprintf("%s.%s.space.car", username, now)
	}
	output, err := getFileOrStdout(carPath)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("file already exists: %s", carPath)
		}
		return err
	}
	defer output.Close()
	if carPath != stdIOPath {
		fmt.Printf("downloading to: %s\n", carPath)
	}
	body, err := sc.GetSpaceRepoCAR(ctx, ident.DID)
	if err != nil {
		// TODO: clean up file if error?
		return err
	}
	defer body.Close()
	_, err = io.Copy(output, body)
	// TODO: clean up file if error?
	return err
}

func runSpaceRepoLatest(ctx context.Context, cmd *cli.Command) error {
	dir := configDirectory(cmd.String("plc-host"))

	if cmd.Args().Len() != 2 {
		return fmt.Errorf("need to provide space-ref and repo-ident as arguments")
	}
	space, err := xsyntax.ParseSpaceRef(cmd.Args().First())
	if err != nil {
		return err
	}
	username := cmd.Args().Slice()[1]
	if username == "" {
		return fmt.Errorf("need to provide username as an argument")
	}
	ident, err := resolveIdent(ctx, cmd, username)
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

	commit, err := sc.GetLatestSpaceCommit(ctx, ident.DID)
	if err != nil {
		// TODO: clean up file if error?
		return err
	}

	// TODO: handle byte encoding as atdata correctly?
	b, err := json.MarshalIndent(commit, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(b))
	return nil
}

func runSpaceRepoOplog(ctx context.Context, cmd *cli.Command) error {
	dir := configDirectory(cmd.String("plc-host"))

	if cmd.Args().Len() != 2 {
		return fmt.Errorf("need to provide space-ref and repo-ident as arguments")
	}
	space, err := xsyntax.ParseSpaceRef(cmd.Args().First())
	if err != nil {
		return err
	}
	username := cmd.Args().Slice()[1]
	if username == "" {
		return fmt.Errorf("need to provide username as an argument")
	}
	ident, err := resolveIdent(ctx, cmd, username)
	if err != nil {
		return err
	}

	since := cmd.String("since")
	if since != "" {
		_, err := syntax.ParseTID(since)
		if err != nil {
			return fmt.Errorf("invalid 'since' revision: %w", err)
		}
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
	rc, err := sc.RepoClient(ctx, ident.DID)
	if err != nil {
		return fmt.Errorf("establishing space repo connection: %w", err)
	}

	cursor := ""
	for {
		// ctx, c, cursor string, excludeValues *bool, limit *int64, repo string, since string, space string
		resp, err := spaceapi.SpaceListRepoOps(ctx, rc, cursor, new(true), new(int64(100)), ident.DID.String(), since, space.String())
		if err != nil {
			return err
		}
		for _, op := range resp.Ops {
			fmt.Printf("%s\t%s/%s\t%v\t%v\n", op.Rev, op.Collection, op.Rkey, ptrVal(op.Prev), ptrVal(op.Cid))
		}
		if resp.Cursor != nil && *resp.Cursor != "" {
			cursor = *resp.Cursor
		} else {
			break
		}
	}

	return nil
}

func ptrVal(s *string) string {
	if s == nil {
		return "nil"
	}
	return *s
}
