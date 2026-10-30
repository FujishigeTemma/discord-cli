// Command discord is a minimal Discord CLI meant to be driven by AI agents.
// Every command writes JSON Lines to stdout and JSON errors to stderr.
package main

import (
	"errors"
	"os"

	"github.com/alecthomas/kong"
	"github.com/bwmarrin/discordgo"
)

type CLI struct {
	Auth     AuthCmd     `cmd:"" help:"Manage the bot token."`
	Channel  ChannelCmd  `cmd:"" help:"Channels in the server."`
	Thread   ThreadCmd   `cmd:"" help:"Threads in the server."`
	Message  MessageCmd  `cmd:"" help:"Read and write messages."`
	Reaction ReactionCmd `cmd:"" help:"Add or remove reactions."`
}

func main() {
	parser, err := newParser()
	if err != nil {
		fail(err)
	}
	ctx, err := parser.Parse(os.Args[1:])
	if err != nil {
		// A missing or unknown subcommand shows help instead of an error.
		var perr *kong.ParseError
		if errors.As(err, &perr) && hasSubcommands(perr.Context) {
			perr.Context.PrintUsage(false)
			os.Exit(0)
		}
		fail(err)
	}
	if err := ctx.Run(); err != nil {
		fail(err)
	}
}

func newParser() (*kong.Kong, error) {
	return kong.New(&CLI{},
		kong.Name("discord"),
		kong.Description("Minimal Discord CLI. Output is JSON Lines; empty fields are omitted."),
	)
}

func hasSubcommands(ctx *kong.Context) bool {
	node := ctx.Selected()
	if node == nil {
		node = ctx.Model.Node
	}
	return len(node.Children) > 0
}

// fail prints err as a JSON object on stderr and exits with status 1.
func fail(err error) {
	out := struct {
		Error  string `json:"error"`
		Status int    `json:"status,omitzero"`
	}{Error: err.Error()}
	var rest *discordgo.RESTError
	if errors.As(err, &rest) && rest.Response != nil {
		out.Status = rest.Response.StatusCode
		if rest.Message != nil {
			out.Error = rest.Message.Message
		}
	}
	writeJSON(os.Stderr, out)
	os.Exit(1)
}
