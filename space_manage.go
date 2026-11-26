package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	// TODO: will end up in indigo
	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/syntax"

	"github.com/urfave/cli/v3"
)

var cmdSpaceManage = &cli.Command{
	Name:  "manage",
	Usage: "commands for creating and managing spaces",
	Flags: []cli.Flag{},
	Commands: []*cli.Command{
		&cli.Command{
			Name:    "list",
			Aliases: []string{"ls"},
			Usage:   "list all space repos for current account",
			Flags:   []cli.Flag{},
			Action:  runSpaceManageList,
		},
		&cli.Command{
			Name:  "policy",
			Usage: "show the policy object for a space",
			Arguments: []cli.Argument{
				&cli.StringArg{
					Name:     "space",
					Required: true,
				},
			},
			Action: runSpaceManagePolicy,
		},
		cmdSpaceManageCreate,
		&cli.Command{
			Name: "delete",
			Arguments: []cli.Argument{
				&cli.StringArg{
					Name:     "space",
					Required: true,
				},
			},
			Usage:  "remove a space",
			Action: runSpaceManageDelete,
		},
		cmdSpaceManageMember,
		&cli.Command{
			Name: "export",
			Arguments: []cli.Argument{
				&cli.StringArg{
					Name:     "space",
					Required: true,
				},
			},
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "output",
					Aliases: []string{"o"},
					Usage:   "file path for STAR download",
				},
			},
			Usage:  "export space meta STAR file (not space repo)",
			Action: runSpaceManageExport,
		},
		&cli.Command{
			Name:  "import",
			Usage: "upload space meta STAR file (eg, for migration)",
			Arguments: []cli.Argument{
				&cli.StringArg{
					Name:     "path",
					Required: true,
				},
			},
			Action: runSpaceManageImport,
		},
	},
}

var cmdSpaceManageCreate = &cli.Command{
	Name:  "create",
	Usage: "create spaces of different policy types",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "space-type",
			Aliases: []string{"t"},
			Usage:   "space type (NSID)",
		},
		&cli.StringFlag{
			Name:  "skey",
			Usage: "space key (record-key syntax)",
		},
	},
	Commands: []*cli.Command{
		&cli.Command{
			Name:   "personal",
			Usage:  "create a personalspace",
			Action: runSpaceManageCreatePersonal,
		},
		&cli.Command{
			Name:  "member",
			Usage: "create a memberspace",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:  "public-read",
					Usage: "anybody can read from the space",
				},
				&cli.BoolFlag{
					Name:  "public-write",
					Usage: "anybody can write to the space",
				},
				&cli.BoolFlag{
					Name:  "publish-policy",
					Usage: "make space config visible to members",
				},
				&cli.BoolFlag{
					Name:  "publish-members",
					Usage: "make space membership list visible to members",
				},
			},
			Action: runSpaceManageCreateMember,
		},
	},
}

var cmdSpaceManageMember = &cli.Command{
	Name:  "member",
	Usage: "manage memberspace rosters",
	Commands: []*cli.Command{
		&cli.Command{
			Name:   "list",
			Usage:  "list members",
			Action: runSpaceManageMemberList,
		},
		&cli.Command{
			Name:  "add",
			Usage: "add an account to space",
			Arguments: []cli.Argument{
				&cli.StringArg{
					Name:     "account",
					Required: true,
				},
			},
			Action: runSpaceManageMemberAdd,
		},
	},
}

func runSpaceManageList(ctx context.Context, cmd *cli.Command) error {

	client, err := loadAuthClient(ctx, cmd)
	if err == ErrNoAuthSession {
		return fmt.Errorf("PDS auth required, but not logged in")
	} else if err != nil {
		return err
	}

	cursor := ""
	for {
		// ctx, c, cursor string, limit int64, spaceDid string, spaceType string
		resp, err := comatproto.SpaceListSpaces(ctx, client, cursor, int64(100), client.AccountDID.String(), "")
		if err != nil {
			return err
		}
		for _, sp := range resp.Spaces {
			fmt.Println(sp.Space)
		}
		if resp.Cursor != nil && *resp.Cursor != "" {
			cursor = *resp.Cursor
		} else {
			break
		}
	}

	return nil
}

