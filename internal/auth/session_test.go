package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestResolve_EnvShortCircuit(t *testing.T) {
	expectedLiAt := "some-li-at-cookie-value"
	expectedJSessionID := "some-jsessionid-cookie-value"

	// Setenv automatically cleans up after the test finishes.
	t.Setenv("LINKEDINCLAW_LI_AT", expectedLiAt)
	t.Setenv("LINKEDINCLAW_JSESSIONID", expectedJSessionID)

	liAt, jsessionID, err := Resolve()
	if err != nil {
		t.Fatalf("expected Resolve() to succeed, got error: %v", err)
	}

	if liAt != expectedLiAt {
		t.Errorf("expected liAt to be %q, got %q", expectedLiAt, liAt)
	}

	if jsessionID != expectedJSessionID {
		t.Errorf("expected jsessionID to be %q, got %q", expectedJSessionID, jsessionID)
	}
}

func TestListen(t *testing.T) {
	// Mock storeFunc
	var calledLiAt, calledJSessionID string
	var storeCalls int
	origStore := storeFunc
	storeFunc = func(liAt, jsessionID string) error {
		calledLiAt = liAt
		calledJSessionID = jsessionID
		storeCalls++
		return nil
	}
	defer func() { storeFunc = origStore }()

	// Create temp session file path
	tempDir := t.TempDir()
	sessionFilePath := filepath.Join(tempDir, "subdir", "session.json")

	// Get a free port
	port := getFreePort(t)

	// Start the listener
	srv, err := Listen(port, sessionFilePath)
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer srv.Close()

	// 1. Test POST with missing fields
	badReqBody, _ := json.Marshal(map[string]string{"li_at": "only_one"})
	resp, err := http.Post(fmt.Sprintf("http://127.0.0.1:%d/session", port), "application/json", bytes.NewReader(badReqBody))
	if err != nil {
		t.Fatalf("failed to make POST request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for missing fields, got %d", resp.StatusCode)
	}

	// 2. Test GET method reject
	resp, err = http.Get(fmt.Sprintf("http://127.0.0.1:%d/session", port))
	if err != nil {
		t.Fatalf("failed to make GET request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 Method Not Allowed for GET, got %d", resp.StatusCode)
	}

	// 3. Test valid POST request
	validReqBody, _ := json.Marshal(map[string]string{
		"li_at":      "my-li-at",
		"jsessionid": "my-jsession",
	})
	resp, err = http.Post(fmt.Sprintf("http://127.0.0.1:%d/session", port), "application/json", bytes.NewReader(validReqBody))
	if err != nil {
		t.Fatalf("failed to make valid POST request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	// Check mock calls
	if storeCalls != 1 {
		t.Errorf("expected storeFunc to be called exactly 1 time, got %d", storeCalls)
	}
	if calledLiAt != "my-li-at" || calledJSessionID != "my-jsession" {
		t.Errorf("expected storeFunc called with (%q, %q), got (%q, %q)", "my-li-at", "my-jsession", calledLiAt, calledJSessionID)
	}

	// Check session file
	info, err := os.Stat(sessionFilePath)
	if err != nil {
		t.Fatalf("expected session file to be created: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("expected session file permission to be 0600, got 0%o", perm)
	}

	// Read and verify file contents
	fileData, err := os.ReadFile(sessionFilePath)
	if err != nil {
		t.Fatalf("failed to read session file: %v", err)
	}
	var savedReq SessionRequest
	if err := json.Unmarshal(fileData, &savedReq); err != nil {
		t.Fatalf("failed to unmarshal saved JSON: %v", err)
	}
	if savedReq.LiAt != "my-li-at" || savedReq.JSessionID != "my-jsession" {
		t.Errorf("saved JSON mismatch, got li_at=%q, jsessionid=%q", savedReq.LiAt, savedReq.JSessionID)
	}
}

func getFreePort(t *testing.T) int {
	t.Helper()
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to resolve tcp addr: %v", err)
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		t.Fatalf("failed to listen on free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}
