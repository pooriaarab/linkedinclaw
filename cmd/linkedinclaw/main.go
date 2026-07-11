package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/pooriaarab/linkedinclaw/internal/cli"
)

var version = cli.Version

type CLI struct {
	Version      kong.VersionFlag   `name:"version" help:"Print version information and exit."`
	Login        cli.LoginCmd       `cmd:"" help:"Log in to LinkedIn using a real Chrome profile."`
	Sync         cli.SyncCmd        `cmd:"" help:"Synchronize LinkedIn data."`
	Export       cli.ExportCmd      `cmd:"" help:"Export commands for LinkedIn data."`
	Mirror       cli.MirrorCmd      `cmd:"" help:"Manage git-backed archive publish/subscribe."`
	Search       cli.SearchCmd      `cmd:"" help:"Search messages, posts, and saved posts using FTS5."`
	Messages     cli.MessagesCmd    `cmd:"" help:"Query messages by sender name and time window."`
	Tui          cli.TuiCmd         `cmd:"" help:"Browse all data in an interactive terminal user interface."`
	Doctor       cli.DoctorCmd      `cmd:"" help:"Verify configuration, credentials, network, and database."`
	Status       cli.StatusCmd      `cmd:"" help:"Show storage status and item counts."`
	Metadata     cli.MetadataCmd    `cmd:"" help:"Show application metadata."`
	Diagnostics  cli.DiagnosticsCmd `cmd:"" help:"Run diagnostic checks."`
	CheckUpdate  cli.CheckUpdateCmd `cmd:"" name:"check-update" help:"Check for newer versions."`
	ServeSession cli.ServeSessionCmd `cmd:"" name:"serve-session" help:"Start a loopback HTTP server to receive cookie updates from the browser extension."`
}

func main() {
	var cli CLI
	ctx := kong.Parse(&cli,
		kong.Name("linkedinclaw"),
		kong.Description("crawlkit-based LinkedIn mirror"),
		kong.UsageOnError(),
		kong.Vars{
			"version": version,
		},
	)
	err := ctx.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
