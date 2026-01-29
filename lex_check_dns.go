package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"

	"github.com/urfave/cli/v3"
)

var cmdLexCheckDNS = &cli.Command{
	Name:        "check-dns",
	Usage:       "checks for any schemas missing DNS NSID resolution",
	Description: "Checks DNS resolution status for all local lexicons. If un-resolvable NSID groups are discovered, prints instructions on how to configure DNS resolution.\nOperates on entire ./lexicons/ directory unless specific files or directories are provided.",
	ArgsUsage:   `<file-or-dir>*`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "lexicons-dir",
			Value:   "lexicons/",
			Usage:   "base directory for project Lexicon files",
			Sources: cli.EnvVars("LEXICONS_DIR"),
		},
		&cli.StringFlag{
			Name:  "example-did",
			Usage: "lexicon publication DID for example text",
			Value: "did:web:lex.example.com",
		},
	},
	Action: runLexCheckDNS,
}

/*
- enumerate all local groups
- resolve and record any missing
- print DNS configuration instructions
*/
func runLexCheckDNS(ctx context.Context, cmd *cli.Command) error {

	// collect all NSID/path mappings
	localSchemas, err := collectSchemaJSON(cmd)
	if err != nil {
		return err
	}

	localGroups := map[string]bool{}
	for k := range localSchemas {
		g := nsidGroup(k)
		localGroups[g] = true
	}

	dir := identity.BaseDirectory{
		PLCURL:    cmd.String("plc-host"),
		UserAgent: userAgentString(),
	}
	missingGroups := []string{}
	for g := range localGroups {
		_, err := dir.ResolveNSID(ctx, syntax.NSID(g+"name"))
		if err != nil {
			missingGroups = append(missingGroups, g)
		}
	}

	if len(missingGroups) == 0 {
		fmt.Println("all lexicon schema NSIDs resolved successfully")
		return nil
	}
	sort.Strings(missingGroups)

	fmt.Println("Some lexicon NSIDs did not resolve via DNS:")
	fmt.Println("")
	for _, g := range missingGroups {
		fmt.Printf("    %s*\n", g)
	}
	fmt.Println("")
	fmt.Println("To make these resolve, add DNS TXT entries like:")
	fmt.Println("")
	for _, g := range missingGroups {
		nsid, err := syntax.ParseNSID(g + "name")
		if err != nil {
			return err
		}
		fmt.Printf("    _lexicon.%s\tTXT\t\"did=%s\"\n", nsid.Authority(), cmd.String("example-did"))
	}
	if !cmd.IsSet("example-did") {
		fmt.Println("")
		fmt.Println("(substituting your account DID for the example value)")
	}
	fmt.Println("")
	fmt.Println("Note that DNS management interfaces commonly require only the sub-domain parts of a name, not the full registered domain.")

	return nil
}
