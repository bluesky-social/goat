package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"tangled.org/bnewbold.net/cobalt/car"

	"github.com/hyphacoop/go-dasl/drisl"
	"github.com/urfave/cli/v3"
)

var cmdCar = &cli.Command{
	Name:  "car",
	Usage: "commands for DASL CAR files",
	Flags: []cli.Flag{},
	Commands: []*cli.Command{
		&cli.Command{
			Name:      "header",
			Usage:     "output header object as JSON",
			ArgsUsage: `<car-file>`,
			Action: runCarHeader,
		},
		&cli.Command{
			Name:      "cat",
			Usage:     "output all blocks as JSON lines (does not include header)",
			ArgsUsage: `<car-file>`,
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:  "with-cid",
					Usage: "includes a wrapper object with the CID value",
				},
			},
			Action:    runCarCat,
		},
	},
}

func runCarHeader(ctx context.Context, cmd *cli.Command) error {
	carPath := cmd.Args().First()
	if carPath == "" {
		return fmt.Errorf("need to provide path to CAR file as argument")
	}
	fi, err := os.Open(carPath)
	if err != nil {
		return fmt.Errorf("failed to open CAR file: %w", err)
	}
	defer fi.Close()

	// NOTE: default block size
	reader := car.NewReader(fi)
	hdr, err := reader.Header()
	if err != nil {
		return err
	}

	b, err := json.MarshalIndent(hdr, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))

	// NOTE: doesn't read or verify the rest of the file
	return nil
}

func runCarCat(ctx context.Context, cmd *cli.Command) error {
	carPath := cmd.Args().First()
	if carPath == "" {
		return fmt.Errorf("need to provide path to CAR file as argument")
	}
	fi, err := os.Open(carPath)
	if err != nil {
		return fmt.Errorf("failed to open CAR file: %w", err)
	}
	defer fi.Close()

	// NOTE: default block size
	reader := car.NewReader(fi)
	enc := json.NewEncoder(os.Stdout)

	for reader.Scan() {
		var value map[string]any
		if err := drisl.Unmarshal(reader.Data(), &value); err != nil {
			return fmt.Errorf("parsing CAR value CBOR: %w", err)
		}
		if cmd.Bool("with-cid") {
			line := map[string]any{
				"cid": reader.CID().String(),
				"data": value,
			}
			if err := enc.Encode(line); err != nil {
				return err
			}
		} else {
			if err := enc.Encode(value); err != nil {
				return err
			}
		}
	}
	return reader.Err()
}
