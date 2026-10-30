package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

type AuthCmd struct {
	Login  AuthLoginCmd  `cmd:"" help:"Read a bot token from stdin, verify it and save it."`
	Status AuthStatusCmd `cmd:"" help:"Show the bot user and server."`
	Logout AuthLogoutCmd `cmd:"" help:"Delete the saved token."`
}

type AuthLoginCmd struct{}

func (AuthLoginCmd) Run() error {
	token, err := readToken()
	if err != nil {
		return err
	}
	s, err := newSession(token)
	if err != nil {
		return err
	}
	me, err := s.User("@me")
	if err != nil {
		return err
	}
	gs, err := s.UserGuilds(2, "", "", false)
	if err != nil {
		return err
	}
	if len(gs) != 1 {
		return fmt.Errorf("the bot must be in exactly one server, but is in %d", len(gs))
	}
	if err := saveConfig(&Config{Token: token, GuildID: gs[0].ID}); err != nil {
		return err
	}
	return emitAuth(me.ID, me.Username, gs[0].ID, gs[0].Name)
}

func readToken() (string, error) {
	var token string
	if fd := int(os.Stdin.Fd()); term.IsTerminal(fd) {
		fmt.Fprint(os.Stderr, "Bot token: ")
		b, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		token = string(b)
	} else {
		token, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", errors.New("no token given")
	}
	return token, nil
}

func emitAuth(userID, userName, guildID, guildName string) error {
	return emit(map[string]any{
		"user":  map[string]string{"id": userID, "name": userName},
		"guild": map[string]string{"id": guildID, "name": guildName},
	})
}

type AuthStatusCmd struct{}

func (AuthStatusCmd) Run() error {
	s, gid, err := session()
	if err != nil {
		return err
	}
	me, err := s.User("@me")
	if err != nil {
		return err
	}
	g, err := s.Guild(gid)
	if err != nil {
		return err
	}
	return emitAuth(me.ID, me.Username, g.ID, g.Name)
}

type AuthLogoutCmd struct{}

func (AuthLogoutCmd) Run() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return emitOK()
}
