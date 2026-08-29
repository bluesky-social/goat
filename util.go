package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"

	"github.com/earthboundkid/versioninfo/v2"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
	"github.com/urfave/cli/v3"
)

// helper to configure identity directory with PLC host (from env var) and user agent
func configDirectory(plcHost string) identity.Directory {
	dir := identity.DefaultDirectory()

	cdir, ok := dir.(*identity.CacheDirectory)
	if ok {
		bdir, ok := cdir.Inner.(*identity.BaseDirectory)
		if ok {
			bdir.UserAgent = userAgentString()
			if plcHost != "" {
				bdir.PLCURL = plcHost
			}
		}
	}
	return dir
}

func resolveIdent(ctx context.Context, cmd *cli.Command, arg string) (*identity.Identity, error) {
	id, err := syntax.ParseAtIdentifier(arg)
	if err != nil {
		return nil, err
	}

	dir := configDirectory(cmd.String("plc-host"))
	return dir.Lookup(ctx, id)
}

func resolveToDID(ctx context.Context, cmd *cli.Command, s string) (syntax.DID, error) {
	atid, err := syntax.ParseAtIdentifier(s)
	if err != nil {
		return "", err
	}
	if atid.IsDID() {
		did, _ := atid.AsDID()
		return did, nil
	}
	hdl, _ := atid.AsHandle()
	bdir := identity.BaseDirectory{
		PLCURL:    cmd.String("plc-host"),
		UserAgent: userAgentString(),
	}
	return bdir.ResolveHandle(ctx, hdl)
}

const stdIOPath = "-"

var portableFilenameReplacer = strings.NewReplacer(
	"<", "_",
	">", "_",
	":", "_",
	`"`, "_",
	"/", "_",
	`\`, "_",
	"|", "_",
	"?", "_",
	"*", "_",
)

func portableFilenameComponent(name string) string {
	name = portableFilenameReplacer.Replace(name)
	name = strings.TrimRight(name, ". ")
	if name == "" {
		return "_"
	}

	base := name
	if idx := strings.IndexRune(base, '.'); idx >= 0 {
		base = base[:idx]
	}
	switch strings.ToUpper(base) {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return "_" + name
	}
	return name
}

func getFileOrStdin(path string) (io.Reader, error) {
	if path == stdIOPath {
		return os.Stdin, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func getFileOrStdout(path string) (io.WriteCloser, error) {
	if path == stdIOPath {
		return os.Stdout, nil
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func configLogger(cmd *cli.Command, writer io.Writer) *slog.Logger {
	var level slog.Level
	switch strings.ToLower(cmd.String("log-level")) {
	case "error":
		level = slog.LevelError
	case "warn":
		level = slog.LevelWarn
	case "info":
		level = slog.LevelInfo
	case "debug":
		level = slog.LevelDebug
	default:
		level = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level: level,
	}))
	slog.SetDefault(logger)
	return logger
}

func userAgentString() string {
	return fmt.Sprintf("goat/%s", versioninfo.Short())
}

// attempts to parse a DID-and-reference string
func parseDIDRef(raw string) error {
	parts := strings.SplitN(raw, "#", 3)
	if len(parts) != 2 {
		return fmt.Errorf("not a DID-and-fragment")
	}
	_, err := syntax.ParseDID(parts[0])
	if err != nil {
		return err
	}
	if len(parts[1]) == 0 {
		return fmt.Errorf("empty fragment")
	}
	// TODO: more syntax checks on fragment
	return nil
}

func computeRawCID(b []byte) (*cid.Cid, error) {
	builder := cid.NewPrefixV1(cid.Raw, multihash.SHA2_256)
	c, err := builder.Sum(b)
	if err != nil {
		return nil, err
	}
	return &c, err
}
