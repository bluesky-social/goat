package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/atdata"
	"github.com/bluesky-social/indigo/atproto/repo"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/util"

	"github.com/ipfs/go-cid"
	"github.com/urfave/cli/v3"
)

var cmdRepo = &cli.Command{
	Name:  "repo",
	Usage: "commands for public atproto data repositories",
	Flags: []cli.Flag{},
	Commands: []*cli.Command{
		&cli.Command{
			Name:      "export",
			Usage:     "download CAR file for given account",
			ArgsUsage: `<at-identifier>`,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "output",
					Aliases: []string{"o"},
					Usage:   "file path for CAR download",
				},
			},
			Action: runRepoExport,
		},
		&cli.Command{
			Name:      "import",
			Usage:     "upload CAR file for current account",
			ArgsUsage: `<path>`,
			Action:    runRepoImport,
		},
		&cli.Command{
			Name:      "ls",
			Aliases:   []string{"list"},
			Usage:     "list records in CAR file",
			ArgsUsage: `<car-file>`,
			Flags:     []cli.Flag{},
			Action:    runRepoList,
		},
		&cli.Command{
			Name:      "inspect",
			Usage:     "show commit metadata from CAR file",
			ArgsUsage: `<car-file>`,
			Flags:     []cli.Flag{},
			Action:    runRepoInspect,
		},
		&cli.Command{
			Name:      "mst",
			Usage:     "show repo MST structure",
			ArgsUsage: `<car-file>`,
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:    "full-cid",
					Aliases: []string{"f"},
					Usage:   "display full CIDs",
				},
				&cli.StringFlag{
					Name:    "root",
					Aliases: []string{"r"},
					Usage:   "CID of root block",
				},
			},
			Action: runRepoMST,
		},
		&cli.Command{
			Name:      "unpack",
			Usage:     "extract records from CAR file as directory of JSON files",
			ArgsUsage: `<car-file>`,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "output",
					Aliases: []string{"o"},
					Usage:   "directory path for unpack",
				},
			},
			Action: runRepoUnpack,
		},
	},
}

func runRepoExport(ctx context.Context, cmd *cli.Command) error {
	username := cmd.Args().First()
	if username == "" {
		return fmt.Errorf("need to provide username as an argument")
	}
	ident, err := resolveIdent(ctx, cmd, username)
	if err != nil {
		return err
	}

	// create a new API client to connect to the account's PDS
	c := atclient.NewAPIClient(ident.PDSEndpoint())
	c.Headers.Set("User-Agent", userAgentString())
	if c.Host == "" {
		return fmt.Errorf("no PDS endpoint for identity")
	}

	// set longer timeout, for large CAR files
	c.Client = util.RobustHTTPClient()
	c.Client.Timeout = 600 * time.Second

	carPath := cmd.String("output")
    if carPath == "" {

		if strings.Contains(username, ":") {
			username = strings.ReplaceAll(username, ":", "_")
		}

        now := time.Now().Format("20060102150405")
        carPath = fmt.Sprintf("%s.%s.car", username, now)
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
		fmt.Printf("downloading from %s to: %s\n", c.Host, carPath)
	}
	repoBytes, err := comatproto.SyncGetRepo(ctx, c, ident.DID.String(), "")
	if err != nil {
		return err
	}
	if _, err := output.Write(repoBytes); err != nil {
		return err
	}
	return nil
}

func runRepoImport(ctx context.Context, cmd *cli.Command) error {

	carPath := cmd.Args().First()
	if carPath == "" {
		return fmt.Errorf("need to provide CAR file path as an argument")
	}

	client, err := loadAuthClient(ctx, cmd)
	if err == ErrNoAuthSession {
		return fmt.Errorf("auth required, but not logged in")
	} else if err != nil {
		return err
	}

	fileBytes, err := os.ReadFile(carPath)
	if err != nil {
		return err
	}

	err = comatproto.RepoImportRepo(ctx, client, bytes.NewReader(fileBytes))
	if err != nil {
		return fmt.Errorf("failed to import repo: %w", err)
	}

	return nil
}

