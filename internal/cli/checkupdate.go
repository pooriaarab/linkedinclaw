package cli

import (
	"context"
	"fmt"

	"github.com/openclaw/crawlkit/releasecheck"
)

// Version is the current application version.
var Version = "0.1.0"

// CheckUpdateCmd represents the check-update command.
type CheckUpdateCmd struct{}

// Run executes the update check.
func (c *CheckUpdateCmd) Run() error {
	ctx := context.Background()

	opts := releasecheck.Options{
		AppName:        "linkedinclaw",
		Owner:          "pooriaarab",
		Repo:           "linkedinclaw",
		CurrentVersion: Version,
		Force:          true,
	}

	result, err := releasecheck.Check(ctx, opts)
	if err != nil {
		return fmt.Errorf("check update failed: %w", err)
	}

	installHint := "Run `go install github.com/pooriaarab/linkedinclaw/cmd/linkedinclaw@latest` to update."
	text := releasecheck.StatusText("linkedinclaw", installHint, result)
	fmt.Println(text)

	return nil
}
