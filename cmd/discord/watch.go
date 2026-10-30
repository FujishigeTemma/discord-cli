package main

import (
	"cmp"
	"context"
	"errors"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

type MessageWatchCmd struct {
	ChannelID   string        `arg:"" name:"channel_id"`
	After       string        `help:"Start after this message ID instead of now. Pass the last printed id to resume without gaps." placeholder:"MESSAGE_ID"`
	Interval    time.Duration `help:"How often to poll." default:"3s"`
	Timeout     time.Duration `help:"Stop after this long, e.g. 10m. 0 means no limit." placeholder:"DURATION"`
	Max         int           `help:"Stop after printing N messages. 0 means no limit." placeholder:"N"`
	IncludeSelf bool          `help:"Also print messages sent by the bot itself."`
}

func (c *MessageWatchCmd) Validate() error {
	if c.Interval <= 0 {
		return errors.New("--interval must be positive")
	}
	if c.Max < 0 || c.Timeout < 0 {
		return errors.New("--max and --timeout must not be negative")
	}
	return nil
}

// now is replaced in tests.
var now = time.Now

// discordEpoch is the first second of 2015, in Unix milliseconds.
const discordEpoch = 1420070400000

// snowflakeAt returns the smallest message ID that Discord could assign at t.
func snowflakeAt(t time.Time) string {
	return strconv.FormatInt((t.UnixMilli()-discordEpoch)<<22, 10)
}

// compareIDs orders snowflakes numerically without parsing them.
func compareIDs(a, b string) int {
	return cmp.Or(cmp.Compare(len(a), len(b)), cmp.Compare(a, b))
}

func (c *MessageWatchCmd) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}

	s, _, err := session()
	if err != nil {
		return err
	}
	selfID := ""
	if !c.IncludeSelf {
		me, err := s.User("@me", discordgo.WithContext(ctx))
		if err != nil {
			return err
		}
		selfID = me.ID
	}

	after := cmp.Or(c.After, snowflakeAt(now()))
	printed := 0
	for {
		ms, err := s.ChannelMessages(c.ChannelID, 100, "", after, "", discordgo.WithContext(ctx))
		if ctx.Err() != nil {
			return nil // timeout or signal: a normal end
		}
		if err != nil {
			return err
		}
		slices.SortFunc(ms, func(a, b *discordgo.Message) int { return compareIDs(a.ID, b.ID) })
		for _, m := range ms {
			after = m.ID
			if selfID != "" && m.Author != nil && m.Author.ID == selfID {
				continue
			}
			out := toMessage(m)
			out.ChannelID = "" // same on every line; it is the argument
			if err := emit(out); err != nil {
				return err
			}
			printed++
			if c.Max > 0 && printed >= c.Max {
				return nil
			}
		}
		if len(ms) == 100 {
			continue // more are waiting; fetch them before sleeping
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(c.Interval):
		}
	}
}
