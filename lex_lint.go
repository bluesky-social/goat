package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/bluesky-social/indigo/atproto/lexicon"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/lex/lexlint"

	"github.com/urfave/cli/v3"
)

var (
	// internal error used to set non-zero return code (but not print separately)
	ErrLintFailures = errors.New("linting issues detected")
)

var cmdLexLint = &cli.Command{
	Name:        "lint",
	Usage:       "check schema syntax, best practices, and style",
	Description: "Parses lexicon schemas (JSON files) from disk and checks various style and best practice rules. Summarizes status for each file.\nOperates on entire ./lexicons/ directory unless specific files or directories are provided.",
	ArgsUsage:   `<file-or-dir>*`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "lexicons-dir",
			Value:   "lexicons/",
			Usage:   "base directory for project Lexicon files",
			Sources: cli.EnvVars("LEXICONS_DIR"),
		},
		&cli.BoolFlag{
			Name:  "json",
			Usage: "output structured JSON",
		},
	},
	Action: runLexLint,
}

func runLexLint(ctx context.Context, cmd *cli.Command) error {

	// enumerate lexicon JSON file paths
	filePaths, err := collectPaths(cmd)
	if err != nil {
		return err
	}

	// TODO: load up entire directory in to a catalog? or have a "linter" struct?

	slog.Debug("starting lint run")
	anyFailures := false
	for _, fp := range filePaths {
		err = lintFilePath(ctx, cmd, fp)
		if err != nil {
			if err == ErrLintFailures {
				anyFailures = true
			} else {
				return err
			}
		}
	}
	if anyFailures {
		return ErrLintFailures
	}
	return nil
}

func lintFilePath(ctx context.Context, cmd *cli.Command, p string) error {
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}

	// parse file regularly
	// TODO: use json/v2 when available for case-sensitivity
	var sf lexicon.SchemaFile

	// two-part parsing before looking at errors
	err = json.Unmarshal(b, &sf)
	if err == nil {
		err = sf.FinishParse()
	}
	if err != nil {
		iss := lexlint.LintIssue{
			FilePath: p,
			//NSID
			LintLevel:       "error",
			LintName:        "schema-json-parse",
			LintDescription: "parsing schema JSON file",
			Message:         err.Error(),
		}
		if cmd.Bool("json") {
			b, err := json.Marshal(iss)
			if err != nil {
				return nil
			}
			fmt.Println(string(b))
		} else {
			fmt.Printf(" 🔴 %s\n", p)
			fmt.Printf("    [%s]: %s\n", iss.LintName, iss.Message)
		}
		return ErrLintFailures
	}

	issues := lexlint.LintSchemaFile(&sf)
	for i := range issues {
		// add path as context
		issues[i].FilePath = p
	}

	// check for unknown fields (more strict, as a lint/warning)
	var unknownSF lexicon.SchemaFile
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&unknownSF); err != nil {
		issues = append(issues, lexlint.LintIssue{
			FilePath:        p,
			NSID:            syntax.NSID(sf.ID),
			LintLevel:       "warn",
			LintName:        "unexpected-field",
			LintDescription: "schema JSON contains unexpected data",
			Message:         err.Error(),
		})
	}

	if cmd.Bool("json") {
		for _, iss := range issues {
			b, err := json.Marshal(iss)
			if err != nil {
				return nil
			}
			fmt.Println(string(b))
		}
	} else {
		if len(issues) == 0 {
			fmt.Printf(" 🟢 %s\n", p)
		} else {
			fmt.Printf(" 🟡 %s\n", p)
			for _, iss := range issues {
				fmt.Printf("    [%s]: %s\n", iss.LintName, iss.Message)
			}
		}
	}
	if len(issues) > 0 {
		return ErrLintFailures
	}
	return nil
}
