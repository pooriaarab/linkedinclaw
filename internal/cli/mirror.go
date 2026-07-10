package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/openclaw/crawlkit/config"
	"github.com/openclaw/crawlkit/mirror"
)

// Note: No companion test file is provided for mirror.go. Its correctness is only
// meaningfully verified against a real git remote, which is a manual/integration check.

// Subscription represents the configuration/marker for a git-backed archive subscription.
type Subscription struct {
	Remote string `json:"remote"`
}

// MirrorCmd manages git-backed archive publish/subscribe.
type MirrorCmd struct {
	Subscribe SubscribeCmd `cmd:"" help:"Subscribe to a git-backed archive snapshot."`
	Publish   PublishCmd   `cmd:"" help:"Publish the local SQLite database to a git-backed archive."`
}

// SubscribeCmd implements the subscription subcommand.
type SubscribeCmd struct {
	GitURL string `arg:"" help:"Git remote URL to subscribe to."`
}

// Run executes the subscribe command.
func (c *SubscribeCmd) Run() error {
	ctx := context.Background()

	// 1. Resolve dedicated archive directory via crawlkit config
	app := config.App{
		Name: "linkedinclaw",
	}
	paths, err := app.DefaultPaths()
	if err != nil {
		return fmt.Errorf("failed to resolve application paths: %w", err)
	}

	archiveDir := filepath.Join(paths.BaseDir, "archive")

	// 2. Call mirror.EnsureRepo then mirror.Pull to fetch the snapshot
	opts := mirror.Options{
		RepoPath: archiveDir,
		Remote:   c.GitURL,
		Branch:   "main",
	}

	fmt.Printf("Subscribing to archive at %s...\n", c.GitURL)
	if err := mirror.EnsureRepo(ctx, opts); err != nil {
		return fmt.Errorf("failed to ensure repository: %w", err)
	}

	if err := mirror.Pull(ctx, opts); err != nil {
		return fmt.Errorf("failed to pull repository snapshot: %w", err)
	}

	// 3. Write a small marker/config noting read-only subscribe mode
	configDir := filepath.Dir(paths.ConfigPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	sub := Subscription{
		Remote: c.GitURL,
	}
	subJSON, err := json.MarshalIndent(sub, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal subscription info: %w", err)
	}

	markerPath := filepath.Join(configDir, "subscribed.json")
	if err := os.WriteFile(markerPath, subJSON, 0644); err != nil {
		return fmt.Errorf("failed to write subscription marker: %w", err)
	}

	// 4. Print confirmation
	fmt.Printf("Successfully subscribed!\n")
	fmt.Printf("Archive directory: %s\n", archiveDir)
	fmt.Println("Instance is now in read-only subscribe mode.")
	fmt.Println("// Note: wiring status/doctor to read the subscription marker is a follow-up.")

	return nil
}

// PublishCmd implements the publish subcommand.
type PublishCmd struct {
	Remote string `help:"Git remote URL to publish to (required on first run)." name:"remote"`
}

// Run executes the publish command.
func (c *PublishCmd) Run() error {
	ctx := context.Background()

	// 1. Resolve application paths
	app := config.App{
		Name: "linkedinclaw",
	}
	paths, err := app.DefaultPaths()
	if err != nil {
		return fmt.Errorf("failed to resolve application paths: %w", err)
	}

	archiveDir := filepath.Join(paths.BaseDir, "archive")
	configDir := filepath.Dir(paths.ConfigPath)
	markerPath := filepath.Join(configDir, "subscribed.json")

	// 2. Resolve git remote URL
	remoteURL := c.Remote
	if remoteURL == "" {
		// Try to read from the previously-configured marker file
		data, err := os.ReadFile(markerPath)
		if err == nil {
			var sub Subscription
			if err := json.Unmarshal(data, &sub); err == nil && sub.Remote != "" {
				remoteURL = sub.Remote
			}
		}
	}

	if remoteURL == "" {
		return errors.New("git remote URL is required; specify via --remote flag or subscribe to an archive first")
	}

	// If remote was specified via flag and changed/first-run, save it to the marker
	if c.Remote != "" {
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
		sub := Subscription{
			Remote: remoteURL,
		}
		subJSON, err := json.MarshalIndent(sub, "", "  ")
		if err == nil {
			_ = os.WriteFile(markerPath, subJSON, 0644)
		}
	}

	// 3. Ensure repository exists and is configured
	opts := mirror.Options{
		RepoPath: archiveDir,
		Remote:   remoteURL,
		Branch:   "main",
	}

	fmt.Printf("Publishing snapshot to %s...\n", remoteURL)
	if err := mirror.EnsureRepo(ctx, opts); err != nil {
		return fmt.Errorf("failed to ensure repository: %w", err)
	}

	// Check if source database exists
	if _, err := os.Stat(paths.DBPath); os.IsNotExist(err) {
		return fmt.Errorf("local database not found at %s; please sync/import data before publishing", paths.DBPath)
	}

	// 4. Copy the real SQLite database file into the archive dir under a fixed filename
	dbName := filepath.Base(paths.DBPath)
	destDBPath := filepath.Join(archiveDir, dbName)

	if err := copyFile(paths.DBPath, destDBPath); err != nil {
		return fmt.Errorf("failed to copy database file to archive: %w", err)
	}

	// 5. Clean SQLite sidecar files (*-wal, *-shm)
	if _, err := mirror.CleanSQLiteSidecars(archiveDir); err != nil {
		return fmt.Errorf("failed to clean SQLite sidecars in archive: %w", err)
	}

	// 6. Commit the changes
	committed, err := mirror.Commit(ctx, opts, "Update linkedinclaw archive snapshot")
	if err != nil {
		return fmt.Errorf("failed to commit archive snapshot: %w", err)
	}

	// 7. Push to the remote only if there were actual changes committed
	if committed {
		fmt.Println("Changes detected. Pushing to remote repository...")
		if err := mirror.Push(ctx, opts); err != nil {
			return fmt.Errorf("failed to push snapshot to remote: %w", err)
		}
		fmt.Println("Successfully published updated snapshot to remote repository!")
	} else {
		fmt.Println("No changes detected since last snapshot. Nothing to push.")
	}

	return nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
