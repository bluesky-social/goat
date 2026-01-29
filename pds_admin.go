package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/syntax"

	"github.com/urfave/cli/v3"
)

var cmdPDSAdmin = &cli.Command{
	Name:  "admin",
	Usage: "comands for PDS administration",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "admin-password",
			Usage:   "PDS admin password (for Basic admin auth)",
			Sources: cli.EnvVars("PDS_ADMIN_PASSWORD", "ATP_AUTH_ADMIN_PASSWORD"),
		},
		&cli.StringFlag{
			// NOTE: duplicate of parent "pds-host", but has a default value
			Name:    "pds-host",
			Usage:   "method, hostname, and port of PDS instance",
			Value:   "http://localhost:3000",
			Sources: cli.EnvVars("PDS_HOST"),
		},
	},
	Commands: []*cli.Command{
		&cli.Command{
			Name:  "account",
			Usage: "commands for managing accounts",
			Commands: []*cli.Command{
				&cli.Command{
					Name:    "list",
					Aliases: []string{"ls"},
					Usage:   "enumerate accounts (eg, takendown)",
					Action:  runPDSAdminAccountList,
				},
				&cli.Command{
					Name:      "takedown",
					ArgsUsage: "<account>",
					Usage:     "takedown a single account",
					Flags: []cli.Flag{
						&cli.BoolFlag{
							Name:    "reverse",
							Aliases: []string{"r"},
							Usage:   "un-takedown",
						},
					},
					Action: runPDSAdminAccountTakedown,
				},
				&cli.Command{
					Name:      "delete",
					ArgsUsage: "<account>",
					Usage:     "permanently delete an account",
					Action:    runPDSAdminAccountDelete,
				},
				&cli.Command{
					Name:      "info",
					ArgsUsage: "<account>",
					Usage:     "fetch private account information",
					Action:    runPDSAdminAccountInfo,
				},
				&cli.Command{
					Name:      "update",
					ArgsUsage: "<account>",
					Usage:     "update account auth info",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:  "email",
							Usage: "account email address",
						},
						&cli.StringFlag{
							Name: "handle",
						},
					},
					Action: runPDSAdminAccountUpdate,
				},
				&cli.Command{
					Name:      "reset-password",
					ArgsUsage: "<account>",
					Usage:     "generate new password (and print to stdout)",
					Action:    runPDSAdminAccountResetPassword,
				},
				&cli.Command{
					Name:  "create",
					Usage: "create a new account (auto-generates invite code)",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "handle",
							Usage:    "handle for new account",
							Required: true,
							Sources:  cli.EnvVars("NEW_ACCOUNT_HANDLE"),
						},
						&cli.StringFlag{
							Name:     "password",
							Usage:    "initial account password",
							Required: true,
							Sources:  cli.EnvVars("NEW_ACCOUNT_PASSWORD"),
						},
						&cli.StringFlag{
							Name:     "email",
							Required: true,
							Usage:    "email address for new account",
							Sources:  cli.EnvVars("NEW_ACCOUNT_EMAIL"),
						},
						&cli.StringFlag{
							Name:  "existing-did",
							Usage: "an existing DID to use (eg, non-PLC DID, or migration)",
						},
						&cli.StringFlag{
							Name:  "recovery-key",
							Usage: "public cryptographic key (did:key) to add as PLC recovery",
						},
					},
					Action: runPDSAdminAccountCreate,
				},
			},
		},
		&cli.Command{
			Name:  "blob",
			Usage: "commands for managing public media files (blobs)",
			Commands: []*cli.Command{
				&cli.Command{
					Name:      "status",
					ArgsUsage: "<account> <cid>",
					Usage:     "check moderation status of a specific blob",
					Action:    runPDSAdminBlobStatus,
				},
				&cli.Command{
					Name:      "purge",
					ArgsUsage: "<account> <cid>",
					Usage:     "takedown a blob",
					Flags: []cli.Flag{
						&cli.BoolFlag{
							Name:    "reverse",
							Aliases: []string{"r"},
							Usage:   "un-purge",
						},
					},
					Action: runPDSAdminBlobPurge,
				},
			},
		},
		&cli.Command{
			Name:  "create-invites",
			Usage: "generate invite codes",
			Flags: []cli.Flag{
				&cli.IntFlag{
					Name:    "count",
					Aliases: []string{"n"},
					Value:   1,
					Usage:   "how many invite codes to create",
				},
				&cli.IntFlag{
					Name:  "uses",
					Value: 1,
					Usage: "how many times each code can be used",
				},
			},
			Action: runPDSAdminCreateInvites,
		},
	},
}

