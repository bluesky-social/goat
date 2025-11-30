package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/bluesky-social/indigo/api/agnostic"
	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/lexicon"
	"github.com/bluesky-social/indigo/atproto/syntax"

	"github.com/urfave/cli/v3"
)

var (
	schemaNSID = syntax.NSID("com.atproto.lexicon.schema")
)

func nsidGroup(nsid syntax.NSID) string {
	parts := strings.Split(string(nsid), ".")
	g := strings.Join(parts[0:len(parts)-1], ".") + "."
	return g
}

// Checks if a string is a valid NSID group pattern, which is a partial NSID ending in '.' or '.*'
func ParseNSIDGroup(raw string) (string, error) {
	if strings.HasSuffix(raw, ".*") {
		raw = raw[:len(raw)-1]
	}
	if !strings.HasSuffix(raw, ".") {
		return "", fmt.Errorf("not an NSID group pattern")
	}
	_, err := syntax.ParseNSID(raw + "name")
	if err != nil {
		return "", fmt.Errorf("not an NSID group pattern")
	}
	return raw, nil
}

// helper which runs a comparison function across local and remote schemas, based on 'cmd' configuration
func runComparisons(ctx context.Context, cmd *cli.Command, comp func(ctx context.Context, cmd *cli.Command, nsid syntax.NSID, localJSON, remoteJSON json.RawMessage) error) error {

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

	for k := range remoteSchemas {
		allNSIDMap[k] = true
	}
	allNSID := []string{}
	for k := range allNSIDMap {
		allNSID = append(allNSID, string(k))
	}
	sort.Strings(allNSID)

	anyFailures := false
	for _, k := range allNSID {
		nsid := syntax.NSID(k)
		if err := comp(ctx, cmd, nsid, localSchemas[nsid], remoteSchemas[nsid]); err != nil {
			if err != ErrLintFailures {
				return err
			}
			anyFailures = true
		}
	}

	if anyFailures {
		return ErrLintFailures
	}
	return nil
}

// helper which resolves and fetches all lexicon schemas (as JSON), storing them in provided map
func resolveLexiconGroup(ctx context.Context, cmd *cli.Command, group string, remote *map[syntax.NSID]json.RawMessage) error {

	slog.Debug("resolving schemas for NSID group", "group", group)

	// TODO: netclient support for listing records
	dir := identity.BaseDirectory{}
	did, err := dir.ResolveNSID(ctx, syntax.NSID(group+"name"))
	if err != nil {
		// if NSID isn't registered, just skip comparison
		slog.Warn("skipping NSID pattern which did not resolve", "group", group)
		return nil
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

			// parse file to check for errors
			// TODO: use json/v2 when available for case-sensitivity
			var sf lexicon.SchemaFile
			err = json.Unmarshal(*rec.Value, &sf)
			if err == nil {
				err = sf.FinishParse()
			}
			if err == nil {
				err = sf.CheckSchema()
			}
			if err != nil {
				return fmt.Errorf("invalid lexicon schema record (%s): %w", nsid, err)
			}

			(*remote)[nsid] = *rec.Value

		}
		if resp.Cursor != nil && *resp.Cursor != "" {
			cursor = *resp.Cursor
		} else {
			break
		}
	}
	return nil
}