func runRepoList(ctx context.Context, cmd *cli.Command) error {
	carPath := cmd.Args().First()
	if carPath == "" {
		return fmt.Errorf("need to provide path to CAR file as argument")
	}
	fi, err := os.Open(carPath)
	if err != nil {
		return fmt.Errorf("failed to open CAR file: %w", err)
	}

	// read repository tree in to memory
	_, r, err := repo.LoadRepoFromCAR(ctx, fi)
	if err != nil {
		return fmt.Errorf("failed to parse repo CAR file: %w", err)
	}

	err = r.MST.Walk(func(k []byte, v cid.Cid) error {
		fmt.Printf("%s\t%s\n", string(k), v.String())
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to read records from repo CAR file: %w", err)
	}
	return nil
}

func runRepoInspect(ctx context.Context, cmd *cli.Command) error {
	carPath := cmd.Args().First()
	if carPath == "" {
		return fmt.Errorf("need to provide path to CAR file as argument")
	}
	fi, err := os.Open(carPath)
	if err != nil {
		return err
	}

	// read repository tree in to memory
	c, _, err := repo.LoadRepoFromCAR(ctx, fi)
	if err != nil {
		return err
	}

	fmt.Printf("ATProto Repo Spec Version: %d\n", c.Version)
	fmt.Printf("DID: %s\n", c.DID)
	fmt.Printf("Data CID: %s\n", c.Data)
	fmt.Printf("Prev CID: %s\n", c.Prev)
	fmt.Printf("Revision: %s\n", c.Rev)
	// TODO: Signature?

	return nil
}

func runRepoMST(ctx context.Context, cmd *cli.Command) error {
	opts := repoMSTOptions{
		carPath: cmd.Args().First(),
		fullCID: cmd.Bool("full-cid"),
		root:    cmd.String("root"),
	}
	// read from file or stdin
	if opts.carPath == "" {
		return fmt.Errorf("need to provide path to CAR file as argument")
	}
	inputCAR, err := getFileOrStdin(opts.carPath)
	if err != nil {
		return err
	}
	return prettyMST(ctx, inputCAR, opts)
}

func runRepoUnpack(ctx context.Context, cmd *cli.Command) error {
	carPath := cmd.Args().First()
	if carPath == "" {
		return fmt.Errorf("need to provide path to CAR file as argument")
	}
	fi, err := os.Open(carPath)
	if err != nil {
		return err
	}

	c, r, err := repo.LoadRepoFromCAR(ctx, fi)
	if err != nil {
		return err
	}

	// extract DID from repo commit
	did, err := syntax.ParseDID(c.DID)
	if err != nil {
		return err
	}

	sanitizedDID := strings.ReplaceAll(did.String(), ":", "_")

	topDir := cmd.String("output")
	if topDir == "" {
		topDir = sanitizedDID
	}
	fmt.Printf("writing output to: %s\n", topDir)

	// first the commit object as a meta file
	commitPath := topDir + "/_commit.json"
	os.MkdirAll(filepath.Dir(commitPath), os.ModePerm)
	commitJSON, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(commitPath, commitJSON, 0666); err != nil {
		return err
	}

	// then all the actual records
	err = r.MST.Walk(func(k []byte, v cid.Cid) error {
		col, rkey, err := syntax.ParseRepoPath(string(k))
		if err != nil {
			return err
		}
		recBytes, _, err := r.GetRecordBytes(ctx, col, rkey)
		if err != nil {
			return err
		}

		rec, err := atdata.UnmarshalCBOR(recBytes)
		if err != nil {
			return err
		}

		recPath := topDir + "/" + string(k)
		fmt.Printf("%s.json\n", recPath)
		err = os.MkdirAll(filepath.Dir(recPath), os.ModePerm)
		if err != nil {
			return err
		}
		recJSON, err := json.MarshalIndent(rec, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(recPath+".json", recJSON, 0666); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}
	return nil
}
