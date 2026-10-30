package main

import (
	"cmp"
	"encoding/json/v2"
	"io"
	"os"
	"strings"
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

// Sent is the output of send and edit: just what an agent needs next.
type Sent struct {
	ID       string `json:"id"`
	Fallback string `json:"fallback,omitzero"`
}

func emitOK() error {
	return emit(struct {
		OK bool `json:"ok"`
	}{true})
}

type Author struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Bot  bool   `json:"bot,omitzero"`
}

func toAuthor(u *discordgo.User) Author {
	rememberUser(u)
	return Author{ID: u.ID, Name: u.Username, Bot: u.Bot}
}

type Reaction struct {
	Emoji string `json:"emoji"`
	Count int    `json:"count"`
	Me    bool   `json:"me,omitzero"` // the bot itself is one of Count
}

// Message omits empty fields to keep output small in an AI's context.
type Message struct {
	ID              string     `json:"id"`
	ChannelID       string     `json:"channel_id,omitzero"`
	Author          Author     `json:"author"`
	Timestamp       time.Time  `json:"timestamp"`
	EditedTimestamp *time.Time `json:"edited_timestamp,omitzero"`
	Content         string     `json:"content"`
	Attachments     []string   `json:"attachments,omitzero"` // filenames; see `attachment download`
	ReplyTo         string     `json:"reply_to,omitzero"`
	Reactions       []Reaction `json:"reactions,omitzero"`
}

func toMessage(m *discordgo.Message) Message {
	out := Message{
		ID:              m.ID,
		ChannelID:       m.ChannelID,
		Timestamp:       m.Timestamp,
		EditedTimestamp: m.EditedTimestamp,
		Content:         contentWithNames(m),
	}
	if m.Author != nil {
		out.Author = toAuthor(m.Author)
	}
	for _, a := range m.Attachments {
		out.Attachments = append(out.Attachments, a.Filename)
	}
	if m.MessageReference != nil {
		out.ReplyTo = m.MessageReference.MessageID
	}
	for _, r := range m.Reactions {
		out.Reactions = append(out.Reactions, Reaction{Emoji: r.Emoji.APIName(), Count: r.Count, Me: r.Me})
	}
	return out
}

// contentWithNames rewrites user mentions <@id> as <@name>, so they stay
// distinguishable from plain "@name" text but are readable. Mentioned users
// are remembered for --mention.
func contentWithNames(m *discordgo.Message) string {
	var pairs []string
	for _, u := range m.Mentions {
		rememberUser(u)
		name := "<@" + u.Username + ">"
		pairs = append(pairs, "<@"+u.ID+">", name, "<@!"+u.ID+">", name)
	}
	return strings.NewReplacer(pairs...).Replace(m.Content)
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
