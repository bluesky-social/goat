package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/syntax"

	"github.com/urfave/cli/v3"
	"golang.org/x/term"
)

var cmdXrpc = &cli.Command{
	Name:        "xrpc",
	Usage:       "call remote XRPC (HTTP API) endpoints",
	ArgsUsage:   `<method> <service> <endpoint> [args...]`,
	Description: "Flexible tool for calling arbitrary XRPC endpoints on remote services. Supports multiple types of service endpoint resolution and auth.\n'method' is the HTTP/XRPC method type (eg 'query' or 'procedure').\n'service' identifies the remote host. Provide an HTTP/HTTPS base URL for direct connections, or a service DID reference for authenticated PDS proxying. Provide '@pds' for authenticated requests to the current account PDS.\n'endpoint' is an NSID identifying the API endpoint.\nAdditional args follow HTTPie CLI syntax: 'key==value' sets a query param, 'key=value' sets a JSON request body string field; 'key:=123' sets a non-string JSON request body field; 'key:value' sets an HTTP request header; '-' reads a request body from stdin (may need to specify Content-Type header)",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "admin-password",
			Usage:   "admin password (for admin auth calls)",
			Sources: cli.EnvVars("ADMIN_PASSWORD", "ATP_AUTH_ADMIN_PASSWORD"),
		},
	},
	Action: runXrpc,
}

func runXrpc(ctx context.Context, cmd *cli.Command) error {

	if cmd.Args().Len() < 3 {
		return fmt.Errorf("most provide at least service and NSID args")
	}

	method := cmd.Args().Get(0)
	switch strings.ToLower(method) {
	case "get", "query":
		method = atclient.MethodQuery
	case "post", "procedure":
		method = atclient.MethodProcedure
	default:
		return fmt.Errorf("unknown XRPC method type: %s", method)
	}

	rawService := cmd.Args().Get(1)

	endpoint, err := syntax.ParseNSID(cmd.Args().Get(2))
	if err != nil {
		return fmt.Errorf("endpoint arg must be an NSID: %w", err)
	}

	var client *atclient.APIClient

	if rawService == "@pds" {
		// authenticated PDS mode
		client, err = loadAuthClient(ctx, cmd)
		if err != nil {
			return fmt.Errorf("PDS API requests require session: %w", err)
		}
	} else if strings.Contains(rawService, "://") {
		if cmd.IsSet("admin-password") {
			// admin auth mode
			client = atclient.NewAdminClient(rawService, cmd.String("admin-password"))
		} else {
			// public API endpoint mode
			client = atclient.NewAPIClient(rawService)
		}
	} else {
		if err := parseDIDRef(rawService); err != nil {
			return fmt.Errorf("unknown service type: %s", rawService)
		}

		// TODO: raw service auth mode would go here

		// PDS service proxy mode
		client, err = loadAuthClient(ctx, cmd)
		if err != nil {
			return fmt.Errorf("PDS proxied requests require session: %w", err)
		}
		client = client.WithService(rawService)
	}

	params := make(url.Values)
	reqBody := make(map[string]any)
	reqStdin := false

	req := atclient.NewAPIRequest(method, endpoint, nil)
	req.Headers.Set("Accept", "application/json")

	for i := range cmd.Args().Len() - 3 {
		arg := cmd.Args().Get(i + 3)
		if arg == "-" {
			reqStdin = true
			continue
		}
		// TODO: all this pattern matching is informal and not correct in corner-cases
		if strings.HasPrefix(arg, "@") {
			p, _ := strings.CutPrefix(arg, "@")
			b, err := os.ReadFile(p)
			if err != nil {
				return fmt.Errorf("could not read request body file: %w", err)
			}
			if err := json.NewDecoder(bytes.NewReader(b)).Decode(&reqBody); err != nil {
				return fmt.Errorf("invalid JSON file contents: %w", err)
			}
		} else if strings.Contains(arg, "==") {
			parts := strings.SplitN(arg, "==", 2)
			if len(parts[0]) == 0 {
				return fmt.Errorf("empty query parameter name")
			}
			params.Add(parts[0], parts[1])
		} else if strings.Contains(arg, ":=") {
			parts := strings.SplitN(arg, ":=", 2)
			if len(parts[0]) == 0 {
				return fmt.Errorf("empty body field name")
			}
			var val any
			if err := json.NewDecoder(bytes.NewReader([]byte(parts[1]))).Decode(&val); err != nil {
				return fmt.Errorf("invalid non-string field value: %w", err)
			}
			reqBody[parts[0]] = val
		} else if strings.Contains(arg, ":") {
			parts := strings.SplitN(arg, ":", 2)
			if len(parts[0]) == 0 {
				return fmt.Errorf("empty request header name")
			}
			req.Headers.Set(parts[0], parts[1])
		} else if strings.Contains(arg, "=") {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts[0]) == 0 {
				return fmt.Errorf("empty body field name")
			}
			reqBody[parts[0]] = parts[1]
		} else {
			return fmt.Errorf("unhandled arg syntax: %s", arg)
		}
	}

	if method == atclient.MethodProcedure {
		if req.Headers.Get("Content-Type") == "" {
			req.Headers.Set("Content-Type", "application/json")
		}
		if reqStdin {
			req.Body = os.Stdin
		} else {
			bodyJSON, err := json.Marshal(reqBody)
			if err != nil {
				return err
			}
			req.Body = bytes.NewReader(bodyJSON)
		}
	}

	if len(params) > 0 {
		req.QueryParams = params
	}

	resp, err := client.Do(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
		var eb atclient.ErrorBody
		if err := json.NewDecoder(resp.Body).Decode(&eb); err != nil {
			return &atclient.APIError{StatusCode: resp.StatusCode}
		}
		return eb.APIError(resp.StatusCode)
	}

	if term.IsTerminal(int(os.Stdout.Fd())) {
		useColor := colorEnabled(cmd)
		statusFmt, headerFmt := "%s %s\n", "%s: %s\n"
		if useColor {
			statusFmt, headerFmt = "\033[32m%s %s\033[0m\n", "\033[36m%s\033[0m: %s\n"
		}
		fmt.Printf(statusFmt, resp.Proto, resp.Status)
		for name, vals := range resp.Header {
			for _, v := range vals {
				fmt.Printf(headerFmt, name, v)
			}
		}
		fmt.Println()
	}

	var respBody json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return fmt.Errorf("failed decoding JSON response body: %w", err)
	}

	return printJSON(respBody, colorEnabled(cmd))
}
