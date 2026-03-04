package main

import (
	"context"
	"encoding/json"
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

func printJSON(v any, color bool) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if color {
		b = colorizeJSON(b)
	}
	fmt.Println(string(b))
	return nil
}

func colorEnabled(cmd *cli.Command) bool {
	if cmd.Bool("no-color") {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func colorizeJSON(src []byte) []byte {
	const (
		reset  = "\033[0m"
		blue   = "\033[1;34m" // keys
		green  = "\033[32m"   // string values
		cyan   = "\033[36m"   // numbers
		yellow = "\033[33m"   // booleans and null
	)

	color := func(out []byte, c string, tok []byte) []byte {
		out = append(out, c...)
		out = append(out, tok...)
		return append(out, reset...)
	}

	out := make([]byte, 0, len(src)*2)
	for i := 0; i < len(src); {
		ch := src[i]
		switch {
		case ch <= ' ' || ch == '{' || ch == '}' || ch == '[' || ch == ']' || ch == ',' || ch == ':':
			out = append(out, ch)
			i++
		case ch == '"':
			end := scanString(src, i)
			if isKey(src, end) {
				out = color(out, blue, src[i:end])
			} else {
				out = color(out, green, src[i:end])
			}
			i = end
		default:
			j := i + 1
			for j < len(src) && src[j] != ',' && src[j] != '}' && src[j] != ']' && src[j] > ' ' {
				j++
			}
			tok := src[i:j]
			if tok[0] == 't' || tok[0] == 'f' || tok[0] == 'n' {
				out = color(out, yellow, tok)
			} else {
				out = color(out, cyan, tok)
			}
			i = j
		}
	}
	return out
}

// get the index just past the closing quote.
func scanString(src []byte, start int) int {
	i := start + 1
	for i < len(src) && src[i] != '"' {
		if src[i] == '\\' {
			i++
		}
		i++
	}
	return i + 1
}

func isKey(src []byte, pos int) bool {
	for pos < len(src) && src[pos] == ' ' {
		pos++
	}
	return pos < len(src) && src[pos] == ':'
}
