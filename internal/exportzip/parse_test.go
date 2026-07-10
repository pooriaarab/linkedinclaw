package exportzip

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestParse(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "export.zip")

	// 1. Create a mock LinkedIn GDPR export zip
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("failed to create zip file: %v", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)

	// Create Connections.csv with 3 lines of disclaimers/notes at the top
	connsFile, err := zw.Create("Connections.csv")
	if err != nil {
		t.Fatalf("failed to create Connections.csv inside zip: %v", err)
	}
	connsCSV := `Notes:
This file contains your first-degree connections.
Email addresses are only shared if connections opted in.

First Name,Last Name,URL,Email Address,Company,Position,Connected On
Jane,Smith,https://www.linkedin.com/in/janesmith,jane@example.com,Google,Engineering Manager,01 Jul 2026
Bob,Johnson,https://www.linkedin.com/in/bobjohnson,,Apple,Product Designer,01/15/2026
`
	if _, err := connsFile.Write([]byte(connsCSV)); err != nil {
		t.Fatalf("failed to write Connections.csv: %v", err)
	}

	// Create messages.csv without disclaimers
	msgsFile, err := zw.Create("messages.csv")
	if err != nil {
		t.Fatalf("failed to create messages.csv inside zip: %v", err)
	}
	msgsCSV := `CONVERSATION ID,CONVERSATION TITLE,FROM,SENDER PROFILE URL,TO,RECIPIENT PROFILE URLS,DATE,SUBJECT,CONTENT,FOLDER
123,Chat with Jane,Jane Smith,https://www.linkedin.com/in/janesmith,Me,https://www.linkedin.com/in/me,"07/08/26, 10:15 PM",,Hello there!,INBOX
123,Chat with Jane,Me,https://www.linkedin.com/in/me,Jane Smith,https://www.linkedin.com/in/janesmith,"07/08/26, 10:20 PM",,Hi Jane!,INBOX
`
	if _, err := msgsFile.Write([]byte(msgsCSV)); err != nil {
		t.Fatalf("failed to write messages.csv: %v", err)
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}

	// 2. Call Parse
	result, err := Parse(zipPath)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// 3. Assert Connections
	if len(result.Connections) != 2 {
		t.Fatalf("expected 2 connections, got %d", len(result.Connections))
	}
	c1 := result.Connections[0]
	if c1.FirstName != "Jane" || c1.LastName != "Smith" || c1.Urn != "https://www.linkedin.com/in/janesmith" || c1.Headline != "Engineering Manager" || c1.Company != "Google" || c1.ConnectedAt != "2026-07-01T00:00:00Z" {
		t.Errorf("unexpected first connection: %+v", c1)
	}
	c2 := result.Connections[1]
	if c2.FirstName != "Bob" || c2.LastName != "Johnson" || c2.Urn != "https://www.linkedin.com/in/bobjohnson" || c2.Headline != "Product Designer" || c2.Company != "Apple" || c2.ConnectedAt != "2026-01-15T00:00:00Z" {
		t.Errorf("unexpected second connection: %+v", c2)
	}

	// 4. Assert Messages
	if len(result.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result.Messages))
	}
	m1 := result.Messages[0]
	if m1.ConversationUrn != "export:conv:123" || m1.SenderUrn != "https://www.linkedin.com/in/janesmith" || m1.Body != "Hello there!" || m1.SentAt != "2026-07-08T22:15:00Z" {
		t.Errorf("unexpected first message: %+v", m1)
	}
	m2 := result.Messages[1]
	if m2.ConversationUrn != "export:conv:123" || m2.SenderUrn != "https://www.linkedin.com/in/me" || m2.Body != "Hi Jane!" || m2.SentAt != "2026-07-08T22:20:00Z" {
		t.Errorf("unexpected second message: %+v", m2)
	}

	// 5. Assert Conversations
	if len(result.Conversations) != 1 {
		t.Fatalf("expected 1 conversation, got %d", len(result.Conversations))
	}
	conv := result.Conversations[0]
	if conv.Urn != "export:conv:123" {
		t.Errorf("expected conversation urn export:conv:123, got %q", conv.Urn)
	}
	expectedParts := "https://www.linkedin.com/in/janesmith,https://www.linkedin.com/in/me"
	if conv.Participants != expectedParts {
		t.Errorf("expected participants %q, got %q", expectedParts, conv.Participants)
	}
	if conv.LastActivityAt != "2026-07-08T22:20:00Z" {
		t.Errorf("expected last activity at 2026-07-08T22:20:00Z, got %q", conv.LastActivityAt)
	}

	// 6. Assert non-zip/corrupt zip returns ErrIncompleteExport
	corruptPath := filepath.Join(tmpDir, "corrupt.zip")
	if err := os.WriteFile(corruptPath, []byte("not a zip file"), 0644); err != nil {
		t.Fatalf("failed to write corrupt file: %v", err)
	}

	_, parseErr := Parse(corruptPath)
	if parseErr == nil {
		t.Fatal("expected error parsing corrupt zip, got nil")
	}
	if !errors.Is(parseErr, ErrIncompleteExport) {
		t.Errorf("expected error to wrap ErrIncompleteExport, got %v", parseErr)
	}
}
