package main

import (
	"bytes"
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

	"github.com/urfave/cli/v3"
)

var cmdSpaceRepo = &cli.Command{
	Name:  "repo",
	Usage: "commands for space repos",
	Flags: []cli.Flag{},
	Commands: []*cli.Command{
		&cli.Command{
			Name:      "export",
			Usage:     "download space repo STAR file",
			ArgsUsage: `<space> <repo>`,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "output",
					Aliases: []string{"o"},
					Usage:   "file path for STAR download",
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
		&cli.Command{
			Name:      "verify",
			Usage:     "check integrity (but not authenticity) of space repo STAR file",
			ArgsUsage: `<star-file>`,
			Flags:     []cli.Flag{},
			Action:    runSpaceRepoVerify,
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

	starPath := cmd.String("output")
	if starPath == "" {
		// NOTE: having the rev in the the path might be nice
		now := time.Now().Format("20060102150405")
		// XXX: somehow encode the space-ref in the filename?
		starPath = fmt.Sprintf("%s.%s.space.star", username, now)
	}
	output, err := getFileOrStdout(starPath)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("file already exists: %s", starPath)
		}
		return err
	}
	defer output.Close()
	if starPath != stdIOPath {
		fmt.Printf("downloading to: %s\n", starPath)
	}
	body, err := sc.GetSpaceRepoBytes(ctx, ident.DID)
	if err != nil {
		// TODO: clean up file if error?
		return err
	}
	_, err = io.Copy(output, bytes.NewBuffer(body))
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

	var since syntax.TID
	rawsince := cmd.String("since")
	if rawsince != "" {
		since, err = syntax.ParseTID(rawsince)
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

	cursor := ""
	for {

		_, entries, next, err := sc.ListSpaceRepoOps(ctx, ident.DID, since, cursor)
		if err != nil {
			return err
		}

		for _, op := range entries {

			c := "nil"
			if op.CID != nil {
				c = op.CID.String()
			}
			prev := "nil"
			if op.Prev != nil {
				prev = op.Prev.String()
			}
			fmt.Printf("%s\t%s/%s\t%v\t%v\n", op.Revision, op.Collection, op.RecordKey, prev, c)
		}
		if next == "" {
			break
		}
		cursor = next
	}

	return nil
}

func runSpaceRepoVerify(ctx context.Context, cmd *cli.Command) error {
	starPath := cmd.Args().First()
	if starPath == "" {
		return fmt.Errorf("need to provide path to STAR file as argument")
	}
	fi, err := os.Open(starPath)
	if err != nil {
		return fmt.Errorf("failed to open STAR file: %w", err)
	}
	defer fi.Close()

	if err := atspace.VerifyRepo(atspace.NewRepoReader(fi)); err != nil {
		return err
	}

	fmt.Println("valid")
	return nil
}
