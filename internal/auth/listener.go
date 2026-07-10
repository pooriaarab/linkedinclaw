package auth

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
)

// SessionRequest represents the JSON body of a session POST request.
type SessionRequest struct {
	LiAt       string `json:"li_at"`
	JSessionID string `json:"jsessionid"`
}

// Listen starts an HTTP server bound strictly to 127.0.0.1:<port> ONLY.
// It registers a handler for POST /session that accepts the session JSON body,
// calls Store() (via storeFunc) with the values, and writes them to the specified
// sessionFilePath with 0600 permissions.
// It returns the starting *http.Server which can be closed/shut down by the caller.
func Listen(port int, sessionFilePath string) (*http.Server, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to bind to %s: %w", addr, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/session", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		var req SessionRequest
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
			return
		}

		if req.LiAt == "" || req.JSessionID == "" {
			http.Error(w, "missing required fields: both li_at and jsessionid must be provided", http.StatusBadRequest)
			return
		}

		// Check and call storeFunc (which maps to Store in production, or mocked in tests)
		storeErr := storeFunc(req.LiAt, req.JSessionID)

		// Ensure parent directory of sessionFilePath exists
		dir := filepath.Dir(sessionFilePath)
		if err := os.MkdirAll(dir, 0700); err != nil {
			http.Error(w, fmt.Sprintf("failed to create session file directory: %v", err), http.StatusInternalServerError)
			return
		}

		// Marshal the request struct back to JSON
		data, err := json.Marshal(req)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal JSON: %v", err), http.StatusInternalServerError)
			return
		}

		// Write to fallback file with 0600 permissions
		if err := os.WriteFile(sessionFilePath, data, 0600); err != nil {
			http.Error(w, fmt.Sprintf("failed to write session file: %v", err), http.StatusInternalServerError)
			return
		}

		// If keyring storage failed, return 500 error to indicate incomplete storage
		if storeErr != nil {
			http.Error(w, fmt.Sprintf("failed to store credentials in keyring: %v", storeErr), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success"}`))
	})

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		// Serve will block until the server is closed
		_ = server.Serve(ln)
	}()

	return server, nil
}