func NewPDSAdminClient(cmd *cli.Command) (*atclient.APIClient, error) {
	adminPass := cmd.String("admin-password")
	if adminPass == "" {
		// try reading from PDS env file (if we are on PDS host or docker container)
		b, err := os.ReadFile("/pds/pds.env")
		if err == nil {
			scanner := bufio.NewScanner(strings.NewReader(string(b)))
			for scanner.Scan() {
				line := scanner.Text()
				suffix, ok := strings.CutPrefix(line, "PDS_ADMIN_PASSWORD=")
				if ok {
					adminPass = suffix
					break
				}
			}
		}
	}
	if adminPass == "" {
		return nil, fmt.Errorf("PDS admin password required")
	}
	c := atclient.NewAdminClient(cmd.String("pds-host"), adminPass)
	return c, nil
}

func runPDSAdminAccountTakedown(ctx context.Context, cmd *cli.Command) error {

	username := cmd.Args().First()
	if username == "" {
		return fmt.Errorf("need to provide username as an argument")
	}
	did, err := resolveToDID(ctx, cmd, username)
	if err != nil {
		return err
	}

	reversal := cmd.Bool("reverse")

	client, err := NewPDSAdminClient(cmd)
	if err != nil {
		return err
	}

	_, err = comatproto.AdminUpdateSubjectStatus(ctx, client, &comatproto.AdminUpdateSubjectStatus_Input{
		Subject: &comatproto.AdminUpdateSubjectStatus_Input_Subject{
			AdminDefs_RepoRef: &comatproto.AdminDefs_RepoRef{
				LexiconTypeID: "com.atproto.admin.defs#repoRef",
				Did:           did.String(),
			},
		},
		Takedown: &comatproto.AdminDefs_StatusAttr{
			Applied: !reversal,
			// NOTE: should ref be datetime?
		},
	})

	return err
}

func runPDSAdminAccountList(ctx context.Context, cmd *cli.Command) error {

	client, err := NewPDSAdminClient(cmd)
	if err != nil {
		return err
	}

	cursor := ""
	var size int64 = 500
	for {
		resp, err := comatproto.SyncListRepos(ctx, client, cursor, size)
		if err != nil {
			return err
		}

		for _, r := range resp.Repos {
			if cmd.Bool("json") {
				b, err := json.Marshal(r)
				if err != nil {
					return err
				}
				fmt.Println(string(b))
			} else {
				status := "unknown"
				if r.Active != nil && *r.Active {
					status = "active"
				} else if r.Status != nil {
					status = *r.Status
				}
				fmt.Printf("%s\t%s\t%s\n", r.Did, status, r.Rev)
			}
		}

		if resp.Cursor == nil || *resp.Cursor == "" {
			break
		}
		cursor = *resp.Cursor
	}
	return nil
}

func runPDSAdminAccountDelete(ctx context.Context, cmd *cli.Command) error {

	username := cmd.Args().First()
	if username == "" {
		return fmt.Errorf("need to provide username as an argument")
	}
	did, err := resolveToDID(ctx, cmd, username)
	if err != nil {
		return err
	}

	client, err := NewPDSAdminClient(cmd)
	if err != nil {
		return err
	}

	return comatproto.AdminDeleteAccount(ctx, client, &comatproto.AdminDeleteAccount_Input{
		Did: did.String(),
	})
}

