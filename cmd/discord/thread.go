package main

import (
	"errors"

	"github.com/bwmarrin/discordgo"
)

type ThreadCmd struct {
	List   ThreadListCmd   `cmd:"" help:"List active threads in the server."`
	Create ThreadCreateCmd `cmd:"" help:"Create a thread. Forum channels need --text or --stdin."`
}

type Thread struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	ParentID     string `json:"parent_id"`
	OwnerID      string `json:"owner_id"`
	MessageCount int    `json:"message_count,omitzero"`
	Archived     bool   `json:"archived,omitzero"`
	Fallback     string `json:"fallback,omitzero"`
}

func toThread(c *discordgo.Channel) Thread {
	t := Thread{
		ID: c.ID, Name: c.Name, Type: channelType(c.Type),
		ParentID: c.ParentID, OwnerID: c.OwnerID, MessageCount: c.MessageCount,
	}
	if c.ThreadMetadata != nil {
		t.Archived = c.ThreadMetadata.Archived
	}
	return t
}

type ThreadListCmd struct {
	Channel string `help:"Only threads under this channel." placeholder:"CHANNEL_ID"`
}

func (c *ThreadListCmd) Run() error {
	s, gid, err := session()
	if err != nil {
		return err
	}
	list, err := s.GuildThreadsActive(gid)
	if err != nil {
		return err
	}
	for _, t := range list.Threads {
		if c.Channel != "" && t.ParentID != c.Channel {
			continue
		}
		if err := emit(toThread(t)); err != nil {
			return err
		}
	}
	return nil
}

type ThreadCreateCmd struct {
	ChannelID string `arg:"" name:"channel_id"`
	Name      string `help:"Thread name." required:"" placeholder:"NAME"`
	Message   string `help:"Start the thread from this message ID." placeholder:"MESSAGE_ID"`
	TextInput `embed:""`
}

func (c *ThreadCreateCmd) Run() error {
	text, err := c.read()
	if err != nil {
		return err
	}
	var data *discordgo.MessageSend
	var fallback string
	if text != "" {
		if data, fallback, err = buildMessage("", text, nil); err != nil {
			return err
		}
	}
	s, _, err := session()
	if err != nil {
		return err
	}

	forum := false
	if c.Message == "" {
		parent, err := s.Channel(c.ChannelID)
		if err != nil {
			return err
		}
		forum = parent.Type == discordgo.ChannelTypeGuildForum || parent.Type == discordgo.ChannelTypeGuildMedia
	}

	var th *discordgo.Channel
	switch {
	case c.Message != "":
		th, err = s.MessageThreadStartComplex(c.ChannelID, c.Message, &discordgo.ThreadStart{Name: c.Name})
	case forum:
		// A forum post is created together with its first message.
		if data == nil {
			return errors.New("forum channels need --text or --stdin")
		}
		th, err = s.ForumThreadStartComplex(c.ChannelID, &discordgo.ThreadStart{Name: c.Name}, data)
	default:
		th, err = s.ThreadStartComplex(c.ChannelID, &discordgo.ThreadStart{
			Name: c.Name, Type: discordgo.ChannelTypeGuildPublicThread,
		})
	}
	if err != nil {
		return err
	}
	if data != nil && !forum {
		if _, err := s.ChannelMessageSendComplex(th.ID, data); err != nil {
			return err
		}
	}
	out := toThread(th)
	out.Fallback = fallback
	return emit(out)
}
