package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/urfave/cli/v3"
)

var cmdRelayAdmin = &cli.Command{
	Name:  "admin",
	Usage: "comands for relay administration",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "admin-password",
			Usage:   "relay admin password (for Basic admin auth)",
			Sources: cli.EnvVars("RELAY_ADMIN_PASSWORD", "ATP_AUTH_ADMIN_PASSWORD"),
		},
		&cli.StringFlag{
			Name:    "admin-bearer-token",
			Usage:   "relay admin auth token (for Bearer auth)",
			Sources: cli.EnvVars("RELAY_ADMIN_BEARER_TOKEN"),
		},
	},
	Commands: []*cli.Command{
		&cli.Command{
			Name:  "account",
			Usage: "commands for managing accounts",
			Commands: []*cli.Command{
				&cli.Command{
					Name:      "takedown",
					ArgsUsage: "<account>",
					Usage:     "takedown a single account on relay",
					Flags: []cli.Flag{
						&cli.BoolFlag{
							Name:    "reverse",
							Aliases: []string{"r"},
							Usage:   "un-takedown",
						},
					},
					Action: runRelayAdminAccountTakedown,
				},
				&cli.Command{
					Name:    "list",
					Aliases: []string{"ls"},
					Usage:   "enumerate accounts (eg, takendown)",
					Action:  runRelayAdminAccountList,
				},
			},
		},
		&cli.Command{
			Name:  "host",
			Usage: "commands for upstream hosts (eg, PDS)",
			Commands: []*cli.Command{
				&cli.Command{
					Name:      "add",
					Usage:     "request crawl of upstream host (eg, PDS)",
					ArgsUsage: `<hostname>`,
					Action:    runRelayAdminHostAdd,
				},
				&cli.Command{
					Name:      "block",
					Usage:     "request crawl of upstream host (eg, PDS)",
					ArgsUsage: `<hostname>`,
					Flags: []cli.Flag{
						&cli.BoolFlag{
							Name:    "reverse",
							Aliases: []string{"r"},
							Usage:   "un-takedown",
						},
					},
					Action: runRelayAdminHostBlock,
				},
				&cli.Command{
					Name:    "list",
					Aliases: []string{"ls"},
					Usage:   "enumerate hosts crawled by relay",
					Action:  runRelayAdminHostList,
				},
				&cli.Command{
					Name:      "config",
					Usage:     "update rate-limits per host",
					ArgsUsage: `<hostname>`,
					Flags: []cli.Flag{
						&cli.IntFlag{
							Name: "account-limit",
						},
					},
					Action: runRelayAdminHostConfig,
				},
			},
		},
		&cli.Command{
			Name:  "domain",
			Usage: "commands for domain-level config",
			Commands: []*cli.Command{
				&cli.Command{
					Name:      "ban",
					Usage:     "ban an entire domain name from being crawled",
					ArgsUsage: `<domain>`,
					Flags: []cli.Flag{
						&cli.BoolFlag{
							Name:    "reverse",
							Aliases: []string{"r"},
							Usage:   "un-takedown",
						},
					},
					Action: runRelayAdminDomainBan,
				},
				&cli.Command{
					Name:    "list",
					Aliases: []string{"ls"},
					Usage:   "enumerate domains with configs (eg, bans)",
					Action:  runRelayAdminDomainList,
				},
			},
		},
		&cli.Command{
			Name:  "consumer",
			Usage: "commands for consumers",
			Commands: []*cli.Command{
				&cli.Command{
					Name:    "list",
					Aliases: []string{"ls"},
					Usage:   "enumerate consumers",
					Action:  runRelayAdminConsumerList,
				},
			},
		},
	},
}

type RelayAdminClient struct {
	Host        string
	Password    string
	BearerToken string
}

func (c *RelayAdminClient) Do(method, path string, params map[string]string, body map[string]any) ([]byte, error) {
	u, err := url.Parse(c.Host)
	if err != nil {
		return nil, err
	}
	u.Path = path
	q := u.Query()
	for k, v := range params {
		q.Add(k, v)
	}
	u.RawQuery = q.Encode()

	var buf *bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		buf = bytes.NewBuffer(b)
	}

	var req *http.Request
	if buf != nil {
		req, err = http.NewRequest(method, u.String(), buf)
	} else {
		req, err = http.NewRequest(method, u.String(), nil)
	}
	if err != nil {
		return nil, err
	}
	if c.Password != "" {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:"+c.Password)))
	} else if c.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.BearerToken)
	}
	req.Header.Set("User-Agent", userAgentString())
	if buf != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		slog.Warn("relay HTTP error", "statusCode", resp.StatusCode, "body", string(respBytes))
		return nil, fmt.Errorf("relay HTTP request failed: %d", resp.StatusCode)
	}
	return respBytes, nil
}

func NewRelayAdminClient(cmd *cli.Command) (*RelayAdminClient, error) {
	client := RelayAdminClient{
		Host:        cmd.String("relay-host"),
		Password:    cmd.String("admin-password"),
		BearerToken: cmd.String("admin-bearer-token"),
	}
	if client.Password == "" && client.BearerToken == "" {
		return nil, fmt.Errorf("either admin password or admin bearer token must be provided")
	}
	return &client, nil
}

