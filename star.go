package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"tangled.org/bnewbold.net/cobalt/star"

	"github.com/hyphacoop/go-dasl/drisl"
	"github.com/urfave/cli/v3"
)

var cmdStar = &cli.Command{
	Name:  "star",
	Usage: "commands for DASL STAR files",
	Flags: []cli.Flag{},
	Commands: []*cli.Command{
		&cli.Command{
			Name:      "meta",
			Usage:     "output meta header as JSON",
			ArgsUsage: `<star-file>`,
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:  "type",
					Usage: "only print the meta '$type' string field",
				},
			},
			Action: runStarMeta,
		},
		&cli.Command{
			Name:      "cat",
			Usage:     "output meta header and all pairs as JSON lines",
			ArgsUsage: `<star-file>`,
			Flags:     []cli.Flag{},
			Action:    runStarCat,
		},
	},
}

func runStarMeta(ctx context.Context, cmd *cli.Command) error {
	starPath := cmd.Args().First()
	if starPath == "" {
		return fmt.Errorf("need to provide path to STAR file as argument")
	}
	fi, err := os.Open(starPath)
	if err != nil {
		return fmt.Errorf("failed to open STAR file: %w", err)
	}
	defer fi.Close()

	// NOTE: default block size
	reader := star.NewReader(fi)

	if cmd.Bool("type") {
		t, err := reader.MetaType()
		if err != nil {
			return err
		}
		fmt.Println(t)
		return nil
	}

	var meta map[string]any
	if err := reader.UnmarshalMeta(&meta); err != nil {
		return err
	}

	b, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))

	// NOTE: doesn't read or verify the rest of the file
	return nil
}

func runStarCat(ctx context.Context, cmd *cli.Command) error {
	starPath := cmd.Args().First()
	if starPath == "" {
		return fmt.Errorf("need to provide path to STAR file as argument")
	}
	fi, err := os.Open(starPath)
	if err != nil {
		return fmt.Errorf("failed to open STAR file: %w", err)
	}
	defer fi.Close()

	// NOTE: default block size
	reader := star.NewReader(fi)
	enc := json.NewEncoder(os.Stdout)

	var meta map[string]any
	if err := reader.UnmarshalMeta(&meta); err != nil {
		return err
	}
	if err := enc.Encode(meta); err != nil {
		return err
	}

	for reader.Scan() {
		key, err := reader.KeyString()
		if err != nil {
			return err
		}
		var value map[string]any
		if err := drisl.Unmarshal(reader.Value(), &value); err != nil {
			return fmt.Errorf("parsing STAR value CBOR: %w", err)
		}
		line := map[string]any{
			"key":   key,
			"value": value,
		}
		if err := enc.Encode(line); err != nil {
			return err
		}
	}
	return reader.Err()
}
