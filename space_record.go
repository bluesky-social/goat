package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/bluesky-social/indigo/api/agnostic"
	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/atdata"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"tangled.org/bnewbold.net/cobalt/atspace"
	"tangled.org/bnewbold.net/cobalt/atspace/xsyntax"
	"tangled.org/bnewbold.net/cobalt/pdsclient"

	// TODO: will end up in indigo
	spaceapi "tangled.org/bnewbold.net/cobalt/atspace/comatproto"

	"github.com/urfave/cli/v3"
)

var cmdSpaceRecord = &cli.Command{
	Name:  "record",
	Usage: "commands for space repo records",
	Flags: []cli.Flag{},
	Commands: []*cli.Command{
		cmdSpaceRecordGet,
		cmdSpaceRecordList,
		&cli.Command{
			Name:      "create",
			Usage:     "create record from JSON",
			ArgsUsage: `<file|->`,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "rkey",
					Aliases: []string{"r"},
					Usage:   "record key",
				},
				&cli.BoolFlag{
					Name:    "no-validate",
					Aliases: []string{"n"},
					Usage:   "tells PDS not to validate record Lexicon schema",
				},
			},
			Action: runSpaceRecordCreate,
		},
		&cli.Command{
			Name:      "update",
			Usage:     "replace existing record from JSON",
			ArgsUsage: `<file>`,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "rkey",
					Aliases:  []string{"r"},
					Required: true,
					Usage:    "record key",
				},
				&cli.BoolFlag{
					Name:    "no-validate",
					Aliases: []string{"n"},
					Usage:   "tells PDS not to validate record Lexicon schema",
				},
			},
			Action: runSpaceRecordUpdate,
		},
		&cli.Command{
			Name:  "delete",
			Usage: "delete an existing record",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "collection",
					Aliases:  []string{"c"},
					Required: true,
					Usage:    "collection (NSID)",
				},
				&cli.StringFlag{
					Name:     "rkey",
					Aliases:  []string{"r"},
					Required: true,
					Usage:    "record key",
				},
			},
			Action: runSpaceRecordDelete,
		},
	},
}

var cmdSpaceRecordGet = &cli.Command{
	Name:      "get",
	Usage:     "fetch space record from the network",
	ArgsUsage: `<space-uri>`,
	Flags:     []cli.Flag{},
	Action:    runSpaceRecordGet,
}

var cmdSpaceRecordList = &cli.Command{
	Name:      "ls",
	Aliases:   []string{"list"},
	Usage:     "list all records for a space repo",
	ArgsUsage: `<space-ref> <repo>`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "collection",
			Usage: "only list records from a specific collection",
		},
	},
	Action: runSpaceRecordList,
}

func runSpaceRecordGet(ctx context.Context, cmd *cli.Command) error {
	dir := configDirectory(cmd.String("plc-host"))

	uriArg := cmd.Args().First()
	if uriArg == "" {
		return fmt.Errorf("expected a single space URI argument")
	}

	suri, err := xsyntax.ParseSpaceURI(uriArg)
	if err != nil {
		return fmt.Errorf("not a valid space URI: %v", err)
	}

	client, err := loadAuthClient(ctx, cmd)
	if err == ErrNoAuthSession {
		return fmt.Errorf("PDS auth required, but not logged in")
	} else if err != nil {
		return err
	}
	user := *client.AccountDID

	var record *json.RawMessage
	if user == suri.Repo() {
		pc := pdsclient.PDSClient{
			APIClient:  client,
			AccountDID: user,
		}
		record, _, err = pc.GetSpaceRecord(ctx, suri.SpaceRef(), suri.Collection(), suri.RecordKey())
		if err != nil {
			return err
		}
	} else {
		sc, err := atspace.NewSpaceClient(ctx, dir, client, suri.SpaceRef())
		if err != nil {
			return fmt.Errorf("establishing space session: %w", err)
		}
		record, _, err = sc.GetSpaceRecord(ctx, user, suri.Collection(), suri.RecordKey())
		if err != nil {
			return err
		}
	}

	b, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(b))
	return nil
}

func runSpaceRecordList(ctx context.Context, cmd *cli.Command) error {
	dir := configDirectory(cmd.String("plc-host"))

	if cmd.Args().Len() != 2 {
		return fmt.Errorf("need to provide space-ref and repo-ident as arguments")
	}
	space, err := xsyntax.ParseSpaceRef(cmd.Args().First())
	if err != nil {
		return err
	}
	username := cmd.Args().Slice()[1]
	ident, err := resolveIdent(ctx, cmd, username)
	if err != nil {
		return err
	}
	repo := ident.DID

	client, err := loadAuthClient(ctx, cmd)
	if err == ErrNoAuthSession {
		return fmt.Errorf("PDS auth required, but not logged in")
	} else if err != nil {
		return err
	}
	user := *client.AccountDID
	var c *atclient.APIClient

	if user == repo {
		c = client
	} else {
		sc, err := atspace.NewSpaceClient(ctx, dir, client, space)
		if err != nil {
			return fmt.Errorf("establishing space session: %w", err)
		}
		c, err = sc.RepoClient(ctx, repo)
		if err != nil {
			return fmt.Errorf("establishing space repo client: %w", err)
		}
	}

	// TODO: optional arg
	collection := ""

	cursor := ""
	for {
		// ctx, client, collection string, cursor string, excludeValues *bool, limit *int64, repo string, reverse *bool, space string
		resp, err := spaceapi.SpaceListRecords(ctx, c, collection, cursor, new(false), new(int64(100)), repo.String(), nil, space.String())
		if err != nil {
			return err
		}
		for _, rec := range resp.Records {
			fmt.Printf("%s\t%s\t%s\n", rec.Collection, rec.Rkey, rec.Cid)
		}
		if resp.Cursor != nil && *resp.Cursor != "" {
			cursor = *resp.Cursor
		} else {
			break
		}
	}

	return nil
}

