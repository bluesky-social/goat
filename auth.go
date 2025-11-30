package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"

	"github.com/adrg/xdg"
	"github.com/urfave/cli/v3"
)

var ErrNoAuthSession = errors.New("no auth session found")

type AuthSession struct {
	DID          syntax.DID `json:"did"`
	Password     string     `json:"password"`
	AccessToken  string     `json:"access_token"`
	RefreshToken string     `json:"session_token"`
	PDS          string     `json:"pds"`
}

func persistAuthSession(sess *AuthSession) error {

	fPath, err := xdg.StateFile("goat/auth-session.json")
	if err != nil {
		return err
	}

	f, err := os.OpenFile(fPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	authBytes, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return err
	}
	_, err = f.Write(authBytes)
	return err
}

func loadAuthSessionFile() (*AuthSession, error) {
	fPath, err := xdg.SearchStateFile("goat/auth-session.json")
	if err != nil {
		return nil, ErrNoAuthSession
	}

	fBytes, err := os.ReadFile(fPath)
	if err != nil {
		return nil, err
	}

	var sess AuthSession
	err = json.Unmarshal(fBytes, &sess)
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

func authRefreshCallback(ctx context.Context, data atclient.PasswordSessionData) {
	fmt.Println("auth refresh callback")
	sess, _ := loadAuthSessionFile()
	if sess == nil {
		sess = &AuthSession{}
	}

	sess.DID = data.AccountDID
	sess.AccessToken = data.AccessToken
	sess.RefreshToken = data.RefreshToken
	sess.PDS = data.Host

	if err := persistAuthSession(sess); err != nil {
		slog.Warn("failed to save refreshed auth session data", "err", err)
	}
}

func loginOrLoadAuthClient(ctx context.Context, cmd *cli.Command) (*atclient.APIClient, error) {

	// if user/pass provided in env vars, login as emphemeral session with those
	username := cmd.String("username")
	password := cmd.String("password")
	if username != "" && password != "" {
		dir := identity.DefaultDirectory()
		atid, err := syntax.ParseAtIdentifier(username)
		if err != nil {
			return nil, err
		}
		return atclient.LoginWithPassword(ctx, dir, *atid, password, "", nil)
	}

	// otherwise try loading from disk
	return loadAuthClient(ctx)
}

func loadAuthClient(ctx context.Context) (*atclient.APIClient, error) {

	sess, err := loadAuthSessionFile()
	if err != nil {
		return nil, err
	}

	// first try to resume session
	client := atclient.ResumePasswordSession(atclient.PasswordSessionData{
		AccessToken:  sess.AccessToken,
		RefreshToken: sess.RefreshToken,
		AccountDID:   sess.DID,
		Host:         sess.PDS,
	}, authRefreshCallback)

	// check that auth is working
	_, err = comatproto.ServerGetSession(ctx, client)
	if nil == err {
		return client, nil
	}

	// otherwise try new auth session using saved password
	dir := identity.DefaultDirectory()
	return atclient.LoginWithPassword(ctx, dir, sess.DID.AtIdentifier(), sess.Password, "", authRefreshCallback)
}

func wipeAuthSession() error {

	fPath, err := xdg.SearchStateFile("goat/auth-session.json")
	if err != nil {
		fmt.Printf("no auth session found (already logged out)\n")
		return nil
	}
	return os.Remove(fPath)
}
