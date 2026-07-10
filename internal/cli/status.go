package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/openclaw/crawlkit/config"
	"github.com/openclaw/crawlkit/control"
	"github.com/openclaw/crawlkit/output"
	"github.com/openclaw/crawlkit/store"
	"github.com/pooriaarab/linkedinclaw/internal/auth"
	"github.com/pooriaarab/linkedinclaw/internal/schema"
	"github.com/pooriaarab/linkedinclaw/internal/voyager"
)

// MetadataCmd represents the metadata subcommand.
type MetadataCmd struct {
	JSON bool `help:"Format output as JSON."`
}

// Run executes the metadata subcommand.
func (c *MetadataCmd) Run() error {
	manifest := control.NewManifest("linkedinclaw", "linkedinclaw", "linkedinclaw")

	format, err := output.Resolve("", c.JSON)
	if err != nil {
		return fmt.Errorf("failed to resolve output format: %w", err)
	}

	if err := output.Write(os.Stdout, format, "metadata", manifest); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}

// StatusCmd represents the status subcommand.
type StatusCmd struct {
	JSON bool `help:"Format output as JSON."`
}

// Run executes the status subcommand.
func (c *StatusCmd) Run() error {
	ctx := context.Background()

	app := config.App{Name: "linkedinclaw"}
	paths, err := app.DefaultPaths()
	if err != nil {
		return fmt.Errorf("failed to get default paths: %w", err)
	}

	s, err := store.Open(ctx, store.Options{
		Path: paths.DBPath,
	})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer s.Close()

	db := s.DB()
	if err := schema.Migrate(db); err != nil {
		return fmt.Errorf("failed to run database migrations: %w", err)
	}

	connCount, err := QueryCount(ctx, db, "connections")
	if err != nil {
		return fmt.Errorf("failed to query connections count: %w", err)
	}
	msgCount, err := QueryCount(ctx, db, "messages")
	if err != nil {
		return fmt.Errorf("failed to query messages count: %w", err)
	}
	postCount, err := QueryCount(ctx, db, "posts")
	if err != nil {
		return fmt.Errorf("failed to query posts count: %w", err)
	}
	savedCount, err := QueryCount(ctx, db, "saved_posts")
	if err != nil {
		return fmt.Errorf("failed to query saved_posts count: %w", err)
	}
	compCount, err := QueryCount(ctx, db, "companies_followed")
	if err != nil {
		return fmt.Errorf("failed to query companies_followed count: %w", err)
	}

	summaryText := fmt.Sprintf("%d connections, %d messages, %d posts, %d saved posts, %d companies followed", connCount, msgCount, postCount, savedCount, compCount)

	status := control.NewStatus("linkedinclaw", summaryText)
	status.DatabasePath = paths.DBPath
	status.ConfigPath = paths.ConfigPath

	if stat, err := os.Stat(paths.DBPath); err == nil {
		status.DatabaseBytes = stat.Size()
	}
	if stat, err := os.Stat(paths.DBPath + "-wal"); err == nil {
		status.WALBytes = stat.Size()
	}

	status.Counts = []control.Count{
		control.NewCount("connections", "Connections", connCount),
		control.NewCount("messages", "Messages", msgCount),
		control.NewCount("posts", "Posts", postCount),
		control.NewCount("saved_posts", "Saved Posts", savedCount),
		control.NewCount("companies_followed", "Companies Followed", compCount),
	}

	format, err := output.Resolve("", c.JSON)
	if err != nil {
		return fmt.Errorf("failed to resolve output format: %w", err)
	}

	if err := output.Write(os.Stdout, format, "status", status); err != nil {
		return fmt.Errorf("failed to write status: %w", err)
	}

	return nil
}

