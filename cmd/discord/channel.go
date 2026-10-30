package main

import "github.com/bwmarrin/discordgo"

type ChannelCmd struct {
	List ChannelListCmd `cmd:"" help:"List channels in the server."`
}

type Channel struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	ParentID string `json:"parent_id,omitzero"`
	Position int    `json:"position"`
	Topic    string `json:"topic,omitzero"`
}

func toChannel(c *discordgo.Channel) Channel {
	return Channel{ID: c.ID, Name: c.Name, Type: channelType(c.Type), ParentID: c.ParentID, Position: c.Position, Topic: c.Topic}
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
