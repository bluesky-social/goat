package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/syntax"

	"github.com/urfave/cli/v3"
)

var cmdPDS = &cli.Command{
	Name:  "pds",
	Usage: "commands for inspecting and administering PDS hosts",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "pds-host",
			Usage:   "method, hostname, and port of PDS instance",
			Sources: cli.EnvVars("PDS_HOST"),
		},
	},
	Commands: []*cli.Command{
		&cli.Command{
			Name:      "describe",
			Usage:     "shows info about a PDS instance",
			ArgsUsage: `<host-url>`,
			Action:    runPDSDescribe,
		},
		&cli.Command{
			Name:  "account",
			Usage: "commands for accounts/repos on PDS",
			Commands: []*cli.Command{
				&cli.Command{
					Name:    "list",
					Aliases: []string{"ls"},
					Usage:   "enumerate all accounts",
					Flags: []cli.Flag{
						&cli.BoolFlag{
							Name:  "json",
							Usage: "print output as JSON lines",
						},
					},
					Action: runPDSAccountList,
				},
				&cli.Command{
					Name:      "status",
					ArgsUsage: `<did>`,
					Usage:     "describe status of individual account",
					Flags: []cli.Flag{
						&cli.BoolFlag{
							Name:  "json",
							Usage: "print output as JSON",
						},
					},
					Action: runPDSAccountStatus,
				},
			},
		},
		cmdPDSAdmin,
	},
}

func pdsHostArg(cmd *cli.Command) (string, error) {
	pdsHost := cmd.Args().First()
	if pdsHost == "" {
		pdsHost = cmd.String("pds-host")
	}
	if pdsHost == "" {
		return "", fmt.Errorf("need to provide PDS host/URL")
	}
	if !strings.Contains(pdsHost, "://") {
		_, err := syntax.ParseHandle(pdsHost)
		if err != nil {
			return "", fmt.Errorf("PDS host is not a URL or hostname: %s", pdsHost)
		}
		pdsHost = "https://" + pdsHost
	}
	return pdsHost, nil
}

func runPDSDescribe(ctx context.Context, cmd *cli.Command) error {

	pdsHost, err := pdsHostArg(cmd)
	if err != nil {
		return err
	}
	client := atclient.NewAPIClient(pdsHost)
	client.Headers.Set("User-Agent", userAgentString())

	resp, err := comatproto.ServerDescribeServer(ctx, client)
	if err != nil {
		return err
	}

	b, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))

	return nil
}

func runPDSAccountList(ctx context.Context, cmd *cli.Command) error {

	pdsHost, err := pdsHostArg(cmd)
	if err != nil {
		return err
	}
	client := atclient.NewAPIClient(pdsHost)
	client.Headers.Set("User-Agent", userAgentString())

	cursor := ""
	var size int64 = 500
	for {
		resp, err := comatproto.SyncListRepos(ctx, client, cursor, size)
		if err != nil {
			return err
		}

		for _, r := range resp.Repos {
			if cmd.Bool("json") {
				b, err := json.Marshal(r)
				if err != nil {
					return err
				}
				fmt.Println(string(b))
			} else {
				status := "unknown"
				if r.Active != nil && *r.Active {
					status = "active"
				} else if r.Status != nil {
					status = *r.Status
				}
				fmt.Printf("%s\t%s\t%s\n", r.Did, status, r.Rev)
			}
		}

		if resp.Cursor == nil || *resp.Cursor == "" {
			break
		}
		cursor = *resp.Cursor
	}
	return nil
}

func runPDSAccountStatus(ctx context.Context, cmd *cli.Command) error {

	pdsHost := cmd.String("pds-host")
	if pdsHost == "" {
		return fmt.Errorf("need to provide PDS host URL")
	}

	didStr := cmd.Args().First()
	if didStr == "" {
		return fmt.Errorf("need to provide account DID as argument")
	}
	if cmd.Args().Len() != 1 {
		return fmt.Errorf("unexpected arguments")
	}

	did, err := syntax.ParseDID(didStr)
	if err != nil {
		return err
	}

	client := atclient.NewAPIClient(pdsHost)
	client.Headers.Set("User-Agent", userAgentString())

	r, err := comatproto.SyncGetRepoStatus(ctx, client, did.String())
	if err != nil {
		return err
	}

	if cmd.Bool("json") {
		b, err := json.Marshal(r)
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	} else {
		status := "unknown"
		if r.Active {
			status = "active"
		} else if r.Status != nil {
			status = *r.Status
		}
		rev := ""
		if r.Rev != nil {
			rev = *r.Rev
		}
		fmt.Printf("%s\t%s\t%s\n", r.Did, status, rev)
	}

	return nil
}
