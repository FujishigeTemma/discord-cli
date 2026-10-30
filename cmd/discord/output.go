package main

import (
	"cmp"
	"encoding/json/v2"
	"io"
	"os"
	"time"

	"github.com/bwmarrin/discordgo"
)

// stdout is replaced in tests.
var stdout io.Writer = os.Stdout

// writeJSON writes v as one JSON line.
func writeJSON(w io.Writer, v any) error {
	if err := json.MarshalWrite(w, v); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}

func emit(v any) error { return writeJSON(stdout, v) }

func emitOK() error {
	return emit(struct {
		OK bool `json:"ok"`
	}{true})
}

type Author struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Bot  bool   `json:"bot"`
}

type Attachment struct {
	Filename string `json:"filename"`
	URL      string `json:"url"`
}

type Reaction struct {
	Emoji string `json:"emoji"`
	Count int    `json:"count"`
}

type Message struct {
	ID              string       `json:"id"`
	ChannelID       string       `json:"channel_id"`
	Author          Author       `json:"author"`
	Timestamp       time.Time    `json:"timestamp"`
	EditedTimestamp *time.Time   `json:"edited_timestamp"`
	Content         string       `json:"content"`
	Attachments     []Attachment `json:"attachments"`
	ReplyTo         *string      `json:"reply_to"`
	Reactions       []Reaction   `json:"reactions"`
	Fallback        string       `json:"fallback,omitempty"`
}

func toMessage(m *discordgo.Message) Message {
	out := Message{
		ID:              m.ID,
		ChannelID:       m.ChannelID,
		Timestamp:       m.Timestamp,
		EditedTimestamp: m.EditedTimestamp,
		Content:         m.Content,
	}
	if m.Author != nil {
		out.Author = Author{ID: m.Author.ID, Name: m.Author.Username, Bot: m.Author.Bot}
	}
	for _, a := range m.Attachments {
		out.Attachments = append(out.Attachments, Attachment{Filename: a.Filename, URL: a.URL})
	}
	if m.MessageReference != nil {
		out.ReplyTo = &m.MessageReference.MessageID
	}
	for _, r := range m.Reactions {
		out.Reactions = append(out.Reactions, Reaction{Emoji: r.Emoji.APIName(), Count: r.Count})
	}
	return out
}

var channelTypes = map[discordgo.ChannelType]string{
	discordgo.ChannelTypeGuildText:          "text",
	discordgo.ChannelTypeGuildVoice:         "voice",
	discordgo.ChannelTypeGuildCategory:      "category",
	discordgo.ChannelTypeGuildNews:          "announcement",
	discordgo.ChannelTypeGuildNewsThread:    "announcement_thread",
	discordgo.ChannelTypeGuildPublicThread:  "public_thread",
	discordgo.ChannelTypeGuildPrivateThread: "private_thread",
	discordgo.ChannelTypeGuildStageVoice:    "stage",
	discordgo.ChannelTypeGuildForum:         "forum",
	discordgo.ChannelTypeGuildMedia:         "media",
}

func channelType(t discordgo.ChannelType) string { return cmp.Or(channelTypes[t], "unknown") }
