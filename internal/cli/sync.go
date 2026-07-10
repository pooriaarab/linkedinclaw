package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openclaw/crawlkit/config"
	"github.com/openclaw/crawlkit/store"
	"github.com/pooriaarab/linkedinclaw/internal/auth"
	"github.com/pooriaarab/linkedinclaw/internal/schema"
	"github.com/pooriaarab/linkedinclaw/internal/sync"
	"github.com/pooriaarab/linkedinclaw/internal/voyager"
)

// SyncCmd represents the synchronization command.
type SyncCmd struct {
	Source string `help:"Source of synchronization (api, export, both)." default:"both" enum:"api,export,both"`
	Full   bool   `help:"Perform a full sync, resetting state cursors."`
	JSON   bool   `help:"Format output as JSON."`
}

// Run executes the synchronization command.
func (c *SyncCmd) Run() error {
	ctx := context.Background()

	// 1. Resolve database path using crawlkit config
	app := config.App{
		Name: "linkedinclaw",
	}
	paths, err := app.DefaultPaths()
	if err != nil {
		return fmt.Errorf("failed to resolve application paths: %w", err)
	}

	// 2. Open SQLite database using crawlkit store
	s, err := store.Open(ctx, store.Options{
		Path: paths.DBPath,
	})
	if err != nil {
		return fmt.Errorf("failed to open database at %s: %w", paths.DBPath, err)
	}
	defer s.Close()

	db := s.DB()

	// 3. Run schema migrations
	if err := schema.Migrate(db); err != nil {
		return fmt.Errorf("database migration failed: %w", err)
	}

	// 4. Map string source to enum
	var src sync.Source
	switch c.Source {
	case "api":
		src = sync.SourceAPI
	case "export":
		src = sync.SourceExport
	case "both":
		src = sync.SourceBoth
	default:
		return fmt.Errorf("invalid sync source: %s", c.Source)
	}

	// 5. Resolve voyager client credentials (unless source is export-only, where we don't need API auth)
	var client *voyager.Client
	if src != sync.SourceExport {
		liAt, jsessionID, err := auth.Resolve()
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
		client = voyager.NewClient(liAt, jsessionID)
	}

	// 6. Execute sync
	summary, err := sync.Run(ctx, db, client, src, c.Full)
	if err != nil {
		return fmt.Errorf("synchronization failed: %w", err)
	}

	// 7. Print summary output
	if c.JSON {
		data, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println("Synchronization finished!")
	if len(summary.Completed) > 0 {
		fmt.Printf("Completed categories:\n")
		for _, cat := range summary.Completed {
			fmt.Printf("  - %s\n", cat)
		}
	}
	if len(summary.Deferred) > 0 {
		fmt.Printf("Deferred/Skipped categories:\n")
		for _, cat := range summary.Deferred {
			fmt.Printf("  - %s\n", cat)
		}
	}
	if summary.Comment != "" {
		fmt.Printf("Note: %s\n", summary.Comment)
	}
	if len(summary.FoundExportZips) > 0 {
		fmt.Printf("Found GDPR export ZIPs in Downloads folder:\n")
		for _, path := range summary.FoundExportZips {
			fmt.Printf("  - %s\n", path)
		}
	}

	return nil
}
