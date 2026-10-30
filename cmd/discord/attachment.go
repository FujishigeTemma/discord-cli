package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

type AttachmentCmd struct {
	Download AttachmentDownloadCmd `cmd:"" help:"Save a message's attachments under $XDG_CACHE_HOME/discord-cli/attachments/<attachment_id>/ and print their paths."`
}

type AttachmentDownloadCmd struct{ msgRef }

func (c *AttachmentDownloadCmd) Run() error {
	s, _, err := session()
	if err != nil {
		return err
	}
	// Fetch the message for fresh URLs; Discord's signed CDN links expire.
	m, err := s.ChannelMessage(c.ChannelID, c.MessageID)
	if err != nil {
		return err
	}
	dir, err := cacheDir()
	if err != nil {
		return err
	}
	for _, a := range m.Attachments {
		// One directory per attachment: a message may hold two files with the same name.
		adir := filepath.Join(dir, "attachments", a.ID)
		if err := os.MkdirAll(adir, 0o700); err != nil {
			return err
		}
		path := filepath.Join(adir, filepath.Base(a.Filename))
		if _, err := os.Stat(path); err != nil {
			if err := download(s.Client, a.URL, path); err != nil {
				return err
			}
		}
		if err := emit(struct {
			Filename string `json:"filename"`
			Path     string `json:"path"`
		}{a.Filename, path}); err != nil {
			return err
		}
	}
	return nil
}

func download(client *http.Client, url, path string) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", filepath.Base(path), resp.Status)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(path)
		return err
	}
	return f.Close()
}