func runSpaceManagePolicy(ctx context.Context, cmd *cli.Command) error {

	space, err := syntax.ParseSpaceRef(cmd.StringArg("space"))
	if err != nil {
		return err
	}

	client, err := loadAuthClient(ctx, cmd)
	if err == ErrNoAuthSession {
		return fmt.Errorf("PDS auth required, but not logged in")
	} else if err != nil {
		return err
	}

	resp, err := comatproto.SpaceGetSpace(ctx, client, space.String())
	if err != nil {
		return err
	}

	b, err := json.MarshalIndent(&resp.Policy, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))

	return nil
}

func runSpaceManageDelete(ctx context.Context, cmd *cli.Command) error {

	space, err := syntax.ParseSpaceRef(cmd.StringArg("space"))
	if err != nil {
		return err
	}

	client, err := loadAuthClient(ctx, cmd)
	if err == ErrNoAuthSession {
		return fmt.Errorf("PDS auth required, but not logged in")
	} else if err != nil {
		return err
	}

	err = comatproto.SpaceDeleteSpace(ctx, client, &comatproto.SpaceDeleteSpace_Input{
		Space: space.String(),
	})
	if err != nil {
		return err
	}

	fmt.Println("success")
	return nil
}

func runSpaceManageCreatePersonal(ctx context.Context, cmd *cli.Command) error {

	spaceType, err := syntax.ParseNSID(cmd.String("space-type"))
	if err != nil {
		return fmt.Errorf("space-type syntax: %w", err)
	}
	var skeyArg *string
	if cmd.IsSet("skey") {
		skey, err := syntax.ParseRecordKey(cmd.String("skey"))
		if err != nil {
			return fmt.Errorf("space-type syntax: %w", err)
		}
		skeyArg = new(skey.String())
	}

	client, err := loadAuthClient(ctx, cmd)
	if err == ErrNoAuthSession {
		return fmt.Errorf("PDS auth required, but not logged in")
	} else if err != nil {
		return err
	}

	_, err = comatproto.SpaceCreateSpace(ctx, client, &comatproto.SpaceCreateSpace_Input{
		SpaceType: spaceType.String(),
		Skey:      skeyArg,
		Policy: &comatproto.SpaceCreateSpace_Input_Policy{
			PersonalspacePolicy: &comatproto.PersonalspacePolicy{},
		},
	})
	if err != nil {
		return err
	}

	fmt.Println("success")
	return nil
}

func runSpaceManageCreateMember(ctx context.Context, cmd *cli.Command) error {

	spaceType, err := syntax.ParseNSID(cmd.String("space-type"))
	if err != nil {
		return fmt.Errorf("space-type syntax: %w", err)
	}
	var skeyArg *string
	if cmd.IsSet("skey") {
		skey, err := syntax.ParseRecordKey(cmd.String("skey"))
		if err != nil {
			return fmt.Errorf("space-type syntax: %w", err)
		}
		skeyArg = new(skey.String())
	}

	client, err := loadAuthClient(ctx, cmd)
	if err == ErrNoAuthSession {
		return fmt.Errorf("PDS auth required, but not logged in")
	} else if err != nil {
		return err
	}

	_, err = comatproto.SpaceCreateSpace(ctx, client, &comatproto.SpaceCreateSpace_Input{
		SpaceType: spaceType.String(),
		Skey:      skeyArg,
		Policy: &comatproto.SpaceCreateSpace_Input_Policy{
			MemberspacePolicy: &comatproto.MemberspacePolicy{
				// TODO: AllowedApps
				PublicRead:     new(cmd.Bool("public-read")),
				PublicWrite:    new(cmd.Bool("public-write")),
				PublishPolicy:  new(cmd.Bool("publish-policy")),
				PublishMembers: new(cmd.Bool("publish-members")),
			},
		},
	})
	if err != nil {
		return err
	}

	fmt.Println("success")
	return nil
}

