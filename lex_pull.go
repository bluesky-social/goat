package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path"

	"github.com/bluesky-social/indigo/api/agnostic"
	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/atdata"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/lexicon"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"tangled.org/bnewbold.net/cobalt/netclient"

	"github.com/urfave/cli/v3"
)

var cmdLexPull = &cli.Command{
	Name:        "pull",
	Usage:       "fetch (or update) lexicon schemas to local directory",
	Description: "Resolves and downloads lexicons, and saves as JSON files in local directory.\nPatterns can be full NSIDs, or \"groups\" ending in '.' or '.*'. Does not recursively fetch sub-groups.\nUse 'status' command to check for missing or out-of-date lexicons which need fetching.",
	ArgsUsage:   `<nsid-pattern>+`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "lexicons-dir",
			Value:   "lexicons/",
			Usage:   "base directory for project Lexicon files",
			Sources: cli.EnvVars("LEXICONS_DIR"),
		},
		&cli.BoolFlag{
			Name:    "update",
			Aliases: []string{"u"},
			Usage:   "overwrite any existing local files",
		},
		&cli.StringFlag{
			Name:    "output-dir",
			Aliases: []string{"o"},
			Usage:   "write schema files to specific directory",
			Sources: cli.EnvVars("LEXICONS_DIR"),
		},
	},
	Action: runLexPull,
}

func runLexPull(ctx context.Context, cmd *cli.Command) error {
	if !cmd.Args().Present() {
		cli.ShowSubcommandHelpAndExit(cmd, 1)
	}

	for _, p := range cmd.Args().Slice() {

		group, err := ParseNSIDGroup(p)
		if nil == err {
			if err := pullLexiconGroup(ctx, cmd, group); err != nil {
				return err
			}
			continue
		}

		nsid, err := syntax.ParseNSID(p)
		if err != nil {
			return fmt.Errorf("invalid Lexicon NSID pattern: %s", p)
		}
		if err := pullLexicon(ctx, cmd, nsid); err != nil {
			return err
		}
	}
	return nil
}

func pullLexicon(ctx context.Context, cmd *cli.Command, nsid syntax.NSID) error {

	fpath := pathForNSID(cmd, nsid)
	if !cmd.Bool("update") {
		_, err := os.Stat(fpath)
		if err == nil {
			fmt.Printf(" 🟣 %s\n", nsid)
			return nil
		}
	}

	// TODO: common net client
	netc := netclient.NewNetClient()
	dir := identity.BaseDirectory{
		PLCURL:    cmd.String("plc-host"),
		UserAgent: userAgentString(),
	}
	did, err := dir.ResolveNSID(ctx, nsid)
	if err != nil {
		return fmt.Errorf("failed to resolve NSID %s: %w", nsid, err)
	}

	var rec json.RawMessage
	cid, err := netc.GetRecord(ctx, did, schemaNSID, syntax.RecordKey(nsid), &rec)
	if err != nil {
		return err
	}
	slog.Debug("fetched NSID schema record", "nsid", nsid, "cid", cid)

	if err := writeLexiconFile(ctx, cmd, nsid, fpath, rec); err != nil {
		return err
	}
	fmt.Printf(" 🟢 %s\n", nsid)
	return nil
}

func writeLexiconFile(ctx context.Context, cmd *cli.Command, nsid syntax.NSID, fpath string, rec json.RawMessage) error {

	var sf lexicon.SchemaFile
	err := json.Unmarshal(rec, &sf)
	if err == nil {
		err = sf.FinishParse()
	}
	// NOTE: not calling CheckSchema()
	if err != nil {
		return fmt.Errorf("schema record syntax invalid (%s): %w", nsid, err)
	}

	// ensure (nested) directory exists
	if err := os.MkdirAll(path.Dir(fpath), 0755); err != nil {
		return err
	}

	// remove $type (from record)
	d, err := atdata.UnmarshalJSON(rec)
	if err != nil {
		return err
	}
	delete(d, "$type")

	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')

	if err := os.WriteFile(fpath, b, 0666); err != nil {
		return err
	}

	slog.Debug("wrote NSID schema record to disk", "nsid", nsid, "path", fpath)
	return nil
}

func pullLexiconGroup(ctx context.Context, cmd *cli.Command, group string) error {

	// TODO: netclient support for listing records
	dir := identity.BaseDirectory{}
	did, err := dir.ResolveNSID(ctx, syntax.NSID(group+"name"))
	if err != nil {
		return err
	}
	ident, err := dir.LookupDID(ctx, did)
	if err != nil {
		return err
	}
	c := atclient.NewAPIClient(ident.PDSEndpoint())

	cursor := ""
	for {
		// collection string, cursor string, limit int64, repo string, reverse bool
		resp, err := agnostic.RepoListRecords(ctx, c, schemaNSID.String(), cursor, 100, ident.DID.String(), false)
		if err != nil {
			return err
		}
		for _, rec := range resp.Records {
			aturi, err := syntax.ParseATURI(rec.Uri)
			if err != nil {
				return err
			}
			nsid, err := syntax.ParseNSID(aturi.RecordKey().String())
			if err != nil {
				slog.Warn("ignoring invalid schema NSID", "did", ident.DID, "rkey", aturi.RecordKey())
				continue
			}
			if nsidGroup(nsid) != group {
				// ignoring other NSIDs
				continue
			}
			if rec.Value == nil {
				return fmt.Errorf("missing record value: %s", nsid)
			}

			fpath := pathForNSID(cmd, nsid)
			if !cmd.Bool("update") {
				_, err := os.Stat(fpath)
				if err == nil {
					fmt.Printf(" 🟣 %s\n", nsid)
					continue
				}
			}
			if err := writeLexiconFile(ctx, cmd, nsid, fpath, *rec.Value); err != nil {
				return nil
			}
			fmt.Printf(" 🟢 %s\n", nsid)
		}
		if resp.Cursor != nil && *resp.Cursor != "" {
			cursor = *resp.Cursor
		} else {
			break
		}
	}
	return nil
}
