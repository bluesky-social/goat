package main

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"github.com/bluesky-social/indigo/api/agnostic"
	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/atdata"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"

	"github.com/urfave/cli/v3"
)

var cmdLexPublish = &cli.Command{
	Name:        "publish",
	Usage:       "upload any new or updated lexicons",
	Description: "Publishes any new or updated local lexicons to the network, by creating schema records under the authenticated account.\nPublication requires a working AT network account, and appropriate DNS configuration. By default will only publish lexicons with DNS configured for the current account. See 'check-dns' command for configuration help, and '--skip-dns-check' to override (note that this can clobber any existing records).\nChecks schema status against live network and will not re-publish identical schemas, or update schemas by default (use '--update' to skip this check).\nOperates on entire ./lexicons/ directory unless specific files or directories are provided.",
	ArgsUsage:   `<file-or-dir>*`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "lexicons-dir",
			Value:   "lexicons/",
			Usage:   "base directory for project Lexicon files",
			Sources: cli.EnvVars("LEXICONS_DIR"),
		},
		&cli.StringFlag{
			Name:    "username",
			Usage:   "account identifier (handle or DID) for login",
			Sources: cli.EnvVars("GOAT_USERNAME", "ATP_USERNAME", "ATP_AUTH_USERNAME"),
		},
		&cli.StringFlag{
			Name:    "password",
			Aliases: []string{"p", "app-password"},
			Usage:   "account password (app password recommended) for login",
			Sources: cli.EnvVars("GOAT_PASSWORD", "ATP_PASSWORD", "ATP_AUTH_PASSWORD"),
		},
		&cli.BoolFlag{
			Name:  "skip-dns-check",
			Usage: "skip NSID DNS resolution match requirement",
		},
		&cli.BoolFlag{
			Name:    "update",
			Aliases: []string{"u"},
			Usage:   "update existing schema records",
		},
	},
	Action: runLexPublish,
}

/*
publish behavior:
- credentials are required
- load all relevant schemas
- filter no-change schemas
- optionally filter schemas where group DNS is not current account (control w/ arg)
- publish remaining schemas
*/
func runLexPublish(ctx context.Context, cmd *cli.Command) error {

	c, err := loginOrLoadAuthClient(ctx, cmd)
	if err != nil {
		return nil
	}

	if c.AccountDID == nil {
		return fmt.Errorf("require API client to have DID configured")
	}

	// collect all NSID/path mappings
	localSchemas, err := collectSchemaJSON(cmd)
	if err != nil {
		return err
	}
	remoteSchemas := map[syntax.NSID]json.RawMessage{}

	localGroups := map[string]bool{}
	allNSIDMap := map[syntax.NSID]bool{}
	for k := range localSchemas {
		g := nsidGroup(k)
		localGroups[g] = true
		allNSIDMap[k] = true
	}

	for g := range localGroups {
		if err := resolveLexiconGroup(ctx, cmd, g, &remoteSchemas); err != nil {
			return err
		}
	}

	dir := identity.BaseDirectory{
		PLCURL:    cmd.String("plc-host"),
		UserAgent: userAgentString(),
	}
	groupResolution := map[string]syntax.DID{}
	for g := range localGroups {
		did, err := dir.ResolveNSID(ctx, syntax.NSID(g+"name"))
		if err != nil {
			continue
		}
		groupResolution[g] = did
	}

	for k := range remoteSchemas {
		allNSIDMap[k] = true
	}
	allNSID := []string{}
	for k := range allNSIDMap {
		allNSID = append(allNSID, string(k))
	}
	sort.Strings(allNSID)

	for _, k := range allNSID {
		nsid := syntax.NSID(k)

		localJSON := localSchemas[nsid]
		remoteJSON := remoteSchemas[nsid]

		if localJSON == nil {
			continue
		}

		// skip if no change
		if remoteJSON != nil {
			if !cmd.Bool("update") {
				fmt.Printf(" 🟠 %s\n", nsid)
				continue
			}

			local, err := atdata.UnmarshalJSON(localJSON)
			if err != nil {
				return err
			}
			remote, err := atdata.UnmarshalJSON(remoteJSON)
			if err != nil {
				return err
			}
			delete(local, "$type")
			delete(remote, "$type")
			if reflect.DeepEqual(local, remote) {
				continue
			}
		}

		if !cmd.Bool("skip-dns-check") {
			g := nsidGroup(nsid)
			did, ok := groupResolution[g]
			if !ok || did != *c.AccountDID {
				fmt.Printf(" ⭕ %s\n", nsid)
				continue
			}
		}

		if err := publishSchema(ctx, c, nsid, localJSON); err != nil {
			return err
		}
		if remoteJSON == nil {
			fmt.Printf(" 🟢 %s\n", nsid)
		} else {
			fmt.Printf(" 🟣 %s\n", nsid)
		}
	}

	return nil
}

func publishSchema(ctx context.Context, c *atclient.APIClient, nsid syntax.NSID, schemaJSON json.RawMessage) error {

	d, err := atdata.UnmarshalJSON(schemaJSON)
	if err != nil {
		return err
	}
	d["$type"] = schemaNSID

	_, err = agnostic.RepoPutRecord(ctx, c, &agnostic.RepoPutRecord_Input{
		Collection: schemaNSID.String(),
		Repo:       c.AccountDID.String(),
		Record:     d,
		Rkey:       nsid.String(),
	})
	if err != nil {
		return err
	}

	return nil
}