func runPDSAdminAccountInfo(ctx context.Context, cmd *cli.Command) error {

	username := cmd.Args().First()
	if username == "" {
		return fmt.Errorf("need to provide username as an argument")
	}
	did, err := resolveToDID(ctx, cmd, username)
	if err != nil {
		return err
	}

	client, err := NewPDSAdminClient(cmd)
	if err != nil {
		return err
	}

	r, err := comatproto.AdminGetAccountInfo(ctx, client, did.String())
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func runPDSAdminAccountUpdate(ctx context.Context, cmd *cli.Command) error {

	username := cmd.Args().First()
	if username == "" {
		return fmt.Errorf("need to provide username as an argument")
	}
	did, err := resolveToDID(ctx, cmd, username)
	if err != nil {
		return err
	}

	client, err := NewPDSAdminClient(cmd)
	if err != nil {
		return err
	}

	email := cmd.String("email")
	if email != "" {
		err := comatproto.AdminUpdateAccountEmail(ctx, client, &comatproto.AdminUpdateAccountEmail_Input{
			Account: did.String(),
			Email:   email,
		})
		if err != nil {
			return err
		}
		fmt.Println("updated email")
	}

	handle := cmd.String("handle")
	if handle != "" {
		_, err := syntax.ParseHandle(handle)
		if err != nil {
			return err
		}

		err = comatproto.AdminUpdateAccountHandle(ctx, client, &comatproto.AdminUpdateAccountHandle_Input{
			Did:    did.String(),
			Handle: handle,
		})
		if err != nil {
			return err
		}
		fmt.Println("updated handle")
	}

	return nil
}

func runPDSAdminAccountResetPassword(ctx context.Context, cmd *cli.Command) error {

	username := cmd.Args().First()
	if username == "" {
		return fmt.Errorf("need to provide username as an argument")
	}
	did, err := resolveToDID(ctx, cmd, username)
	if err != nil {
		return err
	}

	client, err := NewPDSAdminClient(cmd)
	if err != nil {
		return err
	}

	password := rand.Text()
	err = comatproto.AdminUpdateAccountPassword(ctx, client, &comatproto.AdminUpdateAccountPassword_Input{
		Did:      did.String(),
		Password: password,
	})
	if err != nil {
		return err
	}
	fmt.Printf("new password: %s\n", password)

	return nil
}

func runPDSAdminBlobStatus(ctx context.Context, cmd *cli.Command) error {

	username := cmd.Args().First()
	if username == "" {
		return fmt.Errorf("need to provide username as an argument")
	}
	blobCID := cmd.Args().Get(1)
	if blobCID == "" {
		return fmt.Errorf("need to provide blob CID as second argument")
	}

	did, err := resolveToDID(ctx, cmd, username)
	if err != nil {
		return err
	}

	client, err := NewPDSAdminClient(cmd)
	if err != nil {
		return err
	}

	resp, err := comatproto.AdminGetSubjectStatus(ctx, client, blobCID, did.String(), "")
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func runPDSAdminBlobPurge(ctx context.Context, cmd *cli.Command) error {

	username := cmd.Args().First()
	if username == "" {
		return fmt.Errorf("need to provide username as an argument")
	}
	blobCID := cmd.Args().Get(1)
	if blobCID == "" {
		return fmt.Errorf("need to provide blob CID as second argument")
	}

	did, err := resolveToDID(ctx, cmd, username)
	if err != nil {
		return err
	}

	reversal := cmd.Bool("reverse")

	client, err := NewPDSAdminClient(cmd)
	if err != nil {
		return err
	}

	_, err = comatproto.AdminUpdateSubjectStatus(ctx, client, &comatproto.AdminUpdateSubjectStatus_Input{
		Subject: &comatproto.AdminUpdateSubjectStatus_Input_Subject{
			AdminDefs_RepoBlobRef: &comatproto.AdminDefs_RepoBlobRef{
				LexiconTypeID: "com.atproto.admin.defs#repoBlobRef",
				Did:           did.String(),
				Cid:           blobCID,
			},
		},
		Takedown: &comatproto.AdminDefs_StatusAttr{
			Applied: !reversal,
			// NOTE: should ref be datetime?
		},
	})
	return err
}

func runPDSAdminCreateInvites(ctx context.Context, cmd *cli.Command) error {

	client, err := NewPDSAdminClient(cmd)
	if err != nil {
		return err
	}

	for range cmd.Int("count") {
		resp, err := comatproto.ServerCreateInviteCode(ctx, client, &comatproto.ServerCreateInviteCode_Input{
			UseCount: int64(cmd.Int("uses")),
		})
		if err != nil {
			return err
		}
		fmt.Println(resp.Code)
	}
	return nil
}

func runPDSAdminAccountCreate(ctx context.Context, cmd *cli.Command) error {
	adminClient, err := NewPDSAdminClient(cmd)
	if err != nil {
		return err
	}

	inviteResp, err := comatproto.ServerCreateInviteCode(ctx, adminClient, &comatproto.ServerCreateInviteCode_Input{
		UseCount: 1,
	})
	if err != nil {
		return fmt.Errorf("failed to create invite code: %w", err)
	}

	return createAccount(ctx, cmd, inviteResp.Code)
}
