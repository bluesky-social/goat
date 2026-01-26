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
