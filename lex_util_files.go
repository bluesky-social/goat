package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bluesky-social/indigo/atproto/lexicon"
	"github.com/bluesky-social/indigo/atproto/syntax"

	"github.com/urfave/cli/v3"
)

func pathForNSID(cmd *cli.Command, nsid syntax.NSID) string {

	odir := cmd.String("output-dir")
	if odir != "" {
		return path.Join(odir, nsid.Name()+".json")
	}

	base := cmd.String("lexicons-dir")
	sub := strings.ReplaceAll(nsid.String(), ".", "/")
	return path.Join(base, sub+".json")
}

// parses through directories and files provided as CLI args, and returns a list of recursively enumerated .json files
func collectPaths(cmd *cli.Command) ([]string, error) {

	paths := cmd.Args().Slice()
	if !cmd.Args().Present() {
		paths = []string{cmd.String("lexicons-dir")}
		_, err := os.Stat(paths[0])
		if err != nil {
			return nil, fmt.Errorf("no path arguments specified and default lexicon directory not found")
		}
	}

	filePaths := []string{}

	for _, p := range paths {
		finfo, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("failed reading path %s: %w", p, err)
		}
		if finfo.IsDir() {
			if err := filepath.WalkDir(p, func(fp string, d fs.DirEntry, err error) error {
				if d.IsDir() || path.Ext(fp) != ".json" {
					return nil
				}
				filePaths = append(filePaths, fp)
				return nil
			}); err != nil {
				return nil, err
			}
			continue
		}
		filePaths = append(filePaths, p)
	}

	sort.Strings(filePaths)
	return filePaths, nil
}

// parses through directories and files provided as CLI args, and returns broadly inclusive lexicon catalog.
//
// includes 'lexicons-dir', which may be broader than collectPaths() would return
func collectCatalog(cmd *cli.Command) (lexicon.Catalog, error) {

	cat := lexicon.NewBaseCatalog()

	lexDir := cmd.String("lexicons-dir")
	paths := cmd.Args().Slice()
	if !cmd.Args().Present() {
		_, err := os.Stat(lexDir)
		if err != nil {
			return nil, fmt.Errorf("no path arguments specified and default lexicon directory not found")
		}
	}

	// load lexicon dir (recursively)
	ldinfo, err := os.Stat(lexDir)
	if err == nil && ldinfo.IsDir() {
		if err := cat.LoadDirectory(lexDir); err != nil {
			return nil, err
		}
	}

	for _, p := range paths {

		if strings.HasPrefix(p, lexDir) {
			// if path is under lexdir, we have already loaded, so skip
			// NOTE: this isn't particularly reliable
			continue
		}

		finfo, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("failed reading path %s: %w", p, err)
		}
		if finfo.IsDir() {
			if p != lexDir {
				if err := cat.LoadDirectory(p); err != nil {
					return nil, err
				}
			}
			continue
		}
		if !finfo.Mode().IsRegular() && path.Ext(p) == ".json" {
			// load schema file in to catalog
			f, err := os.Open(p)
			if err != nil {
				return nil, err
			}
			defer func() { _ = f.Close() }()

			b, err := io.ReadAll(f)
			if err != nil {
				return nil, err
			}
			var sf lexicon.SchemaFile
			if err := json.Unmarshal(b, &sf); err != nil {
				return nil, err
			}
			if err := cat.AddSchemaFile(sf); err != nil {
				return nil, err
			}
		}
	}
	return &cat, nil
}

func loadSchemaJSON(fpath string) (syntax.NSID, *json.RawMessage, error) {
	b, err := os.ReadFile(fpath)
	if err != nil {
		return "", nil, err
	}

	// parse file to check for errors
	// TODO: use json/v2 when available for case-sensitivity
	var sf lexicon.SchemaFile
	err = json.Unmarshal(b, &sf)
	if err == nil {
		err = sf.FinishParse()
	}
	if err == nil {
		err = sf.CheckSchema()
	}
	if err != nil {
		return "", nil, err
	}

	var rec json.RawMessage
	if err := json.Unmarshal(b, &rec); err != nil {
		return "", nil, err
	}
	return syntax.NSID(sf.ID), &rec, nil
}

func collectSchemaJSON(cmd *cli.Command) (map[syntax.NSID]json.RawMessage, error) {
	schemas := map[syntax.NSID]json.RawMessage{}

	filePaths, err := collectPaths(cmd)
	if err != nil {
		return nil, err
	}

	for _, fp := range filePaths {
		nsid, rec, err := loadSchemaJSON(fp)
		if err != nil {
			return nil, err
		}
		schemas[nsid] = *rec
	}
	return schemas, nil
}
