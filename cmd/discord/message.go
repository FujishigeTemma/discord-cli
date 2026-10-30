package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
)

const maxContent = 2000

// stdin is replaced in tests.
var stdin io.Reader = os.Stdin

// TextInput is shared by commands that take optional message text.
type TextInput struct {
	Text  string `help:"Message text." xor:"text" placeholder:"TEXT"`
	Stdin bool   `help:"Read message text from stdin." xor:"text"`
}

func (t TextInput) read() (string, error) {
	if !t.Stdin {
		return t.Text, nil
	}
	b, err := io.ReadAll(stdin)
	return strings.TrimRight(string(b), "\r\n"), err
}

type MessageCmd struct {
	List   MessageListCmd   `cmd:"" help:"List messages, oldest first. To read only new messages, remember the last ID and pass it as --after next time."`
	Get    MessageGetCmd    `cmd:"" help:"Get one message."`
	Send   MessageSendCmd   `cmd:"" help:"Send a message. Text over 2000 characters is sent as message.txt."`
	Edit   MessageEditCmd   `cmd:"" help:"Edit a message sent by the bot."`
	Delete MessageDeleteCmd `cmd:"" help:"Delete a message."`
}

type MessageListCmd struct {
	ChannelID string `arg:"" name:"channel_id"`
	Limit     int    `help:"Number of messages (1-100)." default:"50" placeholder:"N"`
	Before    string `help:"Messages before this message ID." placeholder:"MESSAGE_ID" xor:"cursor"`
	After     string `help:"Messages after this message ID." placeholder:"MESSAGE_ID" xor:"cursor"`
	Around    string `help:"Messages around this message ID." placeholder:"MESSAGE_ID" xor:"cursor"`
}

func (c *MessageListCmd) Validate() error {
	if c.Limit < 1 || c.Limit > 100 {
		return errors.New("--limit must be between 1 and 100")
	}
	return nil
}

func (c *MessageListCmd) Run() error {
	s, _, err := session()
	if err != nil {
		return err
	}
	ms, err := s.ChannelMessages(c.ChannelID, c.Limit, c.Before, c.After, c.Around)
	if err != nil {
		return err
	}
	slices.Reverse(ms) // the API returns newest first
	for _, m := range ms {
		out := toMessage(m)
		out.ChannelID = "" // same on every line; it is the argument
		if err := emit(out); err != nil {
			return err
		}
	}
	return nil
}

// msgRef is the <channel_id> <message_id> pair shared by several commands.
type msgRef struct {
	ChannelID string `arg:"" name:"channel_id"`
	MessageID string `arg:"" name:"message_id"`
}

type MessageGetCmd struct{ msgRef }

func (c *MessageGetCmd) Run() error {
	s, _, err := session()
	if err != nil {
		return err
	}
	m, err := s.ChannelMessage(c.ChannelID, c.MessageID)
	if err != nil {
		return err
	}
	return emit(toMessage(m))
}

type MessageSendCmd struct {
	ChannelID string `arg:"" name:"channel_id"`
	TextInput `embed:""`
	File      []string `help:"Attach a file. Repeatable." type:"existingfile" placeholder:"PATH"`
	ReplyTo   string   `help:"Reply to this message ID." placeholder:"MESSAGE_ID"`
}

func (c *MessageSendCmd) Run() error {
	text, err := c.read()
	if err != nil {
		return err
	}
	data, fallback, err := buildMessage(text, c.File)
	if err != nil {
		return err
	}
	if c.ReplyTo != "" {
		data.Reference = &discordgo.MessageReference{MessageID: c.ReplyTo}
	}
	s, _, err := session()
	if err != nil {
		return err
	}
	m, err := s.ChannelMessageSendComplex(c.ChannelID, data)
	if err != nil {
		return err
	}
	out := toMessage(m)
	out.Fallback = fallback
	return emit(out)
}

// buildMessage turns text and file paths into a send request. Text longer
// than the Discord limit becomes a message.txt attachment, and fallback is
// set to "attachment".
func buildMessage(text string, paths []string) (*discordgo.MessageSend, string, error) {
	if text == "" && len(paths) == 0 {
		return nil, "", errors.New("nothing to send: give --text, --stdin or --file")
	}
	data := &discordgo.MessageSend{Content: text}
	fallback := ""
	if utf8.RuneCountInString(text) > maxContent {
		data.Content = ""
		data.Files = append(data.Files, &discordgo.File{
			Name: "message.txt", ContentType: "text/plain; charset=utf-8", Reader: strings.NewReader(text),
		})
		fallback = "attachment"
	}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, "", err
		}
		data.Files = append(data.Files, &discordgo.File{
			Name: filepath.Base(p), ContentType: mime.TypeByExtension(filepath.Ext(p)), Reader: bytes.NewReader(b),
		})
	}
	return data, fallback, nil
}

type MessageEditCmd struct {
	msgRef
	TextInput `embed:""`
}

func (c *MessageEditCmd) Run() error {
	text, err := c.read()
	if err != nil {
		return err
	}
	if text == "" {
		return errors.New("give --text or --stdin")
	}
	if n := utf8.RuneCountInString(text); n > maxContent {
		return fmt.Errorf("text is %d characters; edit allows at most %d", n, maxContent)
	}
	s, _, err := session()
	if err != nil {
		return err
	}
	m, err := s.ChannelMessageEdit(c.ChannelID, c.MessageID, text)
	if err != nil {
		return err
	}
	return emit(toMessage(m))
}

type MessageDeleteCmd struct{ msgRef }

func (c *MessageDeleteCmd) Run() error {
	s, _, err := session()
	if err != nil {
		return err
	}
	if err := s.ChannelMessageDelete(c.ChannelID, c.MessageID); err != nil {
		return err
	}
	return emitOK()
}