func runSpaceManageMemberList(ctx context.Context, cmd *cli.Command) error {

	space, err := syntax.ParseSpaceRef(cmd.StringArg("space"))
	if err != nil {
		return err
	}

	client, err := loadAuthClient(ctx, cmd)
	if err == ErrNoAuthSession {
		return fmt.Errorf("PDS auth required, but not logged in")
	} else if err != nil {
		return err
	}

	cursor := ""
	for {
		// ctx, c, cursor string, limit int64, space string
		resp, err := comatproto.MemberspaceListMembers(ctx, client, cursor, int64(100), space.String())
		if err != nil {
			return err
		}
		for _, mem := range resp.Members {
			fmt.Printf("%s\t%t\t%t\n", mem.Did, mem.Read, mem.Write)
		}
		if resp.Cursor != nil && *resp.Cursor != "" {
			cursor = *resp.Cursor
		} else {
			break
		}
	}

	return nil
}

func runSpaceManageMemberAdd(ctx context.Context, cmd *cli.Command) error {

	space, err := syntax.ParseSpaceRef(cmd.StringArg("space"))
	if err != nil {
		return err
	}
	did, err := syntax.ParseDID(cmd.StringArg("account"))
	if err != nil {
		return err
	}

	client, err := loadAuthClient(ctx, cmd)
	if err == ErrNoAuthSession {
		return fmt.Errorf("PDS auth required, but not logged in")
	} else if err != nil {
		return err
	}

	err = comatproto.MemberspacePutMembers(ctx, client, &comatproto.MemberspacePutMembers_Input{
		Space: space.String(),
		Members: []*comatproto.MemberspaceDefs_Member{
			&comatproto.MemberspaceDefs_Member{
				Did: did.String(),
				// TODO: configurable
				Read:  true,
				Write: true,
			},
		},
	})
	if err != nil {
		return err
	}

	fmt.Println("success")
	return nil
}

func runSpaceManageMemberRemove(ctx context.Context, cmd *cli.Command) error {

	space, err := syntax.ParseSpaceRef(cmd.StringArg("space"))
	if err != nil {
		return err
	}
	did, err := syntax.ParseDID(cmd.StringArg("account"))
	if err != nil {
		return err
	}

	client, err := loadAuthClient(ctx, cmd)
	if err == ErrNoAuthSession {
		return fmt.Errorf("PDS auth required, but not logged in")
	} else if err != nil {
		return err
	}

	err = comatproto.MemberspaceRemoveMembers(ctx, client, &comatproto.MemberspaceRemoveMembers_Input{
		Space: space.String(),
		Dids:  []string{did.String()},
	})
	if err != nil {
		return err
	}

	fmt.Println("success")
	return nil
}

func runSpaceManageExport(ctx context.Context, cmd *cli.Command) error {

	space, err := syntax.ParseSpaceRef(cmd.StringArg("space"))
	if err != nil {
		return err
	}

	client, err := loadAuthClient(ctx, cmd)
	if err == ErrNoAuthSession {
		return fmt.Errorf("PDS auth required, but not logged in")
	} else if err != nil {
		return err
	}

	starPath := cmd.String("output")
	if starPath == "" {
		// NOTE: having the rev in the the path might be nice
		now := time.Now().Format("20060102150405")
		starPath = fmt.Sprintf("%s_%s.%s.meta.star", space.SpaceType(), space.SpaceKey(), now)
	}
	output, err := getFileOrStdout(starPath)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("file already exists: %s", starPath)
		}
		return err
	}
	defer output.Close()
	if starPath != stdIOPath {
		fmt.Printf("downloading from %s to: %s\n", client.Host, starPath)
	}

	metaBytes, err := comatproto.SpaceExportSpaceMeta(ctx, client, space.String())
	if err != nil {
		return err
	}
	if _, err := output.Write(metaBytes); err != nil {
		return err
	}
	return nil
}

func runSpaceManageImport(ctx context.Context, cmd *cli.Command) error {

	starPath := cmd.StringArg("path")

	client, err := loadAuthClient(ctx, cmd)
	if err == ErrNoAuthSession {
		return fmt.Errorf("auth required, but not logged in")
	} else if err != nil {
		return err
	}

	// longer timeout for big uploads
	client.Client.Timeout = 300 * time.Second

	// TODO: could stream instead of reading full file
	fileBytes, err := os.ReadFile(starPath)
	if err != nil {
		return err
	}

	err = comatproto.SpaceImportSpaceMeta(ctx, client, bytes.NewReader(fileBytes))
	if err != nil {
		return fmt.Errorf("failed to import space meta: %w", err)
	}

	return nil
}
