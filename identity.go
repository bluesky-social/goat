package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"

	"github.com/urfave/cli/v3"
)

var cmdResolve = &cli.Command{
	Name:      "resolve",
	Usage:     "lookup identity metadata",
	ArgsUsage: `<at-identifier>`,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "did",
			Aliases: []string{"d"},
			Usage:   "just resolve to DID",
		},
	},
	Action: runResolve,
}

func runResolve(ctx context.Context, cmd *cli.Command) error {
	s := cmd.Args().First()
	if s == "" {
		return fmt.Errorf("need to provide account identifier as an argument")
	}

	atid, err := syntax.ParseAtIdentifier(s)
	if err != nil {
		return err
	}
	bdir := identity.BaseDirectory{
		PLCURL:    cmd.String("plc-host"),
		UserAgent: userAgentString(),
	}
	var raw json.RawMessage

	if atid.IsDID() {
		did, err := atid.AsDID()
		if err != nil {
			return err
		}
		if cmd.Bool("did") {
			fmt.Println(did)
			return nil
		}
		raw, err = bdir.ResolveDIDRaw(ctx, did)
		if err != nil {
			return err
		}
	} else {
		handle, err := atid.AsHandle()
		if err != nil {
			return err
		}
		did, err := bdir.ResolveHandle(ctx, handle)
		if err != nil {
			return err
		}
		if cmd.Bool("did") {
			fmt.Println(did)
			return nil
		}
		raw, err = bdir.ResolveDIDRaw(ctx, did)
		if err != nil {
			return err
		}

		var doc identity.DIDDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return err
		}
		ident := identity.ParseIdentity(&doc)
		decl, err := ident.DeclaredHandle()
		if err != nil {
			return err
		}
		if handle != decl {
			return fmt.Errorf("invalid handle")
		}
	}

	b, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(b))
	return nil
}
