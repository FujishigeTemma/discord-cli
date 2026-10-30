package main

type ReactionCmd struct {
	Add    ReactionAddCmd    `cmd:"" help:"Add a reaction as the bot."`
	Remove ReactionRemoveCmd `cmd:"" help:"Remove the bot's reaction."`
}

type reactionArgs struct {
	msgRef
	Emoji string `arg:"" name:"emoji" help:"Unicode emoji (👍) or custom emoji as name:id."`
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