func QueryCount(ctx context.Context, db *sql.DB, table string) (int64, error) {
	var count int64
	query := fmt.Sprintf("SELECT count(*) FROM %s", table)
	err := db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// DiagnosticsCmd represents the diagnostics subcommand.
type DiagnosticsCmd struct {
	JSON bool `help:"Format output as JSON."`
}

// DiagnosticResult is a small local struct to represent diagnostic check outcomes.
type DiagnosticResult struct {
	ConfigCheck      string `json:"config_check"`
	ConfigPath       string `json:"config_path,omitempty"`
	CredentialsCheck string `json:"credentials_check"`
	CredentialsSrc   string `json:"credentials_source,omitempty"`
	ConnectionCheck  string `json:"connection_check"`
	DatabaseCheck    string `json:"database_check"`
	Passed           int    `json:"passed"`
	Total            int    `json:"total"`
}

// Run executes the diagnostics subcommand.
func (c *DiagnosticsCmd) Run() error {
	var paths config.Paths
	var configCheck = "passed"
	var configErr error
	app := config.App{Name: "linkedinclaw"}
	paths, configErr = app.DefaultPaths()
	if configErr != nil {
		configCheck = "failed: " + configErr.Error()
	}

	var credentialsCheck = "passed"
	var credentialsSrc = ""
	liAt, jsessionID, err := auth.Resolve()
	hasCredentials := err == nil
	if hasCredentials {
		credentialsSrc = "keyring"
		if os.Getenv("LINKEDINCLAW_LI_AT") != "" && os.Getenv("LINKEDINCLAW_JSESSIONID") != "" {
			credentialsSrc = "environment variables"
		}
	} else {
		credentialsCheck = "failed: run `linkedinclaw login` to authenticate"
	}

	var connectionCheck = "failed"
	if hasCredentials {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, voyagerErr := voyager.NewClient(liAt, jsessionID).FetchProfile(ctx)
		cancel()
		if voyagerErr == nil {
			connectionCheck = "passed"
		} else {
			connectionCheck = "failed: " + voyagerErr.Error()
		}
	} else {
		connectionCheck = "failed: credentials missing"
	}

	var databaseCheck = "failed"
	if configErr == nil {
		dbWiringErr := func() error {
			ctx := context.Background()
			s, err := store.Open(ctx, store.Options{
				Path: paths.DBPath,
			})
			if err != nil {
				return err
			}
			defer s.Close()

			db := s.DB()
			if err := schema.Migrate(db); err != nil {
				return err
			}

			var ftsCount int
			err = db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('messages_fts', 'posts_fts', 'saved_posts_fts')").Scan(&ftsCount)
			if err != nil {
				return err
			}
			if ftsCount != 3 {
				return fmt.Errorf("expected 3 FTS tables, found %d", ftsCount)
			}
			return nil
		}()
		if dbWiringErr == nil {
			databaseCheck = "passed"
		} else {
			databaseCheck = "failed: " + dbWiringErr.Error()
		}
	} else {
		databaseCheck = "failed: config load failed"
	}

	passed := 0
	total := 4
	if configCheck == "passed" {
		passed++
	}
	if credentialsCheck == "passed" {
		passed++
	}
	if connectionCheck == "passed" {
		passed++
	}
	if databaseCheck == "passed" {
		passed++
	}

	if !c.JSON {
		if configCheck == "passed" {
			fmt.Printf("✓ Config paths loaded (db: %s)\n", paths.DBPath)
		} else {
			fmt.Printf("✗ Config load failed: %s\n", configCheck)
		}

		if credentialsCheck == "passed" {
			fmt.Printf("✓ Credentials found (source: %s)\n", credentialsSrc)
		} else {
			fmt.Printf("✗ Credentials check: %s\n", credentialsCheck)
		}

		if connectionCheck == "passed" {
			fmt.Println("✓ Voyager connection: authentication verified")
		} else {
			fmt.Printf("✗ Voyager connection failed: %s\n", connectionCheck)
		}

		if databaseCheck == "passed" {
			fmt.Println("✓ Database and FTS5 wiring successfully verified")
		} else {
			fmt.Printf("✗ Database and FTS5 wiring failed: %s\n", databaseCheck)
		}

		fmt.Printf("%d/%d checks passed.\n", passed, total)
		return nil
	}

	diag := DiagnosticResult{
		ConfigCheck:      configCheck,
		ConfigPath:       paths.DBPath,
		CredentialsCheck: credentialsCheck,
		CredentialsSrc:   credentialsSrc,
		ConnectionCheck:  connectionCheck,
		DatabaseCheck:    databaseCheck,
		Passed:           passed,
		Total:            total,
	}

	format, err := output.Resolve("", true)
	if err != nil {
		return fmt.Errorf("failed to resolve output format: %w", err)
	}

	if err := output.Write(os.Stdout, format, "diagnostics", diag); err != nil {
		return fmt.Errorf("failed to write diagnostics: %w", err)
	}

	return nil
}
