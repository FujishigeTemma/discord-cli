package main

type ReactionCmd struct {
	List   ReactionListCmd   `cmd:"" help:"List users who reacted with an emoji (up to 100)."`
	Add    ReactionAddCmd    `cmd:"" help:"Add a reaction as the bot."`
	Remove ReactionRemoveCmd `cmd:"" help:"Remove the bot's reaction."`
}

type reactionArgs struct {
	msgRef
	Emoji string `arg:"" name:"emoji" help:"Unicode emoji (👍) or custom emoji as name:id."`
}

type ReactionListCmd struct{ reactionArgs }

func (c *ReactionListCmd) Run() error {
	s, _, err := session()
	if err != nil {
		return err
	}
	users, err := s.MessageReactions(c.ChannelID, c.MessageID, c.Emoji, 100, "", "")
	if err != nil {
		return err
	}
	for _, u := range users {
		if err := emit(toAuthor(u)); err != nil {
			return err
		}
	}
	return nil
}

type ReactionAddCmd struct{ reactionArgs }

func (c *ReactionAddCmd) Run() error {
	s, _, err := session()
	if err != nil {
		return err
	}
	if err := s.MessageReactionAdd(c.ChannelID, c.MessageID, c.Emoji); err != nil {
		return err
	}
	return emitOK()
}

type ReactionRemoveCmd struct{ reactionArgs }

func (c *ReactionRemoveCmd) Run() error {
	s, _, err := session()
	if err != nil {
		return err
	}
	if err := s.MessageReactionRemove(c.ChannelID, c.MessageID, c.Emoji, "@me"); err != nil {
		return err
	}
	return emitOK()
}
