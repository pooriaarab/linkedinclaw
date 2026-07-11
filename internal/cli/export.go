package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/openclaw/crawlkit/config"
	"github.com/openclaw/crawlkit/store"
	"github.com/pooriaarab/linkedinclaw/internal/exportzip"
	"github.com/pooriaarab/linkedinclaw/internal/schema"
)

// ExportCmd is the entry point for export-related subcommands.
type ExportCmd struct {
	Import  ExportImportCmd  `cmd:"" help:"Import LinkedIn GDPR export data from a ZIP file."`
	Request ExportRequestCmd `cmd:"" help:"Request LinkedIn GDPR data export."`
}

// ExportImportCmd represents the command to import a GDPR zip file.
type ExportImportCmd struct {
	Path string `arg:"" help:"Path to the LinkedIn GDPR export ZIP file." type:"existingfile"`
}

// Run executes the export import subcommand.
func (c *ExportImportCmd) Run() error {
	ctx := context.Background()

	// 1. Parse the zip first. If parse fails, nothing is written to the database.
	res, err := exportzip.Parse(c.Path)
	if err != nil {
		return fmt.Errorf("failed to parse export ZIP: %w", err)
	}

	// 2. Open SQLite database using crawlkit store and config
	app := config.App{
		Name: "linkedinclaw",
	}
	paths, err := app.DefaultPaths()
	if err != nil {
		return fmt.Errorf("failed to resolve application paths: %w", err)
	}

	s, err := store.Open(ctx, store.Options{
		Path: paths.DBPath,
	})
	if err != nil {
		return fmt.Errorf("failed to open database at %s: %w", paths.DBPath, err)
	}
	defer s.Close()

	db := s.DB()

	// 3. Ensure migrations are applied
	if err := schema.Migrate(db); err != nil {
		return fmt.Errorf("database migration failed: %w", err)
	}

	// 4. Perform upserts within a transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, conn := range res.Connections {
		_, err := tx.ExecContext(ctx, `
			INSERT OR REPLACE INTO connections (urn, first_name, last_name, headline, company, connected_at, source)
			VALUES (?, ?, ?, ?, ?, ?, 'export');
		`, conn.Urn, conn.FirstName, conn.LastName, conn.Headline, conn.Company, conn.ConnectedAt)
		if err != nil {
			return fmt.Errorf("failed to import connection %s: %w", conn.Urn, err)
		}
	}

	for _, conv := range res.Conversations {
		_, err := tx.ExecContext(ctx, `
			INSERT OR REPLACE INTO conversations (urn, participants, last_activity_at)
			VALUES (?, ?, ?);
		`, conv.Urn, conv.Participants, conv.LastActivityAt)
		if err != nil {
			return fmt.Errorf("failed to import conversation %s: %w", conv.Urn, err)
		}
	}

	for _, msg := range res.Messages {
		_, err := tx.ExecContext(ctx, `
			INSERT OR REPLACE INTO messages (urn, conversation_urn, sender_urn, body, sent_at, source)
			VALUES (?, ?, ?, ?, ?, 'export');
		`, msg.Urn, msg.ConversationUrn, msg.SenderUrn, msg.Body, msg.SentAt)
		if err != nil {
			return fmt.Errorf("failed to import message %s: %w", msg.Urn, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit import transaction: %w", err)
	}

	// 5. Rebuild full-text search indexes
	_, _ = db.ExecContext(ctx, "INSERT INTO messages_fts(messages_fts) VALUES('rebuild');")

	fmt.Printf("Successfully imported %d connections, %d conversations, and %d messages!\n",
		len(res.Connections), len(res.Conversations), len(res.Messages))

	return nil
}

// ExportRequestCmd represents the command to request a LinkedIn GDPR export.
type ExportRequestCmd struct{}

// Run executes the export request subcommand.
func (c *ExportRequestCmd) Run() (err error) {
	// Defer closing the agent-browser session
	defer func() {
		if closeErr := runCmd("agent-browser", "close"); closeErr != nil {
			if err == nil {
				err = closeErr
			} else {
				fmt.Fprintf(os.Stderr, "Warning: failed to close agent-browser: %v\n", closeErr)
			}
		}
	}()

	fmt.Println("Opening LinkedIn Data Export settings in Chrome...")
	if err := runCmd("agent-browser", "--profile", "Default", "open", "https://www.linkedin.com/psettings/member-data"); err != nil {
		return fmt.Errorf("failed to open LinkedIn data-export settings: %w", err)
	}

	if err := runCmd("agent-browser", "wait", "3000"); err != nil {
		return fmt.Errorf("failed during agent-browser wait: %w", err)
	}

	fmt.Println("Please select the data categories you want and submit the request manually in the opened browser window.")
	fmt.Println("Press Enter here once you have submitted the request or closed the browser.")
	if _, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil {
		return fmt.Errorf("failed to read from stdin: %w", err)
	}

	fmt.Println("Once you have the zip file from LinkedIn, run: linkedinclaw export import <path-to-zip>")
	return nil
}
