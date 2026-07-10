package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
)

var version = "0.1.0"

type CLI struct {
	Version kong.VersionFlag `name:"version" help:"Print version information and exit."`
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
