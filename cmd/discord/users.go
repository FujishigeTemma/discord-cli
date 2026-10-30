package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// seenUsers collects username -> ID from everything printed in this run.
// They are cached so `message send --mention NAME` can resolve names.
var seenUsers = map[string]string{}

func rememberUser(u *discordgo.User) {
	if u != nil && u.Username != "" {
		seenUsers[u.Username] = u.ID
	}
}

func usersPath() (string, error) {
	dir, err := cacheDir()
	return filepath.Join(dir, "users.json"), err
}

func loadUsers() (map[string]string, error) {
	path, err := usersPath()
	if err != nil {
		return nil, err
	}
	users := map[string]string{}
	if err := readJSONFile(path, &users); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return users, nil
}

func saveSeenUsers() error {
	if len(seenUsers) == 0 {
		return nil
	}
	users, err := loadUsers()
	if err != nil {
		return err
	}
	changed := false
	for name, id := range seenUsers {
		if users[name] != id {
			users[name], changed = id, true
		}
	}
	if !changed {
		return nil
	}
	path, err := usersPath()
	if err != nil {
		return err
	}
	return writeJSONFile(path, users)
}

// mentionPrefix turns names (name, @name or <@name>) into "<@id> <@id> ".
func mentionPrefix(names []string) (string, error) {
	if len(names) == 0 {
		return "", nil
	}
	users, err := loadUsers()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, name := range names {
		name = strings.Trim(name, "<@>")
		id, ok := users[name]
		if !ok {
			return "", fmt.Errorf("unknown user %q: names are learned from message list/get output", name)
		}
		b.WriteString("<@" + id + "> ")
	}
	return b.String(), nil
}
