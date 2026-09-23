package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

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
		&cli.Command{
			Name:      "get",
			Usage:     "fetch space record from the network",
			ArgsUsage: `<space-uri>`,
			Flags:     []cli.Flag{},
			Action:    runSpaceRecordGet,
		},
		&cli.Command{
			Name:      "ls",
			Aliases:   []string{"list"},
			Usage:     "list all records for a space repo",
			ArgsUsage: `<space-ref> <repo-or-self>`,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "collection",
					Usage: "only list records from a specific collection",
				},
			},
			Action: runSpaceRecordList,
		},
		&cli.Command{
			Name:      "create",
			Usage:     "create record from JSON",
			ArgsUsage: `<space-ref> <file|->`,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "rkey",
					Aliases: []string{"r"},
					Usage:   "record key",
				},
			},
			Action: runSpaceRecordCreate,
		},
		&cli.Command{
			Name:      "update",
			Usage:     "replace existing record from JSON",
			ArgsUsage: `<space-ref> <file>`,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "rkey",
					Aliases:  []string{"r"},
					Required: true,
					Usage:    "record key",
				},
			},
			Action: runSpaceRecordUpdate,
		},
		&cli.Command{
			Name:      "delete",
			Usage:     "delete an existing record",
			ArgsUsage: `<space-ref> <collection> <rkey>`,
			Flags:     []cli.Flag{},
			Action:    runSpaceRecordDelete,
		},
	},
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

	client, err := loadAuthClient(ctx, cmd)
	if err == ErrNoAuthSession {
		return fmt.Errorf("PDS auth required, but not logged in")
	} else if err != nil {
		return err
	}
	user := *client.AccountDID
	var c *atclient.APIClient

	username := cmd.Args().Slice()[1]
	var repo syntax.DID
	if username == "self" {
		repo = user
	} else {
		ident, err := resolveIdent(ctx, cmd, username)
		if err != nil {
			return err
		}
		repo = ident.DID
	}

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
	if cmd.Args().Len() != 2 {
		return fmt.Errorf("need to provide space-ref and repo-ident as arguments")
	}
	space, err := xsyntax.ParseSpaceRef(cmd.Args().First())
	if err != nil {
		return err
	}

	recordPath := cmd.Args().Slice()[1]
	if recordPath == "" {
		return fmt.Errorf("need to provide file path or '-' for stdin as an argument")
	}

	client, err := loadAuthClient(ctx, cmd)
	if err == ErrNoAuthSession {
		return fmt.Errorf("auth required, but not logged in")
	} else if err != nil {
		return err
	}
	pc := pdsclient.PDSClient{
		APIClient:  client,
		AccountDID: *client.AccountDID,
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

	nsidStr, err := atdata.ExtractTypeJSON(recordBytes)
	if err != nil {
		return fmt.Errorf("failed to extract '$type' from record data: %w", err)
	}
	if nsidStr == "" {
		return fmt.Errorf("failed to parse '$type' from record data: empty or undefined")
	}
	nsid, err := syntax.ParseNSID(nsidStr)
	if err != nil {
		return err
	}

	var rkey syntax.RecordKey
	if cmd.String("rkey") != "" {
		rkey, err = syntax.ParseRecordKey(cmd.String("rkey"))
		if err != nil {
			return err
		}
	}

	ruri, rcid, err := pc.CreateSpaceRecord(ctx, space, nsid, rkey, recordVal)
	if err != nil {
		return err
	}

	fmt.Printf("%s\t%s\n", ruri, rcid)
	return nil
}

func runSpaceRecordUpdate(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 2 {
		return fmt.Errorf("need to provide space-ref and repo-ident as arguments")
	}
	space, err := xsyntax.ParseSpaceRef(cmd.Args().First())
	if err != nil {
		return err
	}

	recordPath := cmd.Args().Slice()[1]
	if recordPath == "" {
		return fmt.Errorf("need to provide file path as an argument")
	}

	client, err := loadAuthClient(ctx, cmd)
	if err == ErrNoAuthSession {
		return fmt.Errorf("auth required, but not logged in")
	} else if err != nil {
		return err
	}
	pc := pdsclient.PDSClient{
		APIClient:  client,
		AccountDID: *client.AccountDID,
	}

	recordBytes, err := os.ReadFile(recordPath)
	if err != nil {
		return err
	}

	recordVal, err := atdata.UnmarshalJSON(recordBytes)
	if err != nil {
		return err
	}

	nsidStr, err := atdata.ExtractTypeJSON(recordBytes)
	if err != nil {
		return fmt.Errorf("failed to extract '$type' from record data: %w", err)
	}
	if nsidStr == "" {
		return fmt.Errorf("failed to parse '$type' from record data: empty or undefined")
	}
	nsid, err := syntax.ParseNSID(nsidStr)
	if err != nil {
		return err
	}

	rkey, err := syntax.ParseRecordKey(cmd.String("rkey"))
	if err != nil {
		return err
	}

	rcid, err := pc.PutSpaceRecord(ctx, space, nsid, rkey, recordVal)
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", rcid)
	return nil
}

func runSpaceRecordDelete(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 3 {
		return fmt.Errorf("need to provide space-ref and repo-ident as arguments")
	}
	space, err := xsyntax.ParseSpaceRef(cmd.Args().First())
	if err != nil {
		return err
	}
	collection, err := syntax.ParseNSID(cmd.Args().Slice()[1])
	if err != nil {
		return err
	}
	rkey, err := syntax.ParseRecordKey(cmd.Args().Slice()[2])
	if err != nil {
		return err
	}

	client, err := loadAuthClient(ctx, cmd)
	if err == ErrNoAuthSession {
		return fmt.Errorf("auth required, but not logged in")
	} else if err != nil {
		return err
	}
	pc := pdsclient.PDSClient{
		APIClient:  client,
		AccountDID: *client.AccountDID,
	}

	return pc.DeleteSpaceRecord(ctx, space, collection, rkey)
}
