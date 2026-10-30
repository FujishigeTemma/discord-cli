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

// xdgDir returns $env/discord-cli, or ~/fallback/discord-cli when env is unset.
func xdgDir(env, fallback string) (string, error) {
	dir := os.Getenv(env)
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, fallback)
	}
	return filepath.Join(dir, "discord-cli"), nil
}

func configPath() (string, error) {
	dir, err := xdgDir("XDG_CONFIG_HOME", ".config")
	return filepath.Join(dir, "config.json"), err
}

func cacheDir() (string, error) { return xdgDir("XDG_CACHE_HOME", ".cache") }

// readJSONFile decodes path into v. A missing file returns os.ErrNotExist.
func readJSONFile(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// writeJSONFile writes v to path, readable only by the user.
func writeJSONFile(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(v, json.Deterministic(true))
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func loadConfig() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	var c Config
	err = readJSONFile(path, &c)
	if errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("not logged in: run `discord auth login`")
	}
	return &c, err
}

func saveConfig(c *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	return writeJSONFile(path, c)
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