func runRelayAdminAccountTakedown(ctx context.Context, cmd *cli.Command) error {

	username := cmd.Args().First()
	if username == "" {
		return fmt.Errorf("need to provide username as an argument")
	}
	ident, err := resolveIdent(ctx, cmd, username)
	if err != nil {
		return err
	}

	client, err := NewRelayAdminClient(cmd)
	if err != nil {
		return err
	}

	path := "/admin/repo/takeDown"
	if cmd.Bool("reverse") {
		path = "/admin/repo/reverseTakedown"
	}

	body := map[string]any{
		"did": ident.DID.String(),
	}
	_, err = client.Do("POST", path, nil, body)
	if err != nil {
		return err
	}
	return nil
}

func runRelayAdminAccountList(ctx context.Context, cmd *cli.Command) error {
	client, err := NewRelayAdminClient(cmd)
	if err != nil {
		return err
	}
	path := "/admin/repo/takedowns"
	params := map[string]string{
		"cursor": "",
		"size":   "500",
	}
	for {
		respBytes, err := client.Do("GET", path, params, nil)
		if err != nil {
			return err
		}
		var resp map[string]any
		if err := json.Unmarshal(respBytes, &resp); err != nil {
			return err
		}
		for _, d := range resp["dids"].([]any) {
			fmt.Println(d)
		}
		cursor, ok := resp["cursor"]
		if !ok || cursor == "" {
			break
		}
		params["cursor"] = cursor.(string)
	}
	return nil
}

func runRelayAdminHostAdd(ctx context.Context, cmd *cli.Command) error {

	hostname := cmd.Args().First()
	if hostname == "" {
		return fmt.Errorf("need to provide hostname as an argument")
	}

	client, err := NewRelayAdminClient(cmd)
	if err != nil {
		return err
	}
	path := "/admin/pds/requestCrawl"
	body := map[string]any{
		"hostname": hostname,
	}
	_, err = client.Do("POST", path, nil, body)
	if err != nil {
		return err
	}
	return nil
}

func runRelayAdminHostBlock(ctx context.Context, cmd *cli.Command) error {

	hostname := cmd.Args().First()
	if hostname == "" {
		return fmt.Errorf("need to provide hostname as an argument")
	}

	client, err := NewRelayAdminClient(cmd)
	if err != nil {
		return err
	}

	path := "/admin/pds/block"
	if cmd.Bool("reverse") {
		path = "/admin/pds/unblock"
	}

	params := map[string]string{
		"host": hostname,
	}
	_, err = client.Do("POST", path, params, nil)
	if err != nil {
		return err
	}
	return nil
}

func runRelayAdminHostList(ctx context.Context, cmd *cli.Command) error {
	client, err := NewRelayAdminClient(cmd)
	if err != nil {
		return err
	}
	path := "/admin/pds/list"

	respBytes, err := client.Do("GET", path, nil, nil)
	if err != nil {
		return err
	}
	var rows []map[string]any
	if err := json.Unmarshal(respBytes, &rows); err != nil {
		return err
	}
	for _, r := range rows {
		b, err := json.Marshal(r)
		if err != nil {
			return nil
		}
		fmt.Println(string(b))
	}
	return nil
}

func runRelayAdminHostConfig(ctx context.Context, cmd *cli.Command) error {

	hostname := cmd.Args().First()
	if hostname == "" {
		return fmt.Errorf("need to provide hostname as an argument")
	}

	client, err := NewRelayAdminClient(cmd)
	if err != nil {
		return err
	}

	path := "/admin/pds/changeLimits"

	body := map[string]any{
		"host": hostname,
	}
	if cmd.IsSet("account-limit") {
		body["repo_limit"] = cmd.Int("account-limit")
	}

	_, err = client.Do("POST", path, nil, body)
	if err != nil {
		return err
	}
	return nil
}

func runRelayAdminDomainBan(ctx context.Context, cmd *cli.Command) error {

	domain := cmd.Args().First()
	if domain == "" {
		return fmt.Errorf("need to provide domain as an argument")
	}

	client, err := NewRelayAdminClient(cmd)
	if err != nil {
		return err
	}

	path := "/admin/subs/banDomain"
	if cmd.Bool("reverse") {
		path = "/admin/subs/unbanDomain"
	}

	body := map[string]any{
		"domain": domain,
	}
	_, err = client.Do("POST", path, nil, body)
	if err != nil {
		return err
	}
	return nil
}

func runRelayAdminDomainList(ctx context.Context, cmd *cli.Command) error {
	client, err := NewRelayAdminClient(cmd)
	if err != nil {
		return err
	}
	path := "/admin/subs/listDomainBans"

	respBytes, err := client.Do("GET", path, nil, nil)
	if err != nil {
		return err
	}
	var resp map[string]any
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return err
	}
	for _, d := range resp["banned_domains"].([]any) {
		fmt.Println(d)
	}
	return nil
}

func runRelayAdminConsumerList(ctx context.Context, cmd *cli.Command) error {
	client, err := NewRelayAdminClient(cmd)
	if err != nil {
		return err
	}
	path := "/admin/consumers/list"

	respBytes, err := client.Do("GET", path, nil, nil)
	if err != nil {
		return err
	}
	var rows []map[string]any
	if err := json.Unmarshal(respBytes, &rows); err != nil {
		return err
	}
	for _, r := range rows {
		b, err := json.Marshal(r)
		if err != nil {
			return nil
		}
		fmt.Println(string(b))
	}
	return nil
}
