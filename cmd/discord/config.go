package main

import (
	"encoding/json/v2"
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bwmarrin/discordgo"
)

type Config struct {
	Token   string `json:"token"`
	GuildID string `json:"guild_id"`
}

// configPath is replaced in tests.
var configPath = func() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "discord-cli", "config.json"), nil
}

func loadConfig() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("not logged in: run `discord auth login`")
	}
	if err != nil {
		return nil, err
	}
	var c Config
	return &c, json.Unmarshal(b, &c)
}

func saveConfig(c *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// httpClient is replaced in tests to point at a fake server.
var httpClient *http.Client

func newSession(token string) (*discordgo.Session, error) {
	s, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}
	if httpClient != nil {
		s.Client = httpClient
	}
	return s, nil
}

// session returns a REST session and the server ID saved by `auth login`.
func session() (*discordgo.Session, string, error) {
	c, err := loadConfig()
	if err != nil {
		return nil, "", err
	}
	s, err := newSession(c.Token)
	return s, c.GuildID, err
}
