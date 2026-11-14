package main

import (
	"context"
	"encoding/json"
	"fmt"

	// TODO: will end up in indigo
	"github.com/bluesky-social/indigo/atproto/syntax"
	spaceapi "tangled.org/bnewbold.net/cobalt/atspace/comatproto"
	"tangled.org/bnewbold.net/cobalt/atspace/xsyntax"

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
		// ctx, c, cursor string, limit *int64, spaceDid string, spaceType string
		resp, err := spaceapi.SpaceListSpaces(ctx, client, cursor, new(int64(100)), client.AccountDID.String(), "")
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

	space, err := xsyntax.ParseSpaceRef(cmd.StringArg("space"))
	if err != nil {
		return err
	}

	client, err := loadAuthClient(ctx, cmd)
	if err == ErrNoAuthSession {
		return fmt.Errorf("PDS auth required, but not logged in")
	} else if err != nil {
		return err
	}

	resp, err := spaceapi.SpaceGetSpace(ctx, client, space.String())
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

	space, err := xsyntax.ParseSpaceRef(cmd.StringArg("space"))
	if err != nil {
		return err
	}

	client, err := loadAuthClient(ctx, cmd)
	if err == ErrNoAuthSession {
		return fmt.Errorf("PDS auth required, but not logged in")
	} else if err != nil {
		return err
	}

	err = spaceapi.SpaceDeleteSpace(ctx, client, &spaceapi.SpaceDeleteSpace_Input{
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

	_, err = spaceapi.SpaceCreateSpace(ctx, client, &spaceapi.SpaceCreateSpace_Input{
		SpaceType: spaceType.String(),
		Skey:      skeyArg,
		Policy: spaceapi.SpaceCreateSpace_Input_Policy{
			PersonalspacePolicy: &spaceapi.PersonalspacePolicy{},
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

	_, err = spaceapi.SpaceCreateSpace(ctx, client, &spaceapi.SpaceCreateSpace_Input{
		SpaceType: spaceType.String(),
		Skey:      skeyArg,
		Policy: spaceapi.SpaceCreateSpace_Input_Policy{
			MemberspacePolicy: &spaceapi.MemberspacePolicy{
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

	space, err := xsyntax.ParseSpaceRef(cmd.StringArg("space"))
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
		// ctx, c, cursor string, limit *int64, space string
		resp, err := spaceapi.MemberspaceListMembers(ctx, client, cursor, new(int64(100)), space.String())
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

	space, err := xsyntax.ParseSpaceRef(cmd.StringArg("space"))
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

	err = spaceapi.MemberspacePutMembers(ctx, client, &spaceapi.MemberspacePutMembers_Input{
		Space: space.String(),
		Members: []spaceapi.MemberspaceDefs_Member{
			spaceapi.MemberspaceDefs_Member{
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

	space, err := xsyntax.ParseSpaceRef(cmd.StringArg("space"))
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

	err = spaceapi.MemberspaceRemoveMembers(ctx, client, &spaceapi.MemberspaceRemoveMembers_Input{
		Space: space.String(),
		Dids:  []string{did.String()},
	})
	if err != nil {
		return err
	}

	fmt.Println("success")
	return nil
}
