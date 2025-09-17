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

func getConfiguredDirectory(cmd *cli.Command) identity.Directory {
	plcURL := cmd.String("plc-url")
	pdsURL := cmd.String("pds-url")

	if plcURL == "" && pdsURL == "" {
		return identity.DefaultDirectory()
	}

	baseDir := &identity.BaseDirectory{}
	if plcURL != "" {
		baseDir.PLCURL = plcURL
	}
	if pdsURL != "" {
		// Note: BaseDirectory doesn't have a PDS field, but we can handle this
		// in specific commands that need PDS override
	}
	return baseDir
}

func resolveIdent(ctx context.Context, arg string) (*identity.Identity, error) {
	// This function is kept for backward compatibility but should be replaced
	// with resolveIdentCmd where possible
	id, err := syntax.ParseAtIdentifier(arg)
	if err != nil {
		return nil, err
	}

	dir := identity.DefaultDirectory()
	return dir.Lookup(ctx, *id)
}

func resolveIdentCmd(ctx context.Context, cmd *cli.Command, arg string) (*identity.Identity, error) {
	id, err := syntax.ParseAtIdentifier(arg)
	if err != nil {
		return nil, err
	}

	dir := getConfiguredDirectory(cmd)
	return dir.Lookup(ctx, *id)
}

func resolveIdentWithDirectory(ctx context.Context, arg string, dir identity.Directory) (*identity.Identity, error) {
	id, err := syntax.ParseAtIdentifier(arg)
	if err != nil {
		return nil, err
	}

	return dir.Lookup(ctx, *id)
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

// returns a pointer because that is what xrpc.Client expects
func userAgent() *string {
	s := fmt.Sprintf("goat/%s", versioninfo.Short())
	return &s
}
