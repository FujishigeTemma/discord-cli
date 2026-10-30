package main

import "github.com/bwmarrin/discordgo"

type ChannelCmd struct {
	List ChannelListCmd `cmd:"" help:"List channels in the server."`
}

type Channel struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	ParentID *string `json:"parent_id"`
	Position int     `json:"position"`
	Topic    string  `json:"topic,omitempty"`
}

func toChannel(c *discordgo.Channel) Channel {
	out := Channel{ID: c.ID, Name: c.Name, Type: channelType(c.Type), Position: c.Position, Topic: c.Topic}
	if c.ParentID != "" {
		out.ParentID = &c.ParentID
	}
	return out
}

type ChannelListCmd struct{}

func (ChannelListCmd) Run() error {
	s, gid, err := session()
	if err != nil {
		return err
	}
	cs, err := s.GuildChannels(gid)
	if err != nil {
		return err
	}
	for _, c := range cs {
		if err := emit(toChannel(c)); err != nil {
			return err
		}
	}
	return nil
}
