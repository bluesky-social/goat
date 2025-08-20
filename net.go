package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bluesky-social/indigo/api/agnostic"
	"github.com/bluesky-social/indigo/atproto/client"
	"github.com/bluesky-social/indigo/atproto/data"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

func fetchRecord(ctx context.Context, ident identity.Identity, aturi syntax.ATURI) (map[string]any, error) {

	slog.Debug("fetching record", "did", ident.DID.String(), "collection", aturi.Collection().String(), "rkey", aturi.RecordKey().String())
	c := client.NewAPIClient(ident.PDSEndpoint())
	c.Headers.Set("User-Agent", userAgentString())
	if c.Host == "" {
		return nil, fmt.Errorf("no PDS endpoint for identity")
	}
	resp, err := agnostic.RepoGetRecord(ctx, c, "", aturi.Collection().String(), ident.DID.String(), aturi.RecordKey().String())
	if err != nil {
		return nil, err
	}

	if nil == resp.Value {
		return nil, fmt.Errorf("empty record in response")
	}
	record, err := data.UnmarshalJSON(*resp.Value)
	if err != nil {
		return nil, fmt.Errorf("fetched record was invalid data: %w", err)
	}
	return record, nil
}
