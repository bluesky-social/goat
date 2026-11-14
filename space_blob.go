package main

import (
	"context"
	"fmt"
	"os"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"tangled.org/bnewbold.net/cobalt/atspace"
	"tangled.org/bnewbold.net/cobalt/atspace/xsyntax"

	"github.com/urfave/cli/v3"
)

var cmdSpaceBlob = &cli.Command{
	Name:  "blob",
	Usage: "commands for media files (blobs) in spaces",
	Flags: []cli.Flag{},
	Commands: []*cli.Command{
		&cli.Command{
			Name:      "download",
			Usage:     "download a single space blob",
			ArgsUsage: `<space> <account> <cid>`,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "output",
					Aliases: []string{"o"},
					Usage:   "file path to store blob at",
				},
			},
			Action: runSpaceBlobDownload,
		},
		&cli.Command{
			Name:      "upload",
			Usage:     "upload a file (blob) to PDS",
			ArgsUsage: `<path>`,
			Action:    runBlobUpload,
		},
	},
}

func runSpaceBlobDownload(ctx context.Context, cmd *cli.Command) error {
	dir := configDirectory(cmd.String("plc-host"))

	if cmd.Args().Len() < 3 {
		return fmt.Errorf("need to provide space, repo, and blob CID arguments")
	}
	space, err := xsyntax.ParseSpaceRef(cmd.Args().Get(1))
	if err != nil {
		return err
	}
	repo, err := syntax.ParseDID(cmd.Args().Get(2))
	if err != nil {
		return err
	}
	blobCID, err := syntax.ParseCID(cmd.Args().Get(3))
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

	blobPath := cmd.String("output")
	if blobPath == "" {
		blobPath = blobCID.String()
	}

	fmt.Printf("downloading blob to: %s\n", blobPath)

	if _, err := os.Stat(blobPath); err == nil {
		return fmt.Errorf("file exists: %s", blobPath)
	}
	rdr, err := sc.GetBlobReader(ctx, repo, blobCID)
	if err != nil {
		return err
	}
	defer rdr.Close()

	outFile, err := os.Create(blobPath)
	if err != nil {
		return err
	}
	defer outFile.Close()
	_, err = outFile.ReadFrom(rdr)

	return err
}