func runSpaceRecordCreate(ctx context.Context, cmd *cli.Command) error {
	recordPath := cmd.Args().First()
	if recordPath == "" {
		return fmt.Errorf("need to provide file path or '-' for stdin as an argument")
	}

	client, err := loadAuthClient(ctx, cmd)
	if err == ErrNoAuthSession {
		return fmt.Errorf("auth required, but not logged in")
	} else if err != nil {
		return err
	}

	inputReader, err := getFileOrStdin(recordPath)
	if err != nil {
		return err
	}

	recordBytes, err := io.ReadAll(inputReader)
	if err != nil {
		return err
	}

	recordVal, err := atdata.UnmarshalJSON(recordBytes)
	if err != nil {
		return err
	}

	nsid, err := atdata.ExtractTypeJSON(recordBytes)
	if err != nil {
		return fmt.Errorf("failed to extract '$type' from record data: %w", err)
	}
	if nsid == "" {
		return fmt.Errorf("failed to parse '$type' from record data: empty or undefined")
	}

	var rkey *string
	if cmd.String("rkey") != "" {
		rk, err := syntax.ParseRecordKey(cmd.String("rkey"))
		if err != nil {
			return err
		}
		s := rk.String()
		rkey = &s
	}
	validate := !cmd.Bool("no-validate")

	resp, err := agnostic.RepoCreateRecord(ctx, client, &agnostic.RepoCreateRecord_Input{
		Collection: nsid,
		Repo:       client.AccountDID.String(),
		Record:     recordVal,
		Rkey:       rkey,
		Validate:   &validate,
	})
	if err != nil {
		return err
	}

	fmt.Printf("%s\t%s\n", resp.Uri, resp.Cid)
	return nil
}

func runSpaceRecordUpdate(ctx context.Context, cmd *cli.Command) error {
	recordPath := cmd.Args().First()
	if recordPath == "" {
		return fmt.Errorf("need to provide file path as an argument")
	}

	client, err := loadAuthClient(ctx, cmd)
	if err == ErrNoAuthSession {
		return fmt.Errorf("auth required, but not logged in")
	} else if err != nil {
		return err
	}

	recordBytes, err := os.ReadFile(recordPath)
	if err != nil {
		return err
	}

	recordVal, err := atdata.UnmarshalJSON(recordBytes)
	if err != nil {
		return err
	}

	nsid, err := atdata.ExtractTypeJSON(recordBytes)
	if err != nil {
		return fmt.Errorf("failed to extract '$type' from record data: %w", err)
	}
	if nsid == "" {
		return fmt.Errorf("failed to parse '$type' from record data: empty or undefined")
	}

	rkey := cmd.String("rkey")

	// NOTE: need to fetch existing record CID to perform swap. this is optional in theory, but golang can't deal with "optional" and "nullable", so we always need to set this (?)
	existing, err := agnostic.RepoGetRecord(ctx, client, "", nsid, client.AccountDID.String(), rkey)
	if err != nil {
		return err
	}

	validate := !cmd.Bool("no-validate")

	resp, err := agnostic.RepoPutRecord(ctx, client, &agnostic.RepoPutRecord_Input{
		Collection: nsid,
		Repo:       client.AccountDID.String(),
		Record:     recordVal,
		Rkey:       rkey,
		Validate:   &validate,
		SwapRecord: existing.Cid,
	})
	if err != nil {
		return err
	}

	fmt.Printf("%s\t%s\n", resp.Uri, resp.Cid)
	return nil
}

func runSpaceRecordDelete(ctx context.Context, cmd *cli.Command) error {

	client, err := loadAuthClient(ctx, cmd)
	if err == ErrNoAuthSession {
		return fmt.Errorf("auth required, but not logged in")
	} else if err != nil {
		return err
	}

	rkey, err := syntax.ParseRecordKey(cmd.String("rkey"))
	if err != nil {
		return err
	}
	collection, err := syntax.ParseNSID(cmd.String("collection"))
	if err != nil {
		return err
	}

	_, err = comatproto.RepoDeleteRecord(ctx, client, &comatproto.RepoDeleteRecord_Input{
		Collection: collection.String(),
		Repo:       client.AccountDID.String(),
		Rkey:       rkey.String(),
	})
	if err != nil {
		return err
	}
	return nil
}
